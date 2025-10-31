package types

type Resolution struct {
	Width  int
	Height int
}

var Resolutions = map[string]Resolution{
	"360p":  {640, 360},
	"480p":  {854, 480},
	"720p":  {1280, 720},
	"1080p": {1920, 1080},
	"1440p": {2560, 1440},
	"4k":    {3840, 2160},
}
