package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"lorem.video/internal/config"
	"lorem.video/internal/parser"
)

type VideoService struct {
}

func NewVideoService() *VideoService {
	return &VideoService{}
}

// StartupPregeneration runs video pregeneration in the background on app startup
func (s *VideoService) StartupPregeneration() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		_, err := s.PregenerateVideos(ctx)
		if err != nil {
			log.Printf("❌ Failed to pregenerate videos: %v", err)
			return
		}
	}()
}

// PregenerateVideos generates all pregenerated videos from DefaultPregenSpecs
func (s *VideoService) PregenerateVideos(ctx context.Context) ([]string, error) {
	inputPath := config.AppPaths.DefaultSourceVideo
	filenameNoExt := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outputDir := filepath.Join(config.AppPaths.Video, filenameNoExt)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	var generatedFiles []string

	for i, spec := range config.DefaultPregenSpecs {
		resultCh, errCh := s.Transcode(ctx, spec, inputPath, outputDir)

		// Wait for completion
		select {
		case result := <-resultCh:
			filename := filepath.Base(result)
			generatedFiles = append(generatedFiles, filename)

		case err := <-errCh:
			return nil, fmt.Errorf("failed to generate video %d (%s %dx%d): %w",
				i+1, spec.Codec, spec.Width, spec.Height, err)

		case <-ctx.Done():
			return nil, fmt.Errorf("pregeneration cancelled: %w", ctx.Err())
		}
	}

	return generatedFiles, nil
}

func (s *VideoService) findExistingVideo(filename string) string {
	// Search in pregenerated videos (data/video/bunny folder)
	inputPath := config.AppPaths.DefaultSourceVideo
	filenameNoExt := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	pregeneratedDir := filepath.Join(config.AppPaths.Video, filenameNoExt)
	pregeneratedPath := filepath.Join(pregeneratedDir, filename)

	if _, err := os.Stat(pregeneratedPath); err == nil {
		return pregeneratedPath
	}

	// Search in tmp folder
	tmpPath := filepath.Join(config.AppPaths.Tmp, filename)
	if _, err := os.Stat(tmpPath); err == nil {
		return tmpPath
	}

	return ""
}

func (s *VideoService) GetOrGenerate(ctx context.Context, paramsStr string) (<-chan string, <-chan error) {
	resultCh := make(chan string, 1)
	errCh := make(chan error, 1)

	inputParams, err := parser.ParseFilename(paramsStr)
	if err != nil {
		go func() {
			defer close(errCh)
			defer close(resultCh)
			errCh <- fmt.Errorf("failed to parse filename parameters: %w", err)
		}()
		return resultCh, errCh
	}

	spec := config.ApplyDefaultVideoSpec(inputParams)
	filename := parser.GenerateFilename(&spec)

	// Search for existing video
	existingPath := s.findExistingVideo(filename)
	if existingPath != "" {
		go func() {
			defer close(resultCh)
			defer close(errCh)
			resultCh <- existingPath
		}()
		return resultCh, errCh
	}

	inputPath := config.AppPaths.DefaultSourceVideo
	outputPath := config.AppPaths.Tmp

	return s.Transcode(ctx, spec, inputPath, outputPath)
}

// TranscodeFromParams parses parameters and calls Transcode with appropriate paths
func (s *VideoService) TranscodeFromParams(ctx context.Context, paramsStr string) (<-chan string, <-chan error) {
	// Parse the parameters
	inputParams, err := parser.ParseFilename(paramsStr)
	if err != nil {
		errCh := make(chan error, 1)
		errCh <- err
		close(errCh)
		return nil, errCh
	}

	spec := config.ApplyDefaultVideoSpec(inputParams)

	inputPath := config.AppPaths.DefaultSourceVideo
	outputPath := config.AppPaths.Video

	return s.Transcode(ctx, spec, inputPath, outputPath)
}

// Transcode performs video transcoding with the given VideoSpec and paths
func (s *VideoService) Transcode(ctx context.Context, spec config.VideoSpec, inputPath, outputPath string) (<-chan string, <-chan error) {
	resultCh := make(chan string, 1)
	errCh := make(chan error, 1)

	// Generate proper filename from the VideoSpec
	filename := parser.GenerateFilename(&spec)
	fullOutputPath := filepath.Join(outputPath, filename)

	// Check if file already exists
	if _, err := os.Stat(fullOutputPath); err == nil {
		go func() {
			defer close(resultCh)
			defer close(errCh)
			resultCh <- fullOutputPath
		}()
		return resultCh, errCh
	}

	go func() {
		defer close(resultCh)
		defer close(errCh)

		args := []string{
			"-i", inputPath,
			"-t", fmt.Sprintf("%d", spec.Duration),
			"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d",
				spec.Width, spec.Height, spec.Width, spec.Height),
		}

		videoCodec := config.VideoCodecNameMap[spec.Codec]

		if videoCodec != "none" {
			args = append(args,
				"-c:v", videoCodec,
				"-r", fmt.Sprintf("%d", spec.FPS),
			)

			if codecArgs, ok := config.VideoCodecArgs[videoCodec]; ok {
				args = append(args, codecArgs...)
			}
		} else {
			args = append(args, "-vn") // no video
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

		audioCodec := config.AudioCodecNameMap[spec.AudioCodec]
		if audioCodec != "none" {
			args = append(args,
				"-c:a", audioCodec, // audio codec
				"-b:a", fmt.Sprintf("%dk", spec.AudioBitrate), // audio bitrate
				"-ac", "2", // force 2 channels (stereo)
			)
		} else {
			args = append(args, "-an") // no audio
		}

		args = append(args, fullOutputPath)

		cmd := exec.CommandContext(ctx, "ffmpeg", args...)

		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			log.Printf("FFmpeg failed with error: %v", err)
			log.Printf("FFmpeg stderr output: %s", stderr.String())
			errCh <- fmt.Errorf("ffmpeg failed: %w\nOutput: %s", err, stderr.String())
			return
		}

		log.Printf("Transcode success: %s", filepath.Base(fullOutputPath))

		resultCh <- fullOutputPath
	}()

	return resultCh, errCh

}

func (s *VideoService) GetInfo(name string) (*config.FFProbeOutput, error) {
	videoPath := filepath.Join(config.AppPaths.Video, name)

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
