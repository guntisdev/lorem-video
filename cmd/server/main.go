package main

import (
	"log"
	"net/http"
	"os"

	"kittens/internal/config"
	"kittens/internal/rest"
	"kittens/internal/service"
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

	http.HandleFunc("GET /video/serve/{resolution}", r.ServeVideo)
	http.HandleFunc("GET /video/getInfo/{name}", r.GetVideoInfo)
	http.HandleFunc("GET /video/transcode/{params}", r.Transcode)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("Server starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
