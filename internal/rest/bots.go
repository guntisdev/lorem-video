package rest

import (
	"net/http"
	"strings"
)

func (rest *Rest) BotsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := strings.ToLower(r.URL.Path)

		botPatterns := []string{
			".php",
			"/wp-",
			"/_next/",
			"/.git",
			"/.env",
			"/.htaccess",
			"/.aws",
			"/phpmyadmin",
		}

		for _, pattern := range botPatterns {
			if strings.Contains(url, pattern) {
				http.NotFound(w, r)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
