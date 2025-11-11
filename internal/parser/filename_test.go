package parser

import (
	"kittens/internal/config"
	"testing"
)

func TestParseFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     *config.VideoSpec
	}{
		{
			name:     "full specification with CRF",
			filename: "av1_1280x720_30fps_60s_23crf_aac_128kbps",
			want: &config.VideoSpec{
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
			filename: "h264_1920x1080_60fps_30s_5000cbr_opus_192kbps.mp4",
			want: &config.VideoSpec{
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
			filename: "vp9_1280x720_30fps_60s_25crf_opus_128kbps.webm",
			want: &config.VideoSpec{
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
			filename: "h265_3840x2160_24fps_120s_8000vbr_aac_256kbps",
			want: &config.VideoSpec{
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
			filename: "vp9_720p_30fps_60s_25crf_vorbis_128kbps",
			want: &config.VideoSpec{
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
			filename: "av1_4k_60fps_120s_28crf_opus_192kbps",
			want: &config.VideoSpec{
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
			filename: "h264_1280x720_30fps_10s_23crf_noaudio",
			want: &config.VideoSpec{
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
			filename: "novideo_60s_aac_128kbps",
			want: &config.VideoSpec{
				Duration:     60,
				Codec:        "novideo",
				AudioCodec:   "aac",
				AudioBitrate: 128,
			},
		},
		{
			name:     "different order",
			filename: "30fps_av1_60s_1280x720_23crf_128kbps_aac",
			want: &config.VideoSpec{
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
	tests := []struct {
		name string
		spec *config.VideoSpec
		want string
	}{
		{
			name: "full specification with mp4",
			spec: &config.VideoSpec{
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
			want: "av1_1280x720_30fps_60s_23crf_aac_128kbps.mp4",
		},
		{
			name: "webm container",
			spec: &config.VideoSpec{
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
			want: "vp9_1920x1080_60fps_30s_25crf_opus_192kbps.webm",
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
				Width:        1280,
				Height:       720,
				Codec:        "h264",
				Bitrate:      "5000cbr",
				AudioCodec:   "aac",
				AudioBitrate: 128,
				Container:    "mp4",
			},
			want: "h264_1280x720_5000cbr_aac_128kbps.mp4",
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
	// Test that parsing a generated filename gives back the same spec
	originalSpec := &config.VideoSpec{
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
