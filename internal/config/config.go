package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Hls file naming
const (
	HLSMasterPlaylist = "playlist.m3u8"
	HLSMediaPlaylist  = "media.m3u8"
	HLSChunkFormat    = "chunk_%03d.mp4"
	HLSInit           = "init.mp4"
)

// WebSocket WebM stream file naming
const (
	WSInit        = "init.webm"
	WSChunkFormat = "chunk_%03d.webm"
	WSManifest    = "manifest.json"
)

const (
	WSLoopMs    = 6000 // must divide by frame duration (40ms) and opus frame (20ms)
	WSClusterMs = 1000
	WSFPS       = 25
)

type WSBitrate struct {
	Video int
	Audio int
}

// ordered low to high, drives both pregeneration and the manifest
var WSRenditions = []string{"360p", "480p", "720p", "1080p"}

var WSBitrates = map[string]WSBitrate{
	"360p":  {Video: 500, Audio: 96},
	"480p":  {Video: 800, Audio: 96},
	"720p":  {Video: 2000, Audio: 128},
	"1080p": {Video: 4000, Audio: 128},
}

var WSStreamSuffix = map[string]string{
	"360p":  "low",
	"480p":  "med",
	"720p":  "hi",
	"1080p": "hd",
}

type Paths struct {
	Data        string
	Video       string
	Stream      string
	WSStream    string
	SourceVideo string
	Logs        string
	LogsStats   string
	LogsBots    string
	LogsErrors  string
	Tmp         string

	DefaultSourceVideo string // bunny.mp4 path
}

var AppPaths = initPaths()

const Port = 3000

func GetBaseURL() string {
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%d", Port)
	}
	return baseURL
}

func GetWSBaseURL() string {
	base := GetBaseURL()
	if host, ok := strings.CutPrefix(base, "https://"); ok {
		return "wss://" + host
	}
	if host, ok := strings.CutPrefix(base, "http://"); ok {
		return "ws://" + host
	}
	return base
}

func initPaths() *Paths {
	dataDir := getDataDir()
	sourceVideoDir := filepath.Join(dataDir, "sourceVideo")

	return &Paths{
		Data:        dataDir,
		Video:       filepath.Join(dataDir, "video"),
		Stream:      filepath.Join(dataDir, "stream"),
		WSStream:    filepath.Join(dataDir, "wsstream"),
		SourceVideo: sourceVideoDir,
		Logs:        filepath.Join(dataDir, "logs"),
		LogsStats:   filepath.Join(dataDir, "logs", "stats"),
		LogsBots:    filepath.Join(dataDir, "logs", "bots"),
		LogsErrors:  filepath.Join(dataDir, "logs", "errors"),
		Tmp:         filepath.Join(dataDir, "tmp"),

		// Default files
		DefaultSourceVideo: filepath.Join(sourceVideoDir, "bunny.mp4"),
	}
}

func getDataDir() string {
	// Check if we're in a Docker container (common location)
	if _, err := os.Stat("/data"); err == nil {
		return "/data"
	}

	// Local development: relative to working directory
	wd, err := os.Getwd()
	if err != nil {
		panic("Failed to get working directory: " + err.Error())
	}

	return filepath.Join(wd, "data")
}

func EnsureDirectories() error {
	dirs := []string{
		AppPaths.Data,
		AppPaths.SourceVideo,
		AppPaths.Video,
		AppPaths.Stream,
		AppPaths.WSStream,
		AppPaths.Logs,
		AppPaths.LogsStats,
		AppPaths.LogsBots,
		AppPaths.LogsErrors,
		AppPaths.Tmp,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}

func GetSourceVideoFiles() ([]string, error) {
	entries, err := os.ReadDir(AppPaths.SourceVideo)
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

		if slices.Contains(ValidContainers, ext) {
			fullPath := filepath.Join(AppPaths.SourceVideo, entry.Name())
			videoFiles = append(videoFiles, fullPath)
		}
	}

	return videoFiles, nil
}
