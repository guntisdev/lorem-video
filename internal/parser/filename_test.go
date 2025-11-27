package parser

import (
	"testing"

	"lorem.video/internal/config"
)

func TestParseFilename(t *testing.T) {
	// Setup mock source files for testing
	SetMockSourceFiles([]string{"bunny", "elephant", "cat"})
	defer ClearMockSourceFiles()

	tests := []struct {
		name     string
		filename string
		want     *config.VideoSpec
	}{
		{
			name:     "full specification with CRF",
			filename: "bunny_av1_1280x720_30fps_60s_23crf_aac_128kbps",
			want: &config.VideoSpec{
				Name:         "bunny",
				Width:        1280,
				Height:       720,
				FPS:          30,
				Duration:     60,
				Codec:        "av1",
				Bitrate:      "23crf",
				AudioCodec:   "aac",
				AudioBitrate: 128,
			},
		},
		{
			name:     "with CBR bitrate and mp4 extension",
			filename: "cat_h264_1920x1080_60fps_30s_5000cbr_opus_192kbps.mp4",
			want: &config.VideoSpec{
				Name:         "cat",
				Width:        1920,
				Height:       1080,
				FPS:          60,
				Duration:     30,
				Codec:        "h264",
				Bitrate:      "5000cbr",
				AudioCodec:   "opus",
				AudioBitrate: 192,
				Container:    "mp4",
			},
		},
		{
			name:     "with webm extension",
			filename: "bunny_vp9_1280x720_30fps_60s_25crf_opus_128kbps.webm",
			want: &config.VideoSpec{
				Name:         "bunny",
				Width:        1280,
				Height:       720,
				FPS:          30,
				Duration:     60,
				Codec:        "vp9",
				Bitrate:      "25crf",
				AudioCodec:   "opus",
				AudioBitrate: 128,
				Container:    "webm",
			},
		},
		{
			name:     "with VBR bitrate",
			filename: "bunny_h265_3840x2160_24fps_120s_8000vbr_aac_256kbps",
			want: &config.VideoSpec{
				Name:         "bunny",
				Width:        3840,
				Height:       2160,
				FPS:          24,
				Duration:     120,
				Codec:        "h265",
				Bitrate:      "8000vbr",
				AudioCodec:   "aac",
				AudioBitrate: 256,
			},
		},
		{
			name:     "resolution preset 720p",
			filename: "bunny_vp9_720p_30fps_60s_25crf_vorbis_128kbps",
			want: &config.VideoSpec{
				Name:         "bunny",
				Width:        1280,
				Height:       720,
				FPS:          30,
				Duration:     60,
				Codec:        "vp9",
				Bitrate:      "25crf",
				AudioCodec:   "vorbis",
				AudioBitrate: 128,
			},
		},
		{
			name:     "resolution preset 4k",
			filename: "bunny_av1_4k_60fps_120s_28crf_opus_192kbps",
			want: &config.VideoSpec{
				Name:         "bunny",
				Width:        3840,
				Height:       2160,
				FPS:          60,
				Duration:     120,
				Codec:        "av1",
				Bitrate:      "28crf",
				AudioCodec:   "opus",
				AudioBitrate: 192,
			},
		},
		{
			name:     "no audio",
			filename: "bunny_h264_1280x720_30fps_10s_23crf_noaudio",
			want: &config.VideoSpec{
				Name:       "bunny",
				Width:      1280,
				Height:     720,
				FPS:        30,
				Duration:   10,
				Codec:      "h264",
				Bitrate:    "23crf",
				AudioCodec: "noaudio",
			},
		},
		{
			name:     "no video",
			filename: "bunny_novideo_60s_aac_128kbps",
			want: &config.VideoSpec{
				Name:         "bunny",
				Duration:     60,
				Codec:        "novideo",
				AudioCodec:   "aac",
				AudioBitrate: 128,
			},
		},
		{
			name:     "different order",
			filename: "bunny_30fps_av1_60s_1280x720_23crf_128kbps_aac",
			want: &config.VideoSpec{
				Name:         "bunny",
				Width:        1280,
				Height:       720,
				FPS:          30,
				Duration:     60,
				Codec:        "av1",
				Bitrate:      "23crf",
				AudioCodec:   "aac",
				AudioBitrate: 128,
			},
		},
		{
			name:     "invalid source video name falls back to default",
			filename: "invalidvideo_h264_1280x720_30fps_60s_23crf_aac_128kbps.mp4",
			want: &config.VideoSpec{
				Name:         "", // Should be empty since invalidvideo is not in mock source files
				Width:        1280,
				Height:       720,
				FPS:          30,
				Duration:     60,
				Codec:        "h264",
				Bitrate:      "23crf",
				AudioCodec:   "aac",
				AudioBitrate: 128,
				Container:    "mp4",
			},
		},
		{
			name:     "minimal specification",
			filename: "h264_720p",
			want: &config.VideoSpec{
				Width:  1280,
				Height: 720,
				Codec:  "h264",
			},
		},
		{
			name:     "duplicating_parts",
			filename: "720p_h264_240p",
			want: &config.VideoSpec{
				Width:  426,
				Height: 240,
				Codec:  "h264",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFilename(tt.filename)
			if err != nil {
				t.Errorf("ParseFilename() error = %v", err)
				return
			}

			if got.Width != tt.want.Width {
				t.Errorf("Width = %v, want %v", got.Width, tt.want.Width)
			}
			if got.Height != tt.want.Height {
				t.Errorf("Height = %v, want %v", got.Height, tt.want.Height)
			}
			if got.Name != tt.want.Name {
				t.Errorf("Name = %v, want %v", got.Name, tt.want.Name)
			}
			if got.FPS != tt.want.FPS {
				t.Errorf("FPS = %v, want %v", got.FPS, tt.want.FPS)
			}
			if got.Duration != tt.want.Duration {
				t.Errorf("Duration = %v, want %v", got.Duration, tt.want.Duration)
			}
			if got.Codec != tt.want.Codec {
				t.Errorf("Codec = %v, want %v", got.Codec, tt.want.Codec)
			}
			if got.Bitrate != tt.want.Bitrate {
				t.Errorf("Bitrate = %v, want %v", got.Bitrate, tt.want.Bitrate)
			}
			if got.AudioCodec != tt.want.AudioCodec {
				t.Errorf("AudioCodec = %v, want %v", got.AudioCodec, tt.want.AudioCodec)
			}
			if got.AudioBitrate != tt.want.AudioBitrate {
				t.Errorf("AudioBitrate = %v, want %v", got.AudioBitrate, tt.want.AudioBitrate)
			}
			if got.Container != tt.want.Container {
				t.Errorf("Container = %v, want %v", got.Container, tt.want.Container)
			}
		})
	}
}

