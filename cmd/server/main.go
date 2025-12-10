package main

import (
	"log"
	"net/http"

	"lorem.video/internal/config"
	"lorem.video/internal/rest"
	"lorem.video/internal/service"
	"lorem.video/internal/stats"
)

func main() {
	if err := config.EnsureDirectories(); err != nil {
		log.Fatalf("Failed to create directories: %v", err)
	}

	if err := service.EnsureDefaultSourceVideo(); err != nil {
		log.Fatalf("Failed to create default source video: %v", err)
	}

	videoService := service.NewVideoService()
	videoService.StartupPregeneration()

	rest := rest.New()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", rest.ServeDocumentation)
	mux.HandleFunc("GET /web/{path...}", rest.ServeStaticFiles)
	mux.HandleFunc("GET /getInfo/{name}", rest.GetVideoInfo)
	mux.HandleFunc("GET /transcode/{params}", rest.Transcode)
	mux.HandleFunc("GET /{params}", rest.ServeVideo)

	handler := rest.RecoveryMiddleware(stats.StatsMiddleware(rest.CORSMiddleware(mux)))

	port := "3000"

	log.Printf("Server starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}
