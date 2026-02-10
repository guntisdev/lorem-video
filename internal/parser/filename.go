package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"lorem.video/internal/config"
)

var resolutionRegex = regexp.MustCompile(`^(\d+)x(\d+)$`) // 1280x720
var durationRegex = regexp.MustCompile(`^(\d+)s$`)        // 60s
var crfRegex = regexp.MustCompile(`^(\d+)crf$`)           // constant rate factor 23
var cbrRegex = regexp.MustCompile(`^(\d+)cbr$`)           // constant bitrate 3000
var vbrRegex = regexp.MustCompile(`^(\d+)vbr$`)           // variable bitrate 3000
var audioBitrateRegex = regexp.MustCompile(`^(\d+)kbps$`) // 128kbps

var mockSourceFiles []string

func SetMockSourceFiles(files []string) {
	mockSourceFiles = files
}
func ClearMockSourceFiles() {
	mockSourceFiles = nil
}

func getSourceFileNames() []string {
	// Use mock files if available (for testing)
	if mockSourceFiles != nil {
		return mockSourceFiles
	}

	// Try to get real source files
	var sourceFiles []string
	if files, err := config.GetSourceVideoFiles(); err == nil {
		for _, file := range files {
			baseName := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
			sourceFiles = append(sourceFiles, baseName)
		}
	}
	return sourceFiles
}

// Example: bunny_av1_1280x720_30fps_60s_23crf_aac_128kbps.mp4
func ParseFilename(filename string) (*config.VideoSpec, error) {
	// Extract extension/container
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != "" {
		ext = ext[1:] // Remove the dot
	}

	if ext != "" && !slices.Contains(config.ValidContainers, ext) {
		return nil, fmt.Errorf("invalid container format: %s (valid formats: %v)", ext, config.ValidContainers)
	}

	// Get source file names (using mocks if available for testing)
	sourceFiles := getSourceFileNames()

	filename = strings.TrimSuffix(filename, filepath.Ext(filename))
	parts := strings.Split(filename, "_")
	params := &config.VideoSpec{}

	if ext != "" {
		params.Container = ext
	}

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
			} else if slices.Contains(sourceFiles, part) {
				params.Name = part
			}

		}
	}

	return params, nil
}

// GenerateFilename creates a filename string from VideoSpec
// Example output: bunny_av1_1280x720_30fps_60s_23crf_aac_128kbps.mp4
func GenerateFilename(spec *config.VideoSpec) string {
	var parts []string

	if spec.Name != "" {
		parts = append(parts, spec.Name)
	}

	if spec.Codec != "" {
		parts = append(parts, spec.Codec)
	}

	if spec.Width > 0 && spec.Height > 0 && spec.Codec != "novideo" {
		parts = append(parts, fmt.Sprintf("%dx%d", spec.Width, spec.Height))
	}

	if spec.FPS > 0 && spec.Codec != "novideo" {
		parts = append(parts, fmt.Sprintf("%dfps", spec.FPS))
	}

	if spec.Duration > 0 {
		parts = append(parts, fmt.Sprintf("%ds", spec.Duration))
	}

	if spec.Bitrate != "" && spec.Codec != "novideo" {
		parts = append(parts, spec.Bitrate)
	}

	if spec.AudioCodec != "" {
		parts = append(parts, spec.AudioCodec)
	}

	if spec.AudioBitrate > 0 && spec.AudioCodec != "noaudio" {
		parts = append(parts, fmt.Sprintf("%dkbps", spec.AudioBitrate))
	}

	filename := strings.Join(parts, "_")

	// Add container extension if specified
	if spec.Container != "" {
		filename = fmt.Sprintf("%s.%s", filename, spec.Container)
	}

	return filename
}

func FindExistingVideo(filename string, spec *config.VideoSpec) string {
	// Search in video/ pregen folder
	pregenPath := filepath.Join(config.AppPaths.Video, spec.Name, filename)
	if _, err := os.Stat(pregenPath); err == nil {
		return pregenPath
	}

	// Search in tmp folder
	tmpPath := filepath.Join(config.AppPaths.Tmp, filename)
	if _, err := os.Stat(tmpPath); err == nil {
		return tmpPath
	}

	return ""
}
