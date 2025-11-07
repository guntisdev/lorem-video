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
			name:     "with CBR bitrate",
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
		})
	}
}

func TestParseFilenameEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		filename string
	}{
		{
			name:     "empty filename",
			filename: "",
		},
		{
			name:     "only extension",
			filename: ".mp4",
		},
		{
			name:     "invalid parts",
			filename: "invalid_parts_here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFilename(tt.filename)
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
