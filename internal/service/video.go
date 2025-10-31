package service

import (
	"fmt"
	"kittens/internal/types"
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

func (s *VideoService) GetVideoPath(resolution string) (string, error) {
	if _, exists := types.Resolutions[resolution]; !exists {
		return "", fmt.Errorf("unsupported resolution: %s", resolution)
	}

	videoPath := filepath.Join(s.dataDir, fmt.Sprintf("%s.mp4", resolution))

	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return "", fmt.Errorf("video not found: %s", resolution)
	}

	return videoPath, nil
}
