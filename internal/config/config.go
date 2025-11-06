package config

import (
	"fmt"
	"os"
)

var DataDir = getDataDir()

func getDataDir() string {
	if _, err := os.Stat("/data"); err == nil {
		return "/data"
	}
	return "data"
}

func EnsureDirectories() error {
	dirs := []string{
		DataDir,
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