func TestParseFilenameEdgeCases(t *testing.T) {
	// Setup mock source files for testing
	SetMockSourceFiles([]string{"bunny", "elephant", "cat"})
	defer ClearMockSourceFiles()
	tests := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{
			name:     "empty filename",
			filename: "",
			wantErr:  false,
		},
		{
			name:     "only extension",
			filename: ".mp4",
			wantErr:  false,
		},
		{
			name:     "invalid parts",
			filename: "invalid_parts_here",
			wantErr:  false,
		},
		{
			name:     "invalid container format",
			filename: "h264_1280x720_30fps_60s_23crf_aac_128kbps.avi",
			wantErr:  true,
		},
		{
			name:     "invalid container format mkv",
			filename: "h264_1280x720_30fps_60s_23crf_aac_128kbps.mkv",
			wantErr:  true,
		},
		{
			name:     "invalid source video name uses default",
			filename: "invalidvideo_h264_1280x720_30fps_60s_23crf_aac_128kbps.mp4",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFilename(tt.filename)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseFilename() expected error but got none")
				}
				return
			}

			// Should not error, just return partially filled struct
			if err != nil {
				t.Errorf("ParseFilename() unexpected error = %v", err)
			}
			if got == nil {
				t.Errorf("ParseFilename() returned nil")
			}
		})
	}
}

