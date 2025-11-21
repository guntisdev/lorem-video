package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Paths holds all application directory paths
type Paths struct {
	Data        string
	Video       string
	SourceVideo string
	Logs        string
	Tmp         string

	// Default files
	DefaultSourceVideo string // bunny.mp4 path
}

// Global paths instance - initialized once and reused everywhere
var AppPaths = initPaths()

func initPaths() *Paths {
	dataDir := getDataDir()
	sourceVideoDir := filepath.Join(dataDir, "sourceVideo")

	return &Paths{
		Data:        dataDir,
		Video:       filepath.Join(dataDir, "video"),
		SourceVideo: sourceVideoDir,
		Logs:        filepath.Join(dataDir, "logs"),
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
		AppPaths.Logs,
		AppPaths.Tmp,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	if _, err := os.Stat(AppPaths.DefaultSourceVideo); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("default source video not found: %s", AppPaths.DefaultSourceVideo)
		}
		return fmt.Errorf("failed to access default source video: %w", err)
	}

	return nil
}
