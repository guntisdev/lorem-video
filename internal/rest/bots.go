package rest

import (
	"net/http"
	"strings"

	"lorem.video/internal/config"
	"lorem.video/internal/stats"
)

func (rest *Rest) BotsMiddleware(next http.Handler) http.Handler {
	botStatsMiddleware := stats.StatsMiddleware(config.AppPaths.LogsBots)

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
				// Log the bot request with 404 status using the bot stats middleware
				botHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.NotFound(w, r)
				})
				botStatsMiddleware(botHandler).ServeHTTP(w, r)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
