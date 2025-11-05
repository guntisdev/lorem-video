package rest

import (
	"encoding/json"
	"fmt"
	"kittens/internal/config"
	"kittens/internal/service"
	"net/http"
)

type Rest struct {
	videoService *service.VideoService
}

func New() *Rest {
	return &Rest{
		videoService: service.NewVideoService(),
	}
}

func (rest *Rest) Index(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/dist/index.html")
}

func (rest *Rest) GetVideo(w http.ResponseWriter, r *http.Request) {
	resolution := r.PathValue("resolution")

	videoPath, err := rest.videoService.GetVideoPath(resolution)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Accept-Ranges", "bytes")

	http.ServeFile(w, r, videoPath)
}

func (rest *Rest) ResizeVideo(w http.ResponseWriter, r *http.Request) {
	resolution := r.PathValue("resolution")

	res, ok := config.Resolutions[resolution]
	if !ok {
		http.Error(w, "invalid resolution", http.StatusBadRequest)
		return
	}

	inputPath := fmt.Sprintf("%s/720p.mp4", config.DataDir)
	outputPath := fmt.Sprintf("%s/%s.mp4", config.DataDir, resolution)

	resultCh, errCh := rest.videoService.ResizeVideo(r.Context(), inputPath, outputPath, res.Width, res.Height)

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
