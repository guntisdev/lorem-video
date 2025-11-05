package service

import (
	"context"
	"fmt"
	"kittens/internal/config"
	"os"
	"os/exec"
	"path/filepath"
)

type VideoService struct {
}

func NewVideoService() *VideoService {
	return &VideoService{}
}

func (s *VideoService) GetVideoPath(resolution string) (string, error) {
	if _, exists := config.Resolutions[resolution]; !exists {
		return "", fmt.Errorf("unsupported resolution: %s", resolution)
	}

	videoPath := filepath.Join(config.DataDir, fmt.Sprintf("%s.mp4", resolution))

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
			// if not exact aspect ration then scales up and crops one dimension
			"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d", width, height, width, height),
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
