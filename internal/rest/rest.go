package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lorem.video/internal/config"
	"lorem.video/internal/parser"
	"lorem.video/internal/service"
)

type Rest struct {
	videoService *service.VideoService
	appVersion   string // Cache-busting version generated at startup
}

func New() *Rest {
	return &Rest{
		videoService: service.NewVideoService(),
		appVersion:   fmt.Sprintf("%d", time.Now().Unix()),
	}
}

func (rest *Rest) ServeStaticFiles(w http.ResponseWriter, r *http.Request) {
	// Set cache headers - long cache since we use version parameters for cache busting
	if r.URL.Query().Get("v") != "" {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable") // 1 year
	} else {
		// Non-versioned resources get shorter cache
		w.Header().Set("Cache-Control", "public, max-age=3600") // 1 hour
	}

	fs := http.StripPrefix("/web/", http.FileServer(http.Dir("web/dist/")))
	fs.ServeHTTP(w, r)
}

func (rest *Rest) ServeSitemap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600") // 1 hour cache
	http.ServeFile(w, r, "web/dist/sitemap.xml")
}

func (rest *Rest) ServeRobots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600") // 1 hour cache
	http.ServeFile(w, r, "web/dist/robots.txt")
}

func (rest *Rest) GetVideoInfo(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	info, err := rest.videoService.GetInfo(name)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(info); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (rest *Rest) Transcode(w http.ResponseWriter, r *http.Request) {
	params := r.PathValue("params")
	resultCh, errCh := rest.videoService.TranscodeFromParams(r.Context(), params)

	select {
	case result := <-resultCh:
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"output": result})
	case err := <-errCh:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	case <-r.Context().Done():
		http.Error(w, "request cancelled", http.StatusRequestTimeout)
	}
}

func (rest *Rest) ServeVideo(w http.ResponseWriter, r *http.Request) {
	params := r.PathValue("params")
	inputParams, err := parser.ParseFilename(params)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to parse filename parameters: %v", err), http.StatusBadRequest)
		return
	}

	if *inputParams == (config.VideoSpec{}) {
		http.Error(w, "no valid parameters found", http.StatusNotFound)
		return
	}

	spec := config.ApplyDefaultVideoSpec(inputParams)
	filename := parser.GenerateFilename(&spec)

	// Check for existing video
	existingPath := parser.FindExistingVideo(filename, &spec)
	if existingPath != "" {
		ext := strings.TrimPrefix(filepath.Ext(existingPath), ".")
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Type", "video/"+ext)

		// Check if video is less than 5 minutes old to avoid caching partial transcodes
		if stat, err := os.Stat(existingPath); err == nil {
			fileAge := time.Since(stat.ModTime())
			if fileAge < 5*time.Minute {
				// Video is recent, might be partial - don't cache
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
				w.Header().Set("Pragma", "no-cache")
				w.Header().Set("Expires", "0")
			} else {
				// Video is older, safe to cache
				w.Header().Set("Cache-Control", "public, max-age=3600") // 1 hour cache
			}
		}

		http.ServeFile(w, r, existingPath)
		return
	}

	// Video not found, start transcoding and tell client to retry
	log.Printf("Starting transcoding for: %s", filename)

	// TODO hardcoded .mp4 extension for source video. should be improved later
	inputPath := filepath.Join(config.AppPaths.SourceVideo, spec.Name+".mp4")
	if _, err := os.Stat(inputPath); err != nil {
		http.Error(w, fmt.Sprintf("failed to find source video: %s", spec.Name), http.StatusNotFound)
		return
	}

	// Start transcoding in background
	backgroundCtx := context.Background()
	_, _ = rest.videoService.Transcode(backgroundCtx, spec, inputPath, config.AppPaths.Tmp)

	// Return 202 Accepted with retry instructions
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Retry-After", "5")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)

	json.NewEncoder(w).Encode(map[string]string{
		"status":      "transcoding",
		"message":     "Video is being generated. Please retry this URL in a few moments.",
		"retry_after": "5",
	})
}
