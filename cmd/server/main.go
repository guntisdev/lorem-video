package main

import (
	"log"
	"net/http"

	"lorem.video/internal/config"
	"lorem.video/internal/rest"
	"lorem.video/internal/service"
)

func main() {
	if err := config.EnsureDirectories(); err != nil {
		log.Fatalf("Failed to create directories %v", err)
	}

	// Start video pregeneration in background
	// TODO for initial video create video from ffmpeg command (there is some debug video with bars and audio)
	videoService := service.NewVideoService()
	videoService.StartupPregeneration()

	rest := rest.New()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", rest.ServeDocumentation)
	mux.HandleFunc("GET /web/{path...}", rest.ServeStaticFiles)
	mux.HandleFunc("GET /getInfo/{name}", rest.GetVideoInfo)
	mux.HandleFunc("GET /transcode/{params}", rest.Transcode)
	mux.HandleFunc("GET /{params}", rest.ServeVideo)

	handler := rest.StatsMiddleware(rest.CORSMiddleware(mux))

	port := "3000"

	log.Printf("Server starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}
