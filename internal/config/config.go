package config

import (
	"fmt"
	"os"
	"path/filepath"
)

var DataDir = getDataDir()

func getDataDir() string {
	if _, err := os.Stat("/data"); err == nil {
		return "/data"
	}
	// Local: make it absolute relative to working directory
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.Join(wd, "data")
}

func EnsureDirectories() error {
	dirs := []string{
		DataDir,
		fmt.Sprintf("%s/sourceVideo", DataDir),
		fmt.Sprintf("%s/video", DataDir),
		fmt.Sprintf("%s/logs", DataDir),
		fmt.Sprintf("%s/tmp", DataDir),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}
