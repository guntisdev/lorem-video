package rest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lorem.video/internal/config"
)

// RequestStats represents the structure of logged request data
type RequestStats struct {
	Timestamp    time.Time `json:"timestamp"`
	Method       string    `json:"method"`
	Path         string    `json:"path"`
	QueryParams  string    `json:"query_params,omitempty"`
	IPAddress    string    `json:"ip_address"`
	UserAgent    string    `json:"user_agent"`
	Referer      string    `json:"referer,omitempty"`
	StatusCode   int       `json:"status_code"`
	ResponseTime int64     `json:"response_time_ms"`
	ResponseSize int64     `json:"response_size_bytes"`
	ContentType  string    `json:"content_type,omitempty"`
}

// StatsLogger handles writing request stats to JSON Lines files
type StatsLogger struct {
	logFile *os.File
	writer  *bufio.Writer
	mutex   sync.Mutex
}

// NewStatsLogger creates a new stats logger
func NewStatsLogger() (*StatsLogger, error) {
	logDir := config.AppPaths.Logs
	logPath := filepath.Join(logDir, fmt.Sprintf("access-%s.jsonl", time.Now().Format("2006-01-02")))

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &StatsLogger{
		logFile: file,
		writer:  bufio.NewWriter(file),
	}, nil
}

// Log writes a request stat entry to the log file
func (sl *StatsLogger) Log(stats RequestStats) error {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	data, err := json.Marshal(stats)
	if err != nil {
		return fmt.Errorf("failed to marshal stats: %w", err)
	}

	_, err = sl.writer.Write(append(data, '\n'))
	if err != nil {
		return fmt.Errorf("failed to write to log: %w", err)
	}

	// Flush periodically for minimal performance overhead
	return sl.writer.Flush()
}

// Close closes the log file
func (sl *StatsLogger) Close() error {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	if sl.writer != nil {
		sl.writer.Flush()
	}
	if sl.logFile != nil {
		return sl.logFile.Close()
	}
	return nil
}

// responseWriter wraps http.ResponseWriter to capture response stats
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += int64(n)
	return n, err
}

// StatsMiddleware logs request statistics
func (rest *Rest) StatsMiddleware(next http.Handler) http.Handler {
	// Create a logger instance for this middleware
	logger, err := NewStatsLogger()
	if err != nil {
		// If logger creation fails, continue without logging but print warning
		fmt.Printf("Warning: Failed to create stats logger: %v\n", err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the response writer to capture stats
		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     200, // Default status code
			bytesWritten:   0,
		}

		// Process the request
		next.ServeHTTP(rw, r)

		// Calculate response time
		responseTime := time.Since(start).Milliseconds()

		// Extract IP address (handle proxies)
		ipAddress := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ipAddress = strings.Split(forwarded, ",")[0]
		} else if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
			ipAddress = realIP
		}
		// Remove port from IP address
		if host, _, err := net.SplitHostPort(ipAddress); err == nil {
			ipAddress = host
		}

		// Create stats entry
		stats := RequestStats{
			Timestamp:    start,
			Method:       r.Method,
			Path:         r.URL.Path,
			QueryParams:  r.URL.RawQuery,
			IPAddress:    ipAddress,
			UserAgent:    r.Header.Get("User-Agent"),
			Referer:      r.Header.Get("Referer"),
			StatusCode:   rw.statusCode,
			ResponseTime: responseTime,
			ResponseSize: rw.bytesWritten,
			ContentType:  rw.Header().Get("Content-Type"),
		}

		// Log the stats (only if logger is available)
		if logger != nil {
			if err := logger.Log(stats); err != nil {
				fmt.Printf("Warning: Failed to log request stats: %v\n", err)
			}
		}
	})
}
