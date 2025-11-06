package main

import (
	"log"
	"net/http"
	"os"

	"kittens/internal/config"
	"kittens/internal/rest"
)

func main() {
	if err := config.EnsureDirectories(); err != nil {
		log.Fatalf("Failed to create directories %v", err)
	}

	r := rest.New()
	http.HandleFunc("GET /", r.Index)
	http.HandleFunc("GET /video/get/{resolution}", r.GetVideo)
	http.HandleFunc("GET /video/resize/{resolution}", r.ResizeVideo)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("Server starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
