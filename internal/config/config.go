package config

import "os"

var DataDir = getDataDir()

func getDataDir() string {
	if _, err := os.Stat("/data"); err == nil {
		return "/data"
	}
	return "data"
}
