package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"

	"lorem.video/internal/config"
	"lorem.video/internal/service"
)

const wsVideoCodec = "vp8"
const wsAudioCodec = "opus"

// how many seconds ahead of the wall clock to keep the client buffered
const wsLeadChunks = 3

const wsWriteTimeout = 10 * time.Second

// how long a client may sit connected without asking to play
const wsPlayTimeout = 30 * time.Second

const (
	wsEventPlay       = "PLAY"
	wsEventPlayStatus = "PLAY_STATUS"
)

type wsCommand struct {
	EventType        string `json:"eventType"`
	RequestID        int64  `json:"requestId"`
	RequestTimestamp int64  `json:"requestTimestamp"`
	Stream           string `json:"stream"`
}

type wsPlayStatus struct {
	EventType string `json:"eventType"`
	InReplyTo int64  `json:"inReplyTo"`
	Playing   bool   `json:"playing"`
	Success   bool   `json:"success"`
	Timestamp int64  `json:"timeStamp"`
}

type wsManifest struct {
	Name    string           `json:"name"`
	Streams []wsManifestItem `json:"streams"`
}

type wsManifestItem struct {
	StreamName string `json:"streamName"`
	URL        string `json:"url"`
	StreamID   string `json:"streamId"`
	Bitrate    string `json:"bitrate"`
	VC         string `json:"vc"`
}

func (rest *Rest) ServeWSManifest(w http.ResponseWriter, r *http.Request) {
	videoName := r.PathValue("videoName")
	if videoName == "" {
		videoName = config.DefaultVideoSpec.Name
	}

	if _, err := os.Stat(filepath.Join(config.AppPaths.WSStream, videoName)); os.IsNotExist(err) {
		http.Error(w, "Stream not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")

	if err := json.NewEncoder(w).Encode(buildWSManifest(videoName)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func buildWSManifest(videoName string) wsManifest {
	baseURL := config.GetWSBaseURL()
	streams := make([]wsManifestItem, 0, len(config.WSRenditions))

	for _, res := range config.WSRenditions {
		bitrate, ok := config.WSBitrates[res]
		if !ok {
			continue
		}

		streamName := videoName + "_" + config.WSStreamSuffix[res]

		streams = append(streams, wsManifestItem{
			StreamName: streamName,
			URL: fmt.Sprintf("%s/ws/%s/websocketstream2?vc=%s&ac=%s",
				baseURL, streamName, wsVideoCodec, wsAudioCodec),
			StreamID: config.ResolutionsName[res],
			Bitrate:  strconv.Itoa(bitrate.Video + bitrate.Audio),
			VC:       wsVideoCodec,
		})
	}

	return wsManifest{Name: videoName + "_auto", Streams: streams}
}

func (rest *Rest) ServeWSStream(w http.ResponseWriter, r *http.Request) {
	streamName := r.PathValue("streamName")

	videoName, resKey, ok := parseWSStreamName(streamName)
	if !ok {
		http.Error(w, "Stream not found", http.StatusNotFound)
		return
	}

	streamDir := filepath.Join(config.AppPaths.WSStream, videoName, resKey)

	initSegment, err := os.ReadFile(filepath.Join(streamDir, config.WSInit))
	if err != nil {
		http.Error(w, "Stream not found", http.StatusNotFound)
		return
	}

	chunks, err := filepath.Glob(filepath.Join(streamDir, "chunk_*.webm"))
	if err != nil || len(chunks) == 0 {
		http.Error(w, "No chunks found", http.StatusNotFound)
		return
	}
	chunkCount := int64(len(chunks))

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
	if err != nil {
		return
	}
	defer conn.CloseNow()

	if !wsAwaitPlay(r.Context(), conn, streamName) {
		return
	}

	// further player control messages are irrelevant here, but must be drained so reads keep flowing
	ctx := wsDrainReads(r.Context(), conn)

	if err := wsWrite(ctx, conn, initSegment); err != nil {
		return
	}

	ticker := time.NewTicker(time.Duration(config.WSClusterMs) * time.Millisecond)
	defer ticker.Stop()

	next := time.Now().Unix()

	for {
		for target := time.Now().Unix() + wsLeadChunks; next <= target; next++ {
			chunk, err := os.ReadFile(filepath.Join(streamDir, fmt.Sprintf(config.WSChunkFormat, next%chunkCount)))
			if err != nil {
				return
			}
			if err := service.PatchClusterTimecode(chunk, uint64(next)*uint64(config.WSClusterMs)); err != nil {
				return
			}
			if err := wsWrite(ctx, conn, chunk); err != nil {
				return
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// blocks until the client asks to play this stream, ignoring whatever else it sends
func wsAwaitPlay(ctx context.Context, conn *websocket.Conn, streamName string) bool {
	ctx, cancel := context.WithTimeout(ctx, wsPlayTimeout)
	defer cancel()

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return false
		}

		var cmd wsCommand
		if err := json.Unmarshal(data, &cmd); err != nil || cmd.EventType != wsEventPlay {
			continue
		}

		// an omitted stream means the client is happy with whatever this URL serves
		accepted := cmd.Stream == "" || cmd.Stream == streamName

		err = wsWriteJSON(ctx, conn, wsPlayStatus{
			EventType: wsEventPlayStatus,
			InReplyTo: cmd.RequestID,
			Playing:   accepted,
			Success:   accepted,
			Timestamp: time.Now().UnixMilli(),
		})
		if err != nil {
			return false
		}

		if accepted {
			return true
		}
	}
}

// discards anything the client sends and cancels the returned ctx once it goes away
func wsDrainReads(ctx context.Context, conn *websocket.Conn) context.Context {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		defer cancel()

		for {
			_, reader, err := conn.Reader(ctx)
			if err != nil {
				return
			}
			if _, err := io.Copy(io.Discard, reader); err != nil {
				return
			}
		}
	}()

	return ctx
}

func wsWrite(ctx context.Context, conn *websocket.Conn, data []byte) error {
	ctx, cancel := context.WithTimeout(ctx, wsWriteTimeout)
	defer cancel()
	return conn.Write(ctx, websocket.MessageBinary, data)
}

func wsWriteJSON(ctx context.Context, conn *websocket.Conn, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, wsWriteTimeout)
	defer cancel()
	return conn.Write(ctx, websocket.MessageText, data)
}

func parseWSStreamName(streamName string) (videoName, resKey string, ok bool) {
	i := strings.LastIndex(streamName, "_")
	if i < 1 || strings.Contains(streamName, "..") {
		return "", "", false
	}

	suffix := streamName[i+1:]
	for key, s := range config.WSStreamSuffix {
		if s == suffix {
			return streamName[:i], key, true
		}
	}

	return "", "", false
}
