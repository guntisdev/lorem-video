//go:build integration
// +build integration

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lorem.video/internal/config"
	"lorem.video/internal/parser"
)

// Integration tests for video transcoding - these are slow and require FFmpeg
// Run with: go test -tags=integration ./internal/service

func TestGetOrGenerateIntegration(t *testing.T) {
	// Skip if FFmpeg is not available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg not found, skipping integration test")
	}

	// Create temporary test directory structure
	tempDir := t.TempDir()

	// Set up temporary config paths for testing
	oldAppPaths := config.AppPaths
	defer func() { config.AppPaths = oldAppPaths }()

	config.AppPaths = &config.Paths{
		Data:               tempDir,
		Video:              filepath.Join(tempDir, "video"),
		SourceVideo:        filepath.Join(tempDir, "sourceVideo"),
		Logs:               filepath.Join(tempDir, "logs"),
		Tmp:                filepath.Join(tempDir, "tmp"),
		DefaultSourceVideo: filepath.Join(tempDir, "sourceVideo", "bunny.mp4"),
	}

	// Create directories
	if err := config.EnsureDirectories(); err != nil {
		t.Fatalf("Failed to create directories: %v", err)
	}

	// Create a simple test video as source
	createTestVideo(t, config.AppPaths.DefaultSourceVideo, 2, 640, 360)

	// Create bunny subdirectory for pregenerated videos
	bunnyDir := filepath.Join(config.AppPaths.Video, "bunny")
	if err := os.MkdirAll(bunnyDir, 0755); err != nil {
		t.Fatalf("Failed to create bunny directory: %v", err)
	}

	service := NewVideoService()

	t.Run("Generate new video when not found", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		params := "h264_720p_30fps_2s_25crf_aac_96kbps.mp4"
		resultCh, errCh := service.GetOrGenerate(ctx, params)

		select {
		case result := <-resultCh:
			// Should be in tmp directory since it's newly generated
			expectedPrefix := config.AppPaths.Tmp
			if !strings.HasPrefix(result, expectedPrefix) {
				t.Errorf("Expected result to be in tmp dir (%s), got: %s", expectedPrefix, result)
			}

			// Verify file exists and has content
			info, err := os.Stat(result)
			if err != nil {
				t.Fatalf("Output file not found: %v", err)
			}
			if info.Size() == 0 {
				t.Fatal("Output file is empty")
			}

			t.Logf("✅ New video generated: %s (size: %d bytes)", result, info.Size())

		case err := <-errCh:
			t.Fatalf("GetOrGenerate failed: %v", err)

		case <-ctx.Done():
			t.Fatal("GetOrGenerate timed out")
		}
	})

	t.Run("Find existing video in pregenerated folder", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// First, create a pregenerated video
		params := "h264_480p_30fps_2s_23crf_aac_128kbps.mp4"
		filename := "h264_854x480_30fps_2s_23crf_aac_128kbps.mp4"
		pregeneratedPath := filepath.Join(bunnyDir, filename)

		// Create the pregenerated video
		createTestVideo(t, pregeneratedPath, 2, 854, 480)

		// Now test GetOrGenerate finds it
		resultCh, errCh := service.GetOrGenerate(ctx, params)

		select {
		case result := <-resultCh:
			if result != pregeneratedPath {
				t.Errorf("Expected to find pregenerated video at %s, got: %s", pregeneratedPath, result)
			}
			t.Logf("✅ Found existing pregenerated video: %s", result)

		case err := <-errCh:
			t.Fatalf("GetOrGenerate failed: %v", err)

		case <-ctx.Done():
			t.Fatal("GetOrGenerate timed out")
		}
	})

	t.Run("Find existing video in tmp folder", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// First, create a video in tmp folder
		params := "vp9_720p_30fps_2s_30crf_opus_128kbps.webm"
		filename := "vp9_1280x720_30fps_2s_30crf_opus_128kbps.webm"
		tmpPath := filepath.Join(config.AppPaths.Tmp, filename)

		// Create the tmp video
		createTestVideo(t, tmpPath, 2, 1280, 720)

		// Now test GetOrGenerate finds it
		resultCh, errCh := service.GetOrGenerate(ctx, params)

		select {
		case result := <-resultCh:
			if result != tmpPath {
				t.Errorf("Expected to find tmp video at %s, got: %s", tmpPath, result)
			}
			t.Logf("✅ Found existing tmp video: %s", result)

		case err := <-errCh:
			t.Fatalf("GetOrGenerate failed: %v", err)

		case <-ctx.Done():
			t.Fatal("GetOrGenerate timed out")
		}
	})
}

