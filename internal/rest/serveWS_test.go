package rest

import (
	"context"
	"encoding/binary"
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

func TestServeWSStream(t *testing.T) {
	srv := newWSTestServer(t)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/bunny_hi/websocketstream2?vc=vp8&ac=opus"
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.CloseNow()

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
