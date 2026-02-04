package rest

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"lorem.video/internal/config"
	"lorem.video/internal/stats"
)

func (rest *Rest) BotsMiddleware(next http.Handler) http.Handler {
	botLogger, err := stats.NewStatsLogger(config.AppPaths.LogsBots)
	if err != nil {
		log.Printf("Warning: Failed to create bot logger: %v", err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := strings.ToLower(r.URL.Path)

		botPatterns := []string{
			".php",
			"/wp-",
			"/_next",
			"/_react",
			"/_ignition", // laravel debug/error handler
			"/laravel",
			"/.git",
			"/.env",
			"/.htaccess",
			"/passwd",
			"/.aws",
			"/phpmyadmin",
			"/umi.js",
			"/compoments.js", // yes misspelled compoments from real stats
			"/admin",
			"/login",
			"/wiki",
			"/cgi-bin",
		}

		for _, pattern := range botPatterns {
			if strings.Contains(url, pattern) {
				if botLogger != nil {
					start := time.Now()
					stats := stats.RequestStats{
						Timestamp:    start,
						Method:       r.Method,
						Path:         r.URL.Path,
						IP:           r.RemoteAddr,
						Referer:      r.Header.Get("Referer"),
						UserAgent:    r.Header.Get("User-Agent"),
						Status:       http.StatusNotFound,
						ResponseTime: 0, // Immediate response
						ResponseSize: 0,
						ContentType:  "text/plain",
					}

					if err := botLogger.Log(stats); err != nil {
						fmt.Printf("Warning: Failed to log bot request: %v\n", err)
					}
				}

				// Return 404 without going to next handler
				http.NotFound(w, r)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
