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
	videoService := service.NewVideoService()
	videoService.StartupPregeneration()

	rest := rest.New()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/dist/index.html")
	})

	mux.HandleFunc("GET /web/{path...}", func(w http.ResponseWriter, r *http.Request) {
		fs := http.StripPrefix("/web/", http.FileServer(http.Dir("web/dist/")))
		fs.ServeHTTP(w, r)
	})

	mux.HandleFunc("GET /getInfo/{name}", rest.GetVideoInfo)
	mux.HandleFunc("GET /transcode/{params}", rest.Transcode)
	mux.HandleFunc("GET /{params}", rest.ServeVideo)

	handler := rest.CORSMiddleware(mux)

	port := "3000"

	log.Printf("Server starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}
