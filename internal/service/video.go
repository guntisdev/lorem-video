package service

import (
	"context"
	"fmt"
	"kittens/internal/types"
	"os"
	"os/exec"
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

func (s *VideoService) ResizeVideo(ctx context.Context, inputPath, outputPath string, width, height int) (<-chan string, <-chan error) {
	resultCh := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		defer close(resultCh)
		defer close(errCh)

		cmd := exec.CommandContext(ctx,
			"ffmpeg",
			"-i", inputPath,
			"-vf", fmt.Sprintf("scale=%d:%d", width, height),
			"-c:a", "copy",
			outputPath,
		)

		if err := cmd.Run(); err != nil {
			errCh <- err
			return
		}

		resultCh <- outputPath
	}()

	return resultCh, errCh
}
