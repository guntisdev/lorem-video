package rest

import (
	"net/http"
	"slices"
	"strings"

	"lorem.video/internal/config"
)

const wsManifestSuffix = "/manifest-ws2.json"

// CORSMiddleware adds CORS headers to all requests
func (rest *Rest) CORSMiddleware(next http.Handler) http.Handler {
	wsOrigins := config.GetWSCORSOrigins()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/ws/") && strings.HasSuffix(r.URL.Path, wsManifestSuffix) {
			// allowlisted origins get credentialed CORS, everyone else falls back to "*"
			w.Header().Add("Vary", "Origin")
			if origin := r.Header.Get("Origin"); origin != "" && slices.Contains(wsOrigins, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
