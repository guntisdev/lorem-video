package rest

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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

	videoNameDir := filepath.Join(config.AppPaths.Stream, videoName)
	if _, err := os.Stat(videoNameDir); os.IsNotExist(err) {
		http.Error(w, "Video not found", http.StatusNotFound)
		return
	}

	fullPath := filepath.Join(videoNameDir, path)

	// log.Printf(fullPath)

	// /bunny/playlist.m3u8
	if path == config.HLSMasterPlaylist {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "public, max-age=1")
		http.ServeFile(w, req, fullPath)
		return
	}

	// /bunny/720p/media.m3u8
	if strings.HasSuffix(path, "/"+config.HLSMediaPlaylist) {
		resolutionKey := strings.TrimSuffix(path, "/"+config.HLSMediaPlaylist)
		resolutionDir := filepath.Join(videoNameDir, resolutionKey)
		if stat, err := os.Stat(resolutionDir); err != nil || !stat.IsDir() {
			http.Error(w, "Resolution not found", http.StatusNotFound)
			return
		}

		playlist := generateMediaPlaylist()

		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "public, max-age=1")
		w.Write([]byte(playlist))
		return
	}

	// /bunny/720p/init.mp4
	if strings.HasSuffix(path, "/"+config.HLSInit) {
		w.Header().Set("Content-Type", "video/mp4")
		// w.Header().Set("Cache-Control", "public, max-age=31536000")
		w.Header().Set("Cache-Control", "public, max-age=1")
		http.ServeFile(w, req, fullPath)
		return
	}

	filename := filepath.Base(fullPath)
	// /bunny/720p/media.1679654321.mp4
	if strings.HasPrefix(filename, "media.") && strings.HasSuffix(filename, ".mp4") {
		resolutionKey := strings.TrimSuffix(path, "/"+filename)
		resolutionDir := filepath.Join(videoNameDir, resolutionKey)
		if stat, err := os.Stat(resolutionDir); err != nil || !stat.IsDir() {
			http.Error(w, "Resolution not found", http.StatusNotFound)
			return
		}

		// TODO maybe better pattern match from config.HLSChunkFormat
		chunkPattern := filepath.Join(resolutionDir, "chunk_*.mp4")
		matches, err := filepath.Glob(chunkPattern)
		if err != nil {
			http.Error(w, "Error reading chunks", http.StatusInternalServerError)
			return
		}

		// IMPORTANT: exclude last segment as it may not be full second and wouldn't loop infinitely
		chunkCount := len(matches) - 1

		if chunkCount < 1 {
			http.Error(w, "No chunks found", http.StatusNotFound)
			return
		}

		hlsSeqStr := strings.TrimSuffix(strings.TrimPrefix(filename, "media."), ".mp4")
		hlsSeq, err := strconv.ParseInt(hlsSeqStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid sequence number", http.StatusNotFound)
			return
		}

		chunkId := int(hlsSeq % int64(chunkCount))
		chunkName := fmt.Sprintf("chunk_%03d.mp4", chunkId)
		log.Printf("%s", chunkName)
		chunkFile := filepath.Join(filepath.Dir(fullPath), chunkName)

		w.Header().Set("Content-Type", "video/mp4")
		// w.Header().Set("Cache-Control", "public, max-age=31536000")
		w.Header().Set("Cache-Control", "public, max-age=1")
		http.ServeFile(w, req, chunkFile)
		return
	}

	http.Error(w, "No hls found", http.StatusNotFound)
}

func generateMediaPlaylist() string {
	const segmentsToServe = 5

	now := time.Now().Unix()

	var chunks strings.Builder
	chunks.WriteString("#EXTM3U\n")
	chunks.WriteString("#EXT-X-VERSION:7\n")
	chunks.WriteString("#EXT-X-TARGETDURATION:1\n")
	chunks.WriteString(fmt.Sprintf("#EXT-X-MEDIA-SEQUENCE:%d\n", now))
	chunks.WriteString("#EXT-X-MAP:URI=\"init.mp4\"\n")

	for i := 0; i < segmentsToServe; i++ {
		chunks.WriteString("#EXTINF:1.000000,\n")
		chunks.WriteString(fmt.Sprintf("media.%d.mp4\n", now+int64(i)))
	}

	// log.Printf(chunks.String())

	return chunks.String()
}