func TestGenerateFilename(t *testing.T) {
	// Setup mock source files for testing
	SetMockSourceFiles([]string{"bunny", "elephant", "cat"})
	defer ClearMockSourceFiles()
	tests := []struct {
		name string
		spec *config.VideoSpec
		want string
	}{
		{
			name: "full specification with mp4",
			spec: &config.VideoSpec{
				Name:         "bunny",
				Width:        1280,
				Height:       720,
				FPS:          30,
				Duration:     60,
				Codec:        "av1",
				Bitrate:      "23crf",
				AudioCodec:   "aac",
				AudioBitrate: 128,
				Container:    "mp4",
			},
			want: "bunny_av1_1280x720_30fps_60s_23crf_aac_128kbps.mp4",
		},
		{
			name: "webm container",
			spec: &config.VideoSpec{
				Name:         "bunny",
				Width:        1920,
				Height:       1080,
				FPS:          60,
				Duration:     30,
				Codec:        "vp9",
				Bitrate:      "25crf",
				AudioCodec:   "opus",
				AudioBitrate: 192,
				Container:    "webm",
			},
			want: "bunny_vp9_1920x1080_60fps_30s_25crf_opus_192kbps.webm",
		},
		{
			name: "minimal specification",
			spec: &config.VideoSpec{
				Width:  1280,
				Height: 720,
				Codec:  "h264",
			},
			want: "h264_1280x720",
		},
		{
			name: "no video codec",
			spec: &config.VideoSpec{
				Duration:     60,
				AudioCodec:   "aac",
				AudioBitrate: 128,
				Container:    "mp4",
			},
			want: "60s_aac_128kbps.mp4",
		},
		{
			name: "cbr bitrate",
			spec: &config.VideoSpec{
				Name:         "bunny",
				Width:        1280,
				Height:       720,
				Codec:        "h264",
				Bitrate:      "5000cbr",
				AudioCodec:   "aac",
				AudioBitrate: 128,
				Container:    "mp4",
			},
			want: "bunny_h264_1280x720_5000cbr_aac_128kbps.mp4",
		},
		{
			name: "empty spec",
			spec: &config.VideoSpec{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateFilename(tt.spec)
			if got != tt.want {
				t.Errorf("GenerateFilename() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRoundTripFilename(t *testing.T) {
	// Setup mock source files for testing
	SetMockSourceFiles([]string{"bunny", "elephant", "cat"})
	defer ClearMockSourceFiles()
	// Test that parsing a generated filename gives back the same spec
	originalSpec := &config.VideoSpec{
		Name:         "bunny",
		Width:        1280,
		Height:       720,
		FPS:          30,
		Duration:     60,
		Codec:        "av1",
		Bitrate:      "23crf",
		AudioCodec:   "aac",
		AudioBitrate: 128,
		Container:    "mp4",
	}

	filename := GenerateFilename(originalSpec)
	parsedSpec, err := ParseFilename(filename)

	if err != nil {
		t.Errorf("ParseFilename() error = %v", err)
		return
	}

	if parsedSpec.Name != originalSpec.Name {
		t.Errorf("Name = %v, want %v", parsedSpec.Name, originalSpec.Name)
	}
	if parsedSpec.Width != originalSpec.Width {
		t.Errorf("Width = %v, want %v", parsedSpec.Width, originalSpec.Width)
	}
	if parsedSpec.Height != originalSpec.Height {
		t.Errorf("Height = %v, want %v", parsedSpec.Height, originalSpec.Height)
	}
	if parsedSpec.FPS != originalSpec.FPS {
		t.Errorf("FPS = %v, want %v", parsedSpec.FPS, originalSpec.FPS)
	}
	if parsedSpec.Duration != originalSpec.Duration {
		t.Errorf("Duration = %v, want %v", parsedSpec.Duration, originalSpec.Duration)
	}
	if parsedSpec.Codec != originalSpec.Codec {
		t.Errorf("Codec = %v, want %v", parsedSpec.Codec, originalSpec.Codec)
	}
	if parsedSpec.Bitrate != originalSpec.Bitrate {
		t.Errorf("Bitrate = %v, want %v", parsedSpec.Bitrate, originalSpec.Bitrate)
	}
	if parsedSpec.AudioCodec != originalSpec.AudioCodec {
		t.Errorf("AudioCodec = %v, want %v", parsedSpec.AudioCodec, originalSpec.AudioCodec)
	}
	if parsedSpec.AudioBitrate != originalSpec.AudioBitrate {
		t.Errorf("AudioBitrate = %v, want %v", parsedSpec.AudioBitrate, originalSpec.AudioBitrate)
	}
	if parsedSpec.Container != originalSpec.Container {
		t.Errorf("Container = %v, want %v", parsedSpec.Container, originalSpec.Container)
	}
}

// Fuzz test to find edge cases in filename parsing
func FuzzParseFilename(f *testing.F) {
	// Seed with known good inputs
	f.Add("h264_1280x720_30fps_60s_23crf_aac_128kbps.mp4")
	f.Add("vp9_720p_30fps_10s_25crf_opus_128kbps.webm")
	f.Add("av1_4k_60fps_120s_28crf_noaudio.webm")
	f.Add("")
	f.Add("invalid")

	f.Fuzz(func(t *testing.T, filename string) {
		// ParseFilename should never crash, regardless of input
		spec, err := ParseFilename(filename)

		// If parsing succeeds, check invariants
		if err == nil && spec != nil {
			// Dimensions should never be negative
			if spec.Width < 0 {
				t.Errorf("Width should not be negative: %d", spec.Width)
			}
			if spec.Height < 0 {
				t.Errorf("Height should not be negative: %d", spec.Height)
			}

			// FPS should be reasonable
			if spec.FPS < 0 {
				t.Errorf("FPS should not be negative: %d", spec.FPS)
			}
			if spec.FPS > 1000 {
				t.Errorf("FPS seems unreasonable: %d", spec.FPS)
			}

			// Duration should be reasonable
			if spec.Duration < 0 {
				t.Errorf("Duration should not be negative: %d", spec.Duration)
			}

			// Audio bitrate should be reasonable
			if spec.AudioBitrate < 0 {
				t.Errorf("AudioBitrate should not be negative: %d", spec.AudioBitrate)
			}
			// Allow high bitrates for professional/archival audio (up to ~10MB/s)
			if spec.AudioBitrate > 10000000 {
				t.Errorf("AudioBitrate seems unreasonable: %d", spec.AudioBitrate)
			}

			// Container should be valid if specified
			if spec.Container != "" && spec.Container != "mp4" && spec.Container != "webm" {
				t.Errorf("Invalid container: %s", spec.Container)
			}
		}
	})
}
