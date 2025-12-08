package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

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

	w.Header().Set("Accept-Ranges", "bytes")

	resultCh, errCh := rest.videoService.GetOrGenerate(r.Context(), params)

	select {
	case videoPath := <-resultCh:
		ext := strings.TrimPrefix(filepath.Ext(videoPath), ".")
		w.Header().Set("Content-Type", "video/"+ext)

		http.ServeFile(w, r, videoPath)

	case err := <-errCh:
		http.Error(w, err.Error(), http.StatusInternalServerError)

	case <-r.Context().Done():
		http.Error(w, "request cancelled", http.StatusRequestTimeout)
	}
}
