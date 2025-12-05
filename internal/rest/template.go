package rest

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"lorem.video/internal/config"
)

type TemplateData struct {
	Domain       string
	Version      string
	VideoCodecs  []string
	AudioCodecs  []string
	Containers   []string
	Resolutions  []string
	SourceVideos []string
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
		VideoCodecs:  config.ValidVideoCodecs,
		AudioCodecs:  config.ValidAudioCodecs,
		Containers:   config.ValidContainers,
		Resolutions:  resolutionNames,
		SourceVideos: sourceVideoNames,
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
