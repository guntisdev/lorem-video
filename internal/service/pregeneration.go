package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"lorem.video/internal/config"
)

// PregenerationService handles all pregeneration and path-related logic
type PregenerationService struct {
	videoService *VideoService
}

func NewPregenerationService(videoService *VideoService) *PregenerationService {
	return &PregenerationService{
		videoService: videoService,
	}
}

// StartupPregeneration runs video pregeneration in the background on app startup
func (s *PregenerationService) StartupPregeneration() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		_, err := s.PregenerateAllVideos(ctx)
		if err != nil {
			log.Printf("❌ Failed to pregenerate videos: %v", err)
			return
		}
	}()
}

// PregenerateAllVideos generates all pregenerated videos for all source files
func (s *PregenerationService) PregenerateAllVideos(ctx context.Context) (map[string][]string, error) {
	sourceFiles, err := config.GetSourceVideoFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get source video files: %w", err)
	}

	results := make(map[string][]string)

	for _, sourceFile := range sourceFiles {
		generatedFiles, err := s.PregenerateVideos(ctx, sourceFile)
		if err != nil {
			log.Printf("❌ Failed to pregenerate videos for %s: %v", filepath.Base(sourceFile), err)
			continue
		}

		results[filepath.Base(sourceFile)] = generatedFiles
	}

	return results, nil
}

func (s *PregenerationService) PregenerateVideos(ctx context.Context, inputPath string) ([]string, error) {
	filenameNoExt := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outputDir := filepath.Join(config.AppPaths.Video, filenameNoExt)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	var generatedFiles []string

	for i, spec := range config.DefaultPregenSpecs {
		spec.Name = filenameNoExt
		resultCh, errCh := s.videoService.Transcode(ctx, spec, inputPath, outputDir)

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

func (s *PregenerationService) GetDefaultSourceVideo() string {
	return config.AppPaths.DefaultSourceVideo
}

// GenerateDefaultSourceVideo creates a default test video using FFmpeg generators
func GenerateDefaultSourceVideo(outputPath string) error {
	cmd := exec.Command("ffmpeg",
		"-f", "lavfi",
		"-i", "testsrc2=duration=60:size=1920x1080:rate=30", // Test pattern video
		"-f", "lavfi",
		"-i", "sine=frequency=440:duration=60", // 440Hz test tone
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "25",
		"-c:a", "aac",
		"-b:a", "128k",
		"-y", // Overwrite if exists
		outputPath,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed to generate test video: %w", err)
	}

	log.Printf("Generated default source video: %s", outputPath)
	return nil
}

// EnsureDefaultSourceVideo checks if default source video exists and generates it if not
func EnsureDefaultSourceVideo() error {
	defaultPath := config.AppPaths.DefaultSourceVideo

	if _, err := os.Stat(defaultPath); os.IsNotExist(err) {
		log.Printf("Default source video not found, generating: %s", defaultPath)
		return GenerateDefaultSourceVideo(defaultPath)
	} else if err != nil {
		return fmt.Errorf("failed to check default source video: %w", err)
	}

	return nil
}
