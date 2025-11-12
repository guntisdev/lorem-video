package config

import (
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
}

// Global paths instance - initialized once and reused everywhere
var AppPaths = initPaths()

func initPaths() *Paths {
	dataDir := getDataDir()

	return &Paths{
		Data:        dataDir,
		Video:       filepath.Join(dataDir, "video"),
		SourceVideo: filepath.Join(dataDir, "sourceVideo"),
		Logs:        filepath.Join(dataDir, "logs"),
		Tmp:         filepath.Join(dataDir, "tmp"),
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

	return nil
}
