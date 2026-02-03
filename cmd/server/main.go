package main

import (
	"fmt"
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

	service.StartupPregeneration()

	rest := rest.New()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", rest.ServeDocumentation)
	mux.HandleFunc("GET /sitemap.xml", rest.ServeSitemap)
	mux.HandleFunc("GET /robots.txt", rest.ServeRobots)
	mux.HandleFunc("GET /web/{path...}", rest.ServeStaticFiles)
	mux.HandleFunc("GET /getInfo/{name}", rest.GetVideoInfo)
	mux.HandleFunc("GET /transcode/{params}", rest.Transcode)
	mux.HandleFunc("GET /hls/{videoName}/{path...}", rest.ServeHLS)
	mux.HandleFunc("GET /{params}", rest.ServeVideo)

	statsMiddleware := stats.StatsMiddleware(config.AppPaths.LogsStats)
	handler := rest.RecoveryMiddleware(statsMiddleware(rest.BotsMiddleware(rest.CORSMiddleware(mux))))

	log.Printf("Server starting on port %d...", config.Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", config.Port), handler); err != nil {
		log.Fatal(err)
	}
}
