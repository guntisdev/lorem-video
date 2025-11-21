package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
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
	sourceFiles, err := s.getSourceVideoFiles()
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

// PregenerateVideos generates all pregenerated videos from DefaultPregenSpecs for a specific source file
func (s *PregenerationService) PregenerateVideos(ctx context.Context, inputPath string) ([]string, error) {
	filenameNoExt := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outputDir := filepath.Join(config.AppPaths.Video, filenameNoExt)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	var generatedFiles []string

	for i, spec := range config.DefaultPregenSpecs {
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

// FindExistingVideo searches for an existing video file with the given filename
// Returns the full path if found, empty string if not found
func (s *PregenerationService) FindExistingVideo(filename string) string {
	// Get all source video files to search their corresponding directories
	sourceFiles, err := s.getSourceVideoFiles()
	if err != nil {
		log.Printf("Warning: failed to get source video files: %v", err)
		return ""
	}

	// Search in pregenerated videos (each source has its own folder)
	for _, sourceFile := range sourceFiles {
		filenameNoExt := strings.TrimSuffix(filepath.Base(sourceFile), filepath.Ext(sourceFile))
		pregeneratedDir := filepath.Join(config.AppPaths.Video, filenameNoExt)
		pregeneratedPath := filepath.Join(pregeneratedDir, filename)

		if _, err := os.Stat(pregeneratedPath); err == nil {
			return pregeneratedPath
		}
	}

	// Search in tmp folder
	tmpPath := filepath.Join(config.AppPaths.Tmp, filename)
	if _, err := os.Stat(tmpPath); err == nil {
		return tmpPath
	}

	return ""
}

func (s *PregenerationService) GetDefaultSourceVideo() string {
	return config.AppPaths.DefaultSourceVideo
}

// getSourceVideoFiles scans the sourceVideo directory and returns all valid video files
func (s *PregenerationService) getSourceVideoFiles() ([]string, error) {
	entries, err := os.ReadDir(config.AppPaths.SourceVideo)
	if err != nil {
		return nil, fmt.Errorf("failed to read source video directory: %w", err)
	}

	var videoFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Check if it's a valid video file
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != "" {
			ext = ext[1:] // Remove the dot
		}

		if slices.Contains(config.ValidContainers, ext) {
			fullPath := filepath.Join(config.AppPaths.SourceVideo, entry.Name())
			videoFiles = append(videoFiles, fullPath)
		}
	}

	return videoFiles, nil
}
