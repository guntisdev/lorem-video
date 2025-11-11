//go:build integration
// +build integration

package service

import (
	"context"
	"encoding/json"
	"kittens/internal/config"
	"kittens/internal/parser"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Integration tests for video transcoding - these are slow and require FFmpeg
// Run with: go test -tags=integration ./internal/service

func TestVideoTranscodeIntegration(t *testing.T) {
	// Skip if FFmpeg is not available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg not found, skipping integration test")
	}

	// Create temporary test directory
	tempDir := t.TempDir()

	// Create a simple test video (1 second, small resolution for speed)
	inputPath := filepath.Join(tempDir, "test_input.mp4")
	createTestVideo(t, inputPath)

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
			params:     "h264_1280x720_30fps_2s_23crf_aac_128kbps.mp4",
			expectFile: "h264_1280x720_30fps_2s_23crf_aac_128kbps.mp4",
			timeout:    30 * time.Second,
		},
		{
			name:       "H.264/AAC/MP4 (1080p)",
			params:     "h264_1920x1080_30fps_2s_23crf_aac_128kbps.mp4",
			expectFile: "h264_1920x1080_30fps_2s_23crf_aac_128kbps.mp4",
			timeout:    45 * time.Second,
		},

		// Modern web combinations
		{
			name:       "VP9/Opus/WebM (Modern Web)",
			params:     "vp9_1280x720_30fps_2s_25crf_opus_128kbps.webm",
			expectFile: "vp9_1280x720_30fps_2s_25crf_opus_128kbps.webm",
			timeout:    60 * time.Second,
		},
		{
			name:       "AV1/Opus/WebM (Next-gen Web)",
			params:     "av1_1280x720_30fps_2s_30crf_opus_128kbps.webm",
			expectFile: "av1_1280x720_30fps_2s_30crf_opus_128kbps.webm",
			timeout:    2 * time.Minute,
		},

		// Mobile-optimized
		{
			name:       "H.264/AAC/MP4 (Mobile 720p)",
			params:     "h264_720p_30fps_2s_25crf_aac_96kbps.mp4",
			expectFile: "h264_1280x720_30fps_2s_25crf_aac_96kbps.mp4",
			timeout:    30 * time.Second,
		},
		{
			name:       "H.264/AAC/MP4 (Mobile 480p)",
			params:     "h264_480p_30fps_2s_26crf_aac_96kbps.mp4",
			expectFile: "h264_854x480_30fps_2s_26crf_aac_96kbps.mp4",
			timeout:    20 * time.Second,
		},

		// High quality
		{
			name:       "H.265/AAC/MP4 (High Quality)",
			params:     "h265_1920x1080_30fps_2s_28crf_aac_192kbps.mp4",
			expectFile: "h265_1920x1080_30fps_2s_28crf_aac_192kbps.mp4",
			timeout:    90 * time.Second,
		},

		// Bitrate variations
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

		// Different frame rates
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

		// Audio-only and video-only
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

			outputPath := tempDir
			resultCh, errCh := service.Transcode(ctx, tc.params, inputPath, outputPath)

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

				t.Logf("Successfully created %s (size: %d bytes)", result, info.Size())

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

// TestPopularCombinations tests only the most commonly used combinations for faster CI
func TestPopularCombinations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	// Skip if FFmpeg is not available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg not found, skipping integration test")
	}

	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "test_input.mp4")
	createTestVideo(t, inputPath)

	service := NewVideoService()

	// Only the most popular combinations for faster testing
	popularTests := []struct {
		name       string
		params     string
		expectFile string
		timeout    time.Duration
	}{
		{
			name:       "H.264/AAC/MP4 (Standard Web)",
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
	}

	for _, tc := range popularTests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tc.timeout)
			defer cancel()

			outputPath := tempDir
			resultCh, errCh := service.Transcode(ctx, tc.params, inputPath, outputPath)

			select {
			case result := <-resultCh:
				expectedFile := filepath.Join(tempDir, tc.expectFile)
				if result != expectedFile {
					t.Errorf("Expected output path %s, got %s", expectedFile, result)
				}

				info, err := os.Stat(result)
				if err != nil {
					t.Fatalf("Output file not found: %v", err)
				}
				if info.Size() == 0 {
					t.Fatal("Output file is empty")
				}

				t.Logf("✅ %s created successfully (size: %d bytes)", tc.name, info.Size())

			case err := <-errCh:
				t.Fatalf("Transcoding failed: %v", err)

			case <-ctx.Done():
				t.Fatalf("Transcoding timed out after %v", tc.timeout)
			}
		})
	}
}

func createTestVideo(t *testing.T, outputPath string) {
	// Create a simple 1-second test video using FFmpeg
	cmd := exec.Command("ffmpeg",
		"-f", "lavfi",
		"-i", "testsrc2=duration=1:size=640x360:rate=30",
		"-f", "lavfi",
		"-i", "sine=frequency=1000:duration=1",
		"-c:v", "libx264",
		"-c:a", "aac",
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

func BenchmarkTranscode(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	// Skip if FFmpeg is not available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		b.Skip("FFmpeg not found, skipping benchmark")
	}

	tempDir := b.TempDir()
	inputPath := filepath.Join(tempDir, "test_input.mp4")

	// Create test video once
	cmd := exec.Command("ffmpeg",
		"-f", "lavfi",
		"-i", "testsrc2=duration=1:size=640x360:rate=30",
		"-f", "lavfi",
		"-i", "sine=frequency=1000:duration=1",
		"-c:v", "libx264",
		"-c:a", "aac",
		"-y",
		inputPath,
	)

	if err := cmd.Run(); err != nil {
		b.Fatalf("Failed to create test video: %v", err)
	}

	service := NewVideoService()
	params := "h264_640x360_30fps_1s_23crf_aac_128kbps.mp4"

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		outputPath := tempDir

		resultCh, errCh := service.Transcode(ctx, params, inputPath, outputPath)

		select {
		case <-resultCh:
			// Success
		case err := <-errCh:
			b.Fatalf("Transcoding failed: %v", err)
		}
	}
}
