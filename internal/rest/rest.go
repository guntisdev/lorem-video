package rest

import (
	"fmt"
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

func (rest *Rest) Video(w http.ResponseWriter, r *http.Request) {
	resolution := r.PathValue("resolution")
	fmt.Println("Resolution: ", resolution)
	rest.videoService.ServeVideo(w, r, resolution)
}
