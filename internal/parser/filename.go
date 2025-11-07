package parser

import (
	"kittens/internal/config"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var resolutionRegex = regexp.MustCompile(`^(\d+)x(\d+)$`) // 1280x720
var durationRegex = regexp.MustCompile(`^(\d+)s$`)        // 60s
var crfRegex = regexp.MustCompile(`^(\d+)crf$`)           // constant rate factor 23
var cbrRegex = regexp.MustCompile(`^(\d+)cbr$`)           // constant bitrate 3000
var vbrRegex = regexp.MustCompile(`^(\d+)vbr$`)           // variable bitrate 3000
var audioBitrateRegex = regexp.MustCompile(`^(\d+)kbps$`) // 128kbps

// Example: av1_1280x720_30fps_60s_23crf_aac_128kbps
func ParseFilename(filename string) (*config.VideoSpec, error) {
	filename = strings.TrimSuffix(filename, ".mp4")
	parts := strings.Split(filename, "_")
	params := &config.VideoSpec{}

	for _, part := range parts {
		switch {

		case resolutionRegex.MatchString(part):
			matches := resolutionRegex.FindStringSubmatch(part)
			if len(matches) == 3 {
				width, err1 := strconv.Atoi(matches[1])
				height, err2 := strconv.Atoi(matches[2])
				if err1 == nil && err2 == nil {
					params.Width = width
					params.Height = height
				}
			}

		case strings.HasSuffix(part, "fps"):
			fpsStr := strings.TrimSuffix(part, "fps")
			if fps, err := strconv.Atoi(fpsStr); err == nil {
				params.FPS = fps
			}

		case durationRegex.MatchString(part):
			durationStr := strings.TrimSuffix(part, "s")
			if duration, err := strconv.Atoi(durationStr); err == nil {
				params.Duration = duration
			}

		case crfRegex.MatchString(part):
			params.Bitrate = part

		case cbrRegex.MatchString(part):
			params.Bitrate = part

		case vbrRegex.MatchString(part):
			params.Bitrate = part

		case audioBitrateRegex.MatchString(part):
			audioBitrateStr := strings.TrimSuffix(part, "kbps")
			if audioBitrate, err := strconv.Atoi(audioBitrateStr); err == nil {
				params.AudioBitrate = audioBitrate
			}

		default:
			if res, ok := config.Resolutions[part]; ok {
				params.Width = res.Width
				params.Height = res.Height
			} else if slices.Contains(config.ValidVideoCodecs, part) {
				params.Codec = part
			} else if slices.Contains(config.ValidAudioCodecs, part) {
				params.AudioCodec = part
			}

		}
	}

	return params, nil
}