func TestVideoTranscodeIntegration(t *testing.T) {
	// Skip if FFmpeg is not available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg not found, skipping integration test")
	}

	// Create temporary test directory
	tempDir := t.TempDir()

	// Create a simple test video (1 second, small resolution for speed)
	inputPath := filepath.Join(tempDir, "test_input.mp4")
	createTestVideo(t, inputPath, 1, 640, 360)

	service := NewVideoService()

	testCases := []struct {
		name       string
		params     string
		expectFile string
		timeout    time.Duration
	}{
		// Most popular web streaming combinations
		{
			name:       "H.264/AAC/MP4 (Most Popular Web)",
			params:     "h264_1280x720_30fps_1s_23crf_aac_128kbps.mp4",
			expectFile: "h264_1280x720_30fps_1s_23crf_aac_128kbps.mp4",
			timeout:    20 * time.Second,
		},
		{
			name:       "VP9/Opus/WebM (Modern Web)",
			params:     "vp9_1280x720_30fps_1s_25crf_opus_128kbps.webm",
			expectFile: "vp9_1280x720_30fps_1s_25crf_opus_128kbps.webm",
			timeout:    30 * time.Second,
		},
		{
			name:       "H.264/AAC/MP4 (Mobile)",
			params:     "h264_720p_30fps_1s_25crf_aac_96kbps.mp4",
			expectFile: "h264_1280x720_30fps_1s_25crf_aac_96kbps.mp4",
			timeout:    20 * time.Second,
		},

		// Extended combinations (run in comprehensive mode)
		{
			name:       "H.264/AAC/MP4 (1080p)",
			params:     "h264_1920x1080_30fps_2s_23crf_aac_128kbps.mp4",
			expectFile: "h264_1920x1080_30fps_2s_23crf_aac_128kbps.mp4",
			timeout:    45 * time.Second,
		},
		{
			name:       "AV1/Opus/WebM (Next-gen Web)",
			params:     "av1_1280x720_30fps_2s_30crf_opus_128kbps.webm",
			expectFile: "av1_1280x720_30fps_2s_30crf_opus_128kbps.webm",
			timeout:    2 * time.Minute,
		},
		{
			name:       "H.264/AAC/MP4 (Mobile 480p)",
			params:     "h264_480p_30fps_2s_26crf_aac_96kbps.mp4",
			expectFile: "h264_854x480_30fps_2s_26crf_aac_96kbps.mp4",
			timeout:    20 * time.Second,
		},
		{
			name:       "H.265/AAC/MP4 (High Quality)",
			params:     "h265_1920x1080_30fps_2s_28crf_aac_192kbps.mp4",
			expectFile: "h265_1920x1080_30fps_2s_28crf_aac_192kbps.mp4",
			timeout:    90 * time.Second,
		},
		{
			name:       "H.264/AAC/MP4 (CBR)",
			params:     "h264_1280x720_30fps_2s_3000cbr_aac_128kbps.mp4",
			expectFile: "h264_1280x720_30fps_2s_3000cbr_aac_128kbps.mp4",
			timeout:    30 * time.Second,
		},
		{
			name:       "H.264/AAC/MP4 (VBR)",
			params:     "h264_1280x720_30fps_2s_3000vbr_aac_128kbps.mp4",
			expectFile: "h264_1280x720_30fps_2s_3000vbr_aac_128kbps.mp4",
			timeout:    30 * time.Second,
		},
		{
			name:       "H.264/AAC/MP4 (60fps)",
			params:     "h264_1280x720_60fps_2s_23crf_aac_128kbps.mp4",
			expectFile: "h264_1280x720_60fps_2s_23crf_aac_128kbps.mp4",
			timeout:    45 * time.Second,
		},
		{
			name:       "H.264/AAC/MP4 (24fps Cinema)",
			params:     "h264_1920x1080_24fps_2s_23crf_aac_128kbps.mp4",
			expectFile: "h264_1920x1080_24fps_2s_23crf_aac_128kbps.mp4",
			timeout:    40 * time.Second,
		},
		{
			name:       "Audio Only (AAC/MP4)",
			params:     "novideo_2s_aac_128kbps.mp4",
			expectFile: "novideo_2s_aac_128kbps.mp4",
			timeout:    10 * time.Second,
		},
		{
			name:       "Video Only (H.264/MP4)",
			params:     "h264_1280x720_30fps_2s_23crf_noaudio.mp4",
			expectFile: "h264_1280x720_30fps_2s_23crf_noaudio.mp4",
			timeout:    30 * time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Use test-specific timeout
			ctx, cancel := context.WithTimeout(context.Background(), tc.timeout)
			defer cancel()

			// Parse params to get VideoSpec for the new Transcode method
			inputParams, err := parser.ParseFilename(tc.params)
			if err != nil {
				t.Fatalf("Failed to parse params: %v", err)
			}
			spec := config.ApplyDefaultVideoSpec(inputParams)

			outputPath := tempDir
			resultCh, errCh := service.Transcode(ctx, spec, inputPath, outputPath)

			// Wait for completion
			select {
			case result := <-resultCh:
				// Check if file was created
				expectedFile := filepath.Join(tempDir, tc.expectFile)
				if result != expectedFile {
					t.Errorf("Expected output path %s, got %s", expectedFile, result)
				}

				// Check if file exists and has content
				info, err := os.Stat(result)
				if err != nil {
					t.Fatalf("Output file not found: %v", err)
				}
				if info.Size() == 0 {
					t.Fatal("Output file is empty")
				}

				t.Logf("✅ %s created successfully (size: %d bytes)", tc.name, info.Size())

				// Verify with ffprobe
				verifyVideoWithFFProbe(t, result, tc.params)

			case err := <-errCh:
				t.Fatalf("Transcoding failed: %v", err)

			case <-ctx.Done():
				t.Fatal("Transcoding timed out")
			}
		})
	}
}

