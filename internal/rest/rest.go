package rest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"lorem.video/internal/service"
)

type Rest struct {
	videoService *service.VideoService
}

func New() *Rest {
	return &Rest{
		videoService: service.NewVideoService(),
	}
}

func (rest *Rest) CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
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
