package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"lorem.video/internal/config"
)

// StartupPregeneration runs video pregeneration in the background on app startup
func StartupPregeneration() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		_, err := PregenerateAllVideos(ctx)
		if err != nil {
			log.Printf("❌ Failed to pregenerate videos: %v", err)
			return
		}

		_, err = PregenerateAllHLS(ctx)
		if err != nil {
			log.Printf("❌ Failed to pregenerate HLS streams: %v", err)
			return
		}
	}()
}

// PregenerateAllVideos generates all pregenerated videos for all source files
func PregenerateAllVideos(ctx context.Context) (map[string][]string, error) {
	sourceFiles, err := config.GetSourceVideoFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get source video files: %w", err)
	}

	results := make(map[string][]string)

	for _, sourceFile := range sourceFiles {
		generatedFiles, err := PregenerateVideos(ctx, sourceFile)
		if err != nil {
			log.Printf("❌ Failed to pregenerate videos for %s: %v", filepath.Base(sourceFile), err)
			continue
		}

		results[filepath.Base(sourceFile)] = generatedFiles
	}

	return results, nil
}

func PregenerateVideos(ctx context.Context, inputPath string) ([]string, error) {
	filenameNoExt := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outputDir := filepath.Join(config.AppPaths.Video, filenameNoExt)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	var generatedFiles []string

	// Create a video service for transcoding
	videoService := NewVideoService()

	for i, spec := range config.DefaultPregenSpecs {
		spec.Name = filenameNoExt
		resultCh, errCh := videoService.Transcode(ctx, spec, inputPath, outputDir)

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

// PregenerateAllHLS generates HLS streams for all source video files
func PregenerateAllHLS(ctx context.Context) (map[string][]string, error) {
	sourceFiles, err := config.GetSourceVideoFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get source video files: %w", err)
	}

	results := make(map[string][]string)

	for _, sourceFile := range sourceFiles {
		generatedStreams, err := PregenerateHLS(ctx, sourceFile)
		if err != nil {
			log.Printf("❌ Failed to pregenerate HLS streams for %s: %v", filepath.Base(sourceFile), err)
			continue
		}

		results[filepath.Base(sourceFile)] = generatedStreams
	}

	return results, nil
}

// PregenerateHLS generates HLS streams for a specific source video file
func PregenerateHLS(ctx context.Context, inputPath string) ([]string, error) {
	filenameNoExt := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outputDir := filepath.Join(config.AppPaths.Stream, filenameNoExt)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Check if input video is vertical (portrait orientation)
	isVertical, err := isVideoVertical(inputPath)
	if err != nil {
		log.Printf("⚠️  Failed to detect video orientation for %s, using default resolutions: %v", filenameNoExt, err)
		isVertical = false
	}

	hlsResolutions := map[string]config.Resolution{
		"480p":  config.Resolutions["480p"],
		"720p":  config.Resolutions["720p"],
		"1080p": config.Resolutions["1080p"],
	}

	// If video is vertical, swap width/height for HLS transcoding
	if isVertical {
		for key, res := range hlsResolutions {
			hlsResolutions[key] = config.Resolution{
				Width:  res.Height,
				Height: res.Width,
			}
		}
		// log.Printf("Detected vertical video %s, using portrait resolutions for HLS", filenameNoExt)
	}

	var generatedStreams []string
	videoService := NewVideoService()

	for resName, resolution := range hlsResolutions {
		hlsDir := filepath.Join(outputDir, resName)
		playlistPath := filepath.Join(hlsDir, config.HLSMediaPlaylist)

		if _, err := os.Stat(playlistPath); err == nil {
			// HLS stream already exists, skip generation
			generatedStreams = append(generatedStreams, resName+": "+filepath.Base(playlistPath)+" (existing)")
			continue
		}

		// Create directory before transcoding
		if err := os.MkdirAll(hlsDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create HLS directory %s: %w", hlsDir, err)
		}

		resultCh, errCh := videoService.TranscodeHLS(ctx, resolution, inputPath, hlsDir)

		select {
		case result := <-resultCh:
			generatedStreams = append(generatedStreams, resName+": "+filepath.Base(result))
			log.Printf("✅ Generated HLS stream %s for %s: %s", resName, filenameNoExt, filepath.Base(result))

		case err := <-errCh:
			return nil, fmt.Errorf("failed to generate HLS stream %s (%dx%d): %w",
				resName, resolution.Width, resolution.Height, err)

		case <-ctx.Done():
			return nil, fmt.Errorf("HLS pregeneration cancelled: %w", ctx.Err())
		}
	}

	masterPlaylistPath := filepath.Join(outputDir, config.HLSMasterPlaylist)
	if _, err := os.Stat(masterPlaylistPath); err == nil {
		// Master playlist already exists, skip generation
		generatedStreams = append(generatedStreams, "master: "+filepath.Base(masterPlaylistPath)+" (existing)")
	} else {
		if err := generateMasterPlaylist(masterPlaylistPath, hlsResolutions, filenameNoExt); err != nil {
			return nil, fmt.Errorf("failed to generate master playlist: %w", err)
		}

		generatedStreams = append(generatedStreams, "master: "+filepath.Base(masterPlaylistPath))
		log.Printf("✅ Generated master playlist for %s: %s", filenameNoExt, filepath.Base(masterPlaylistPath))
	}

	return generatedStreams, nil
}

func generateMasterPlaylist(masterPlaylistPath string, hlsResolutions map[string]config.Resolution, videoName string) error {
	// Define approximate bandwidth for each resolution (these are rough estimates)
	bandwidths := map[string]int{
		"480p":  800000,  // 800 kbps
		"720p":  2000000, // 2 Mbps
		"1080p": 5000000, // 5 Mbps
	}

	var content strings.Builder
	content.WriteString("#EXTM3U\n")
	content.WriteString("#EXT-X-VERSION:6\n\n")

	resolutionOrder := []string{"480p", "720p", "1080p"}
	baseURL := config.GetBaseURL()

	for _, resKey := range resolutionOrder {
		if resolution, exists := hlsResolutions[resKey]; exists {
			bandwidth := bandwidths[resKey]
			resName := config.ResolutionsName[resKey]

			content.WriteString(fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d,NAME=%s,RESOLUTION=%dx%d\n",
				bandwidth, resName, resolution.Width, resolution.Height))
			content.WriteString(fmt.Sprintf("%s/hls/%s/%s/%s\n\n", baseURL, videoName, resKey, config.HLSMediaPlaylist))
		}
	}

	return os.WriteFile(masterPlaylistPath, []byte(content.String()), 0644)
}

func isVideoVertical(inputPath string) (bool, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height:stream_side_data=rotation",
		"-of", "json",
		inputPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("ffprobe failed: %w", err)
	}

	// Parse JSON output
	var result struct {
		Streams []struct {
			Width        int `json:"width"`
			Height       int `json:"height"`
			SideDataList []struct {
				Rotation int `json:"rotation"`
			} `json:"side_data_list"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return false, fmt.Errorf("failed to parse ffprobe JSON: %w", err)
	}

	if len(result.Streams) == 0 {
		return false, fmt.Errorf("no video streams found")
	}

	stream := result.Streams[0]
	width, height := stream.Width, stream.Height

	// Check for rotation metadata
	rotation := 0
	if len(stream.SideDataList) > 0 {
		rotation = stream.SideDataList[0].Rotation
	}

	// Video is considered vertical if:
	// 1. Natural portrait orientation (height > width), OR
	// 2. Rotated 90 or 270 degrees (±90)
	isNaturalPortrait := height > width
	isRotatedPortrait := math.Abs(float64(rotation)) == 90

	return isNaturalPortrait || isRotatedPortrait, nil
}
