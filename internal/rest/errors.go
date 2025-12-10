package rest

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"lorem.video/internal/config"
)

// logError writes error message to both stdout and error log file
func logError(msg string) {
	// Log to stdout as usual
	log.Print(msg)

	// Also write to error log file
	date := time.Now().Format("2006-01-02")
	errorLogPath := filepath.Join(config.AppPaths.Errors, fmt.Sprintf("error-%s.log", date))

	errorFile, err := os.OpenFile(errorLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		defer errorFile.Close()
		errorFile.WriteString(time.Now().Format("2006/01/02 15:04:05") + " " + msg + "\n")
	}
}

// RecoveryMiddleware catches panics and logs them with detailed information
func (rest *Rest) RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				errorMsg := fmt.Sprintf("PANIC RECOVERED: %v\nRequest: %s %s\nRemote: %s\nUser-Agent: %s\nStack Trace:\n%s",
					err, r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent(), debug.Stack())
				logError(errorMsg)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