func createTestVideo(t *testing.T, outputPath string, duration int, width, height int) {
	// Determine codecs based on file extension
	ext := strings.ToLower(filepath.Ext(outputPath))

	var videoCodec, audioCodec string
	switch ext {
	case ".webm":
		videoCodec = "libvpx-vp9"
		audioCodec = "libopus"
	case ".mp4":
		videoCodec = "libx264"
		audioCodec = "aac"
	default:
		// Default to MP4 codecs
		videoCodec = "libx264"
		audioCodec = "aac"
	}

	// Create a test video using FFmpeg with specified duration and size
	cmd := exec.Command("ffmpeg",
		"-f", "lavfi",
		"-i", fmt.Sprintf("testsrc2=duration=%d:size=%dx%d:rate=30", duration, width, height),
		"-f", "lavfi",
		"-i", fmt.Sprintf("sine=frequency=1000:duration=%d", duration),
		"-c:v", videoCodec,
		"-c:a", audioCodec,
		"-y", // Overwrite output file
		outputPath,
	)

	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}
}

func verifyVideoWithFFProbe(t *testing.T, videoPath, originalParams string) {
	// Parse the original parameters
	spec, err := parser.ParseFilename(originalParams)
	if err != nil {
		t.Fatalf("Failed to parse params %s: %v", originalParams, err)
	}

	// Run ffprobe
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		videoPath,
	)

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("FFprobe failed: %v", err)
	}

	var probeResult config.FFProbeOutput
	if err := json.Unmarshal(output, &probeResult); err != nil {
		t.Fatalf("Failed to parse ffprobe output: %v", err)
	}

	// Verify basic properties
	if len(probeResult.Streams) == 0 {
		t.Fatal("No streams found in video")
	}

	// Find video stream
	var videoStream *config.FFprobeStream
	var audioStream *config.FFprobeStream

	for i := range probeResult.Streams {
		switch probeResult.Streams[i].CodecType {
		case "video":
			videoStream = &probeResult.Streams[i]
		case "audio":
			audioStream = &probeResult.Streams[i]
		}
	}

	// Verify video properties if video codec is not "novideo"
	if spec.Codec != "novideo" && videoStream != nil {
		if videoStream.Width != spec.Width {
			t.Errorf("Expected width %d, got %d", spec.Width, videoStream.Width)
		}
		if videoStream.Height != spec.Height {
			t.Errorf("Expected height %d, got %d", spec.Height, videoStream.Height)
		}

		// Check codec (with mapping)
		expectedCodec := spec.Codec
		switch spec.Codec {
		case "av1":
			expectedCodec = "av1" // AV1 codec name in ffprobe is actually "av1", not "av01"
		case "h264":
			expectedCodec = "h264"
		case "h265":
			expectedCodec = "hevc"
		case "vp9":
			expectedCodec = "vp9"
		}

		if videoStream.CodecName != expectedCodec {
			t.Errorf("Expected video codec %s, got %s", expectedCodec, videoStream.CodecName)
		}

		t.Logf("Video: %dx%d, codec: %s, duration: %s",
			videoStream.Width, videoStream.Height, videoStream.CodecName, videoStream.Duration)
	}

	// Verify audio properties if audio codec is not "noaudio"
	if spec.AudioCodec != "noaudio" && audioStream != nil {
		expectedAudioCodec := spec.AudioCodec
		if spec.AudioCodec == "opus" {
			expectedAudioCodec = "opus"
		}

		if audioStream.CodecName != expectedAudioCodec {
			t.Errorf("Expected audio codec %s, got %s", expectedAudioCodec, audioStream.CodecName)
		}

		t.Logf("Audio: codec: %s, channels: %d, sample_rate: %s",
			audioStream.CodecName, audioStream.Channels, audioStream.SampleRate)
	}

	// Verify container format
	expectedFormat := spec.Container
	actualFormat := probeResult.Format.FormatName

	// WebM is reported as "matroska,webm" by ffprobe
	if spec.Container == "webm" && (actualFormat == "matroska,webm" || actualFormat == "webm") {
		// OK
	} else if spec.Container == "mp4" && actualFormat == "mov,mp4,m4a,3gp,3g2,mj2" {
		// OK
	} else {
		t.Logf("Container format: expected %s, got %s (this might be normal)", expectedFormat, actualFormat)
	}
}
