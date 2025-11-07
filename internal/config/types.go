package config

import (
	"fmt"
	"strconv"
	"strings"
)

type VideoSpec struct {
	Width        int
	Height       int
	Duration     int // seconds
	Codec        string
	FPS          int
	Bitrate      string // "23crf", "3000cbr", or "3000vbr"
	AudioCodec   string
	AudioBitrate int // kbps
}

var DefaultVideoSpec = VideoSpec{
	Width:        1280,
	Height:       720,
	Duration:     20,
	Codec:        "h264",
	FPS:          30,
	Bitrate:      "23crf",
	AudioCodec:   "aac",
	AudioBitrate: 128,
}

var ValidVideoCodecs = []string{"h264", "h265", "av1", "vp9", "novideo"}   // novideo - shoul be mapped to "none" for ffmpeg
var ValidAudioCodecs = []string{"aac", "opus", "mp3", "vorbis", "noaudio"} // noaudio - shoul be mapped to "none" for ffmpeg

type Resolution struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

var Resolutions = map[string]Resolution{
	"240p":  {426, 240},
	"360p":  {640, 360},
	"480p":  {854, 480},
	"720p":  {1280, 720},
	"1080p": {1920, 1080},
	"1440p": {2560, 1440},
	"4k":    {3840, 2160},
}

const (
	MinDimension = 64
	MaxDimension = 3840 // 4K
)

func ApplyDefaultVideoSpec(input *VideoSpec) VideoSpec {
	result := DefaultVideoSpec
	if input.Width != 0 {
		result.Width = input.Width
	}
	if input.Height != 0 {
		result.Height = input.Height
	}
	if input.Duration != 0 {
		result.Duration = input.Duration
	}
	if input.Codec != "" {
		result.Codec = input.Codec
	}
	if input.FPS != 0 {
		result.FPS = input.FPS
	}
	if input.Bitrate != "" {
		result.Bitrate = input.Bitrate
	}
	if input.AudioCodec != "" {
		result.AudioCodec = input.AudioCodec
	}
	if input.AudioBitrate != 0 {
		result.AudioBitrate = input.AudioBitrate
	}
	return result
}

// ParseResolution parses "720p" or "640x360" format
func ParseResolution(s string) (Resolution, error) {
	// Try predefined resolutions first
	if res, ok := Resolutions[s]; ok {
		return res, nil
	}

	// Try parsing WxH format
	parts := strings.Split(s, "x")
	if len(parts) != 2 {
		return Resolution{}, fmt.Errorf("invalid resolution format: %s", s)
	}

	width, err := strconv.Atoi(parts[0])
	if err != nil {
		return Resolution{}, fmt.Errorf("invalid width: %s", parts[0])
	}

	height, err := strconv.Atoi(parts[1])
	if err != nil {
		return Resolution{}, fmt.Errorf("invalid height: %s", parts[1])
	}

	if width < MinDimension || width > MaxDimension || height < MinDimension || height > MaxDimension {
		return Resolution{}, fmt.Errorf("resolution out of bounds: %dx%d", width, height)
	}

	return Resolution{Width: width, Height: height}, nil
}
