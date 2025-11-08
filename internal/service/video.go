package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"kittens/internal/config"
	"kittens/internal/parser"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type VideoService struct {
}

func NewVideoService() *VideoService {
	return &VideoService{}
}

func (s *VideoService) GetPath(resolution config.Resolution) (string, error) {
	videoPath := filepath.Join(config.DataDir, fmt.Sprintf("%dx%d.mp4", resolution.Width, resolution.Height))

	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return "", fmt.Errorf("video not found: %dx%d", resolution.Width, resolution.Height)
	}

	return videoPath, nil
}

func (s *VideoService) GetInfo(name string) (*config.FFProbeOutput, error) {
	// videoPath := filepath.Join(config.DataDir, "sourceVideo", name)
	videoPath := filepath.Join(config.DataDir, "video", name)

	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("video not found: %s", name)
	}

	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		videoPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var info config.FFProbeOutput
	if err := json.Unmarshal(output, &info); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	return &info, nil
}

func (s *VideoService) Resize(ctx context.Context, inputPath, outputPath string, width, height int) (<-chan string, <-chan error) {
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

func (s *VideoService) Transcode(ctx context.Context, paramsStr, inputPath, outputPath string) (<-chan string, <-chan error) {
	resultCh := make(chan string, 1)
	errCh := make(chan error, 1)

	inputParams, err := parser.ParseFilename(paramsStr)
	if err != nil {
		errCh <- err
		close(errCh)
		return nil, errCh
	}

	spec := config.ApplyDefaultVideoSpec(inputParams)

	go func() {
		defer close(resultCh)
		defer close(errCh)

		args := []string{
			"-i", inputPath,
			"-t", fmt.Sprintf("%d", spec.Duration),
			"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d",
				spec.Width, spec.Height, spec.Width, spec.Height),
			"-c:v", spec.Codec,
			"-r", fmt.Sprintf("%d", spec.FPS),
		}

		// Bitrate handling
		if strings.HasSuffix(spec.Bitrate, "crf") {
			crf := strings.TrimSuffix(spec.Bitrate, "crf")
			args = append(args, "-crf", crf)
		} else if strings.HasSuffix(spec.Bitrate, "cbr") {
			bitrate := strings.TrimSuffix(spec.Bitrate, "cbr")
			args = append(args, "-b:v", bitrate+"k", "-maxrate", bitrate+"k", "-bufsize", bitrate+"k")
		} else if strings.HasSuffix(spec.Bitrate, "vbr") {
			bitrate := strings.TrimSuffix(spec.Bitrate, "vbr")
			args = append(args, "-b:v", bitrate+"k")
		}

		// Audio
		args = append(args,
			"-c:a", spec.AudioCodec, // audio codec
			"-b:a", fmt.Sprintf("%dk", spec.AudioBitrate), // audio bitrate
			"-ac", "2", // force 2 channels (stereo)
			outputPath,
		)

		cmd := exec.CommandContext(ctx, "ffmpeg", args...)

		log.Printf("Running: ffmpeg %s", strings.Join(args, " "))

		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			errCh <- fmt.Errorf("ffmpeg failed: %w\nOutput: %s", err, stderr.String())
			return
		}

		// log ffmpeg output
		// log.Printf("FFmpeg stderr:\n%s", stderr.String())

		resultCh <- outputPath
	}()

	return resultCh, errCh

}
