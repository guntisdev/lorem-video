package service

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

type VideoService struct {
	dataDir string
}

func NewVideoService() *VideoService {
	dataDir := "/data" // for docker environment
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		dataDir = "data" // for local development
	}
	return &VideoService{dataDir: dataDir}
}

func (s *VideoService) ServeVideo(w http.ResponseWriter, r *http.Request, resolution string) {
	videoPath := filepath.Join(s.dataDir, fmt.Sprintf("%s.mp4", resolution))
	fmt.Println("Path", videoPath)

	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		http.Error(w, "Video not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Accept-Ranges", "bytes")

	http.ServeFile(w, r, videoPath)
}
