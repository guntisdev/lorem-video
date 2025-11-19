package main

import (
	"log"
	"net/http"
	"os"

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

	r := rest.New()

	http.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/dist/index.html")
	})

	http.HandleFunc("GET /web/{path...}", func(w http.ResponseWriter, r *http.Request) {
		fs := http.StripPrefix("/web/", http.FileServer(http.Dir("web/dist/")))
		fs.ServeHTTP(w, r)
	})

	http.HandleFunc("GET /getInfo/{name}", r.GetVideoInfo)
	http.HandleFunc("GET /transcode/{params}", r.Transcode)
	http.HandleFunc("GET /{params}", r.ServeVideo)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("Server starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
