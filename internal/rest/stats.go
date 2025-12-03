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

type RequestStats struct {
	Timestamp    time.Time `json:"timestamp"`
	Method       string    `json:"method"`
	Path         string    `json:"path"`
	IP           string    `json:"ip"`
	Referer      string    `json:"referer,omitempty"`
	UserAgent    string    `json:"ua"`
	Status       int       `json:"status"`
	ResponseTime int64     `json:"responseTime"` // ms
	ResponseSize int64     `json:"responseSize"` // bytes
	ContentType  string    `json:"content_type,omitempty"`
}

type StatsLogger struct {
	logFile *os.File
	writer  *bufio.Writer
	mutex   sync.Mutex
}

func NewStatsLogger() (*StatsLogger, error) {
	logDir := config.AppPaths.Logs
	logPath := filepath.Join(logDir, fmt.Sprintf("stats-%s.jsonl", time.Now().Format("2006-01-02")))

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &StatsLogger{
		logFile: file,
		writer:  bufio.NewWriter(file),
	}, nil
}

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

	return sl.writer.Flush()
}

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

func (rest *Rest) StatsMiddleware(next http.Handler) http.Handler {
	logger, err := NewStatsLogger()
	if err != nil {
		fmt.Printf("Warning: Failed to create stats logger: %v\n", err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     200, // Default status code
			bytesWritten:   0,
		}

		next.ServeHTTP(rw, r)

		responseTime := time.Since(start).Milliseconds()

		ipAddress := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ipAddress = strings.Split(forwarded, ",")[0]
		} else if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
			ipAddress = realIP
		}
		if host, _, err := net.SplitHostPort(ipAddress); err == nil {
			ipAddress = host
		}

		stats := RequestStats{
			Timestamp:    start,
			Method:       r.Method,
			Path:         r.URL.Path,
			IP:           ipAddress,
			UserAgent:    r.Header.Get("User-Agent"),
			Referer:      r.Header.Get("Referer"),
			Status:       rw.statusCode,
			ResponseTime: responseTime,
			ResponseSize: rw.bytesWritten,
			ContentType:  rw.Header().Get("Content-Type"),
		}

		if logger != nil {
			if err := logger.Log(stats); err != nil {
				fmt.Printf("Warning: Failed to log request stats: %v\n", err)
			}
		}
	})
}
