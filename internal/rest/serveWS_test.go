package rest

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"lorem.video/internal/config"
	"lorem.video/internal/service"
	"lorem.video/internal/stats"
)

const testChunkCount = 30

// fakeChunk mimics a normalized cluster: 4 byte id, 8 byte size, 0xE7 0x88, 8 byte timecode
func fakeChunk(index int) []byte {
	body := []byte(fmt.Sprintf("chunk-%03d-body", index))
	b := []byte{0x1F, 0x43, 0xB6, 0x75}
	size := make([]byte, 8)
	binary.BigEndian.PutUint64(size, uint64(10+len(body)))
	size[0] = 0x01
	b = append(b, size...)
	b = append(b, 0xE7, 0x88)
	b = binary.BigEndian.AppendUint64(b, uint64(index)*1000)
	return append(b, body...)
}

func newWSTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	tempDir := t.TempDir()
	old := config.AppPaths
	t.Cleanup(func() { config.AppPaths = old })

	config.AppPaths = &config.Paths{
		Data:       tempDir,
		WSStream:   filepath.Join(tempDir, "wsstream"),
		Logs:       filepath.Join(tempDir, "logs"),
		LogsStats:  filepath.Join(tempDir, "logs", "stats"),
		LogsBots:   filepath.Join(tempDir, "logs", "bots"),
		LogsErrors: filepath.Join(tempDir, "logs", "errors"),
	}
	for _, d := range []string{config.AppPaths.LogsStats, config.AppPaths.LogsBots, config.AppPaths.LogsErrors} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	streamDir := filepath.Join(config.AppPaths.WSStream, "bunny", "720p")
	if err := os.MkdirAll(streamDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(streamDir, config.WSInit), []byte("INIT-SEGMENT"), 0644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < testChunkCount; i++ {
		name := fmt.Sprintf(config.WSChunkFormat, i)
		if err := os.WriteFile(filepath.Join(streamDir, name), fakeChunk(i), 0644); err != nil {
			t.Fatal(err)
		}
	}

	rest := New()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws/{videoName}/manifest-ws2.json", rest.ServeWSManifest)
	mux.HandleFunc("GET /ws/{streamName}/websocketstream2", rest.ServeWSStream)

	// same chain as cmd/server/main.go, so the hijack path is exercised
	statsMiddleware := stats.StatsMiddleware(config.AppPaths.LogsStats)
	handler := rest.RecoveryMiddleware(rest.BotsMiddleware(statsMiddleware(rest.CORSMiddleware(mux))))

	return httptest.NewServer(handler)
}

func wsDial(t *testing.T, ctx context.Context, srv *httptest.Server, stream string) *websocket.Conn {
	t.Helper()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/" + stream + "/websocketstream2?vc=vp8&ac=opus"
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.CloseNow() })
	return conn
}

func wsSendPlay(t *testing.T, ctx context.Context, conn *websocket.Conn, requestID int64, stream string) wsPlayStatus {
	t.Helper()

	cmd, err := json.Marshal(wsCommand{
		EventType:        wsEventPlay,
		RequestID:        requestID,
		RequestTimestamp: time.Now().UnixMilli(),
		Stream:           stream,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Write(ctx, websocket.MessageText, cmd); err != nil {
		t.Fatalf("write PLAY: %v", err)
	}

	typ, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read PLAY_STATUS: %v", err)
	}
	if typ != websocket.MessageText {
		t.Errorf("PLAY_STATUS type = %v, want text", typ)
	}

	var status wsPlayStatus
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("unmarshal %q: %v", data, err)
	}
	if status.EventType != wsEventPlayStatus {
		t.Errorf("eventType = %q, want %q", status.EventType, wsEventPlayStatus)
	}
	if status.InReplyTo != requestID {
		t.Errorf("inReplyTo = %d, want %d", status.InReplyTo, requestID)
	}
	if status.Timestamp == 0 {
		t.Error("timeStamp = 0, want a wall clock value")
	}
	return status
}

func TestServeWSStream(t *testing.T) {
	srv := newWSTestServer(t)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn := wsDial(t, ctx, srv, "bunny_hi")

	if status := wsSendPlay(t, ctx, conn, 1, "bunny_hi"); !status.Success || !status.Playing {
		t.Fatalf("PLAY_STATUS = %+v, want success and playing", status)
	}

	typ, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read init: %v", err)
	}
	if typ != websocket.MessageBinary {
		t.Errorf("init message type = %v, want binary", typ)
	}
	if string(data) != "INIT-SEGMENT" {
		t.Errorf("init = %q, want INIT-SEGMENT", data)
	}

	var prevSeq int64
	for i := 0; i < 4; i++ {
		_, chunk, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read chunk %d: %v", i, err)
		}

		absMs := binary.BigEndian.Uint64(chunk[service.WSTimecodeOffset : service.WSTimecodeOffset+8])
		seq := int64(absMs) / 1000

		if absMs%1000 != 0 {
			t.Errorf("chunk %d timecode %d not a whole second", i, absMs)
		}
		if i > 0 && seq != prevSeq+1 {
			t.Errorf("chunk %d seq = %d, want %d", i, seq, prevSeq+1)
		}
		prevSeq = seq

		wantBody := fmt.Sprintf("chunk-%03d-body", seq%testChunkCount)
		if !strings.HasSuffix(string(chunk), wantBody) {
			t.Errorf("chunk %d (seq %d) body = %q, want suffix %q", i, seq, chunk[22:], wantBody)
		}
	}

	if now := time.Now().Unix(); prevSeq < now || prevSeq > now+wsLeadChunks+1 {
		t.Errorf("lead seq = %d, now = %d, want within %d ahead", prevSeq, now, wsLeadChunks)
	}
}

