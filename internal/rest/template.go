package rest

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"lorem.video/internal/config"
)

type TemplateData struct {
	Domain       string
	Version      string
	CurrentYear  int
	VideoCodecs  []string
	AudioCodecs  []string
	Containers   []string
	Resolutions  []string
	SourceVideos []string
	// Defaults from DefaultVideoSpec
	DefaultResolution   string
	DefaultCodec        string
	DefaultFPS          int
	DefaultDuration     int
	DefaultBitrate      string
	DefaultAudioCodec   string
	DefaultAudioBitrate int
	DefaultContainer    string
}

// ServeDocumentation serves the documentation page with dynamic data from config
func (rest *Rest) ServeDocumentation(w http.ResponseWriter, r *http.Request) {
	resolutionNames := make([]string, 0, len(config.Resolutions)+1)
	for name := range config.Resolutions {
		resolutionNames = append(resolutionNames, name)
	}
	resolutionNames = append(resolutionNames, "WxH custom")

	sourceVideoFiles, err := config.GetSourceVideoFiles()
	var sourceVideoNames []string
	if err != nil {
		log.Printf("Warning: Could not get source videos: %v", err)
		sourceVideoNames = []string{"bunny"} // fallback
	} else {
		sourceVideoNames = make([]string, 0, len(sourceVideoFiles))
		for _, file := range sourceVideoFiles {
			// Extract just the filename without extension
			base := filepath.Base(file)
			name := strings.TrimSuffix(base, filepath.Ext(base))
			sourceVideoNames = append(sourceVideoNames, name)
		}
	}

	data := TemplateData{
		Domain:       "lorem.video",
		Version:      rest.appVersion, // for caching
		CurrentYear:  time.Now().Year(),
		VideoCodecs:  config.ValidVideoCodecs,
		AudioCodecs:  config.ValidAudioCodecs,
		Containers:   config.ValidContainers,
		Resolutions:  resolutionNames,
		SourceVideos: sourceVideoNames,

		DefaultResolution:   fmt.Sprintf("%dx%d", config.DefaultVideoSpec.Width, config.DefaultVideoSpec.Height),
		DefaultCodec:        config.DefaultVideoSpec.Codec,
		DefaultFPS:          config.DefaultVideoSpec.FPS,
		DefaultDuration:     config.DefaultVideoSpec.Duration,
		DefaultBitrate:      config.DefaultVideoSpec.Bitrate,
		DefaultAudioCodec:   config.DefaultVideoSpec.AudioCodec,
		DefaultAudioBitrate: config.DefaultVideoSpec.AudioBitrate,
		DefaultContainer:    config.DefaultVideoSpec.Container,
	}

	tmpl, err := template.ParseFiles("web/dist/index.html")
	if err != nil {
		log.Printf("Error parsing template: %v", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", "no-cache, must-revalidate")
	w.Header().Set("Content-Type", "text/html")
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Template execution error", http.StatusInternalServerError)
	}
}
