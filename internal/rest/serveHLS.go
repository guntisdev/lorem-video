package rest

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"lorem.video/internal/config"
)

func (r *Rest) ServeHLS(w http.ResponseWriter, req *http.Request) {
	videoName := req.PathValue("videoName")
	path := req.PathValue("path")

	log.Printf("serveHLS %s - %s", videoName, path)

	if videoName == "" {
		videoName = config.DefaultVideoSpec.Name
	}
	if path == "" {
		path = config.HLSMasterPlaylist
	}

	videoDir := filepath.Join(config.AppPaths.Stream, videoName)
	if _, err := os.Stat(videoDir); os.IsNotExist(err) {
		http.Error(w, "Video not found", http.StatusNotFound)
		return
	}

	// TODO generate dynimcally based on current time and chunk count
	if strings.HasSuffix(path, "/media.m3u8") {
		playlist := `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:8
#EXT-X-MEDIA-SEQUENCE:1
#EXT-X-MAP:URI="init.mp4"
#EXTINF:8.356546,
chunk_001.mp4
#EXTINF:4.579387,
chunk_002.mp4
#EXTINF:5.080780,
chunk_003.mp4
#EXTINF:8.356546,
chunk_004.mp4
#EXTINF:1.738162,
chunk_005.mp4
#EXT-X-ENDLIST
`
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write([]byte(playlist))
		return
	}

	// Serve file from video directory
	fullPath := filepath.Join(videoDir, path)

	// Security: ensure we're still within video directory
	if !strings.HasPrefix(filepath.Clean(fullPath), videoDir) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// Set content type
	contentType := "video/mp4"
	if strings.HasSuffix(path, ".m3u8") {
		contentType = "application/vnd.apple.mpegurl"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000")

	http.ServeFile(w, req, fullPath)
}