func TestServeWSStreamNotFound(t *testing.T) {
	srv := newWSTestServer(t)
	defer srv.Close()

	for _, name := range []string{"bunny_nope", "nosuch_hi", "bunny", "..%2F.._hi"} {
		resp, err := http.Get(srv.URL + "/ws/" + name + "/websocketstream2")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", name, resp.StatusCode)
		}
	}
}

func TestParseWSStreamName(t *testing.T) {
	cases := []struct {
		in    string
		video string
		res   string
		ok    bool
	}{
		{"bunny_low", "bunny", "360p", true},
		{"bunny_med", "bunny", "480p", true},
		{"bunny_hi", "bunny", "720p", true},
		{"bunny_hd", "bunny", "1080p", true},
		{"my_video_hi", "my_video", "720p", true},
		{"bunny_xxx", "", "", false},
		{"bunny", "", "", false},
		{"_hi", "", "", false},
		{"../etc_hi", "", "", false},
	}
	for _, c := range cases {
		video, res, ok := parseWSStreamName(c.in)
		if video != c.video || res != c.res || ok != c.ok {
			t.Errorf("parseWSStreamName(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.in, video, res, ok, c.video, c.res, c.ok)
		}
	}
}

func TestServeWSStreamIgnoresClientMessages(t *testing.T) {
	srv := newWSTestServer(t)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn := wsDial(t, ctx, srv, "bunny_hi")
	wsSendPlay(t, ctx, conn, 1, "bunny_hi")

	if _, _, err := conn.Read(ctx); err != nil {
		t.Fatalf("read init: %v", err)
	}

	// a player wiring up its own control channel must not kill the stream
	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"cmd":"play"}`)); err != nil {
		t.Fatalf("write text: %v", err)
	}
	if err := conn.Write(ctx, websocket.MessageBinary, []byte{0x01, 0x02, 0x03}); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	for i := 0; i < 3; i++ {
		if _, _, err := conn.Read(ctx); err != nil {
			t.Fatalf("read chunk %d after client message: %v", i, err)
		}
	}
}

func TestServeWSStreamWaitsForPlay(t *testing.T) {
	srv := newWSTestServer(t)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn := wsDial(t, ctx, srv, "bunny_hi")

	// a cancelled Read would close the connection, so one goroutine owns the read side
	type msg struct {
		typ  websocket.MessageType
		data []byte
	}
	msgs := make(chan msg, 4)
	go func() {
		for {
			typ, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			msgs <- msg{typ, data}
		}
	}()

	// junk before PLAY must neither be answered nor start the stream
	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"eventType":"VOLUME","requestId":7}`)); err != nil {
		t.Fatalf("write junk: %v", err)
	}
	if err := conn.Write(ctx, websocket.MessageText, []byte("not json at all")); err != nil {
		t.Fatalf("write junk: %v", err)
	}

	select {
	case m := <-msgs:
		t.Fatalf("got %v message %q before PLAY, want silence", m.typ, m.data)
	case <-time.After(2 * time.Second):
	}

	// and the stream still starts once PLAY finally arrives
	cmd, err := json.Marshal(wsCommand{EventType: wsEventPlay, RequestID: 42, Stream: "bunny_hi"})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Write(ctx, websocket.MessageText, cmd); err != nil {
		t.Fatalf("write PLAY: %v", err)
	}

	m := <-msgs
	var status wsPlayStatus
	if err := json.Unmarshal(m.data, &status); err != nil {
		t.Fatalf("unmarshal %q: %v", m.data, err)
	}
	if status.EventType != wsEventPlayStatus || status.InReplyTo != 42 || !status.Success || !status.Playing {
		t.Errorf("PLAY_STATUS = %+v, want PLAY_STATUS for 42, success and playing", status)
	}

	if m := <-msgs; m.typ != websocket.MessageBinary || string(m.data) != "INIT-SEGMENT" {
		t.Errorf("first push after PLAY = (%v, %q), want binary INIT-SEGMENT", m.typ, m.data)
	}
}

func TestServeWSStreamRejectsMismatchedStream(t *testing.T) {
	srv := newWSTestServer(t)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn := wsDial(t, ctx, srv, "bunny_hi")

	status := wsSendPlay(t, ctx, conn, 3, "bunny_hd")
	if status.Success || status.Playing {
		t.Errorf("PLAY_STATUS = %+v, want success and playing false", status)
	}

	// a retry with the right stream is still accepted on the same connection
	if status := wsSendPlay(t, ctx, conn, 4, "bunny_hi"); !status.Success || !status.Playing {
		t.Fatalf("retry PLAY_STATUS = %+v, want success and playing", status)
	}
	if _, data, err := conn.Read(ctx); err != nil || string(data) != "INIT-SEGMENT" {
		t.Fatalf("read init after retry = (%q, %v)", data, err)
	}
}
