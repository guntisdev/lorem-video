package stats

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
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
	logFile     *os.File
	writer      *bufio.Writer
	currentDate string // Track current date for file rotation
	logPath     string // Directory path for log files
	mutex       sync.Mutex
}

func NewStatsLogger(logPath string) (*StatsLogger, error) {
	return &StatsLogger{
		logFile:     nil,
		writer:      nil,
		currentDate: "", // Empty means no file opened yet
		logPath:     logPath,
	}, nil
}

func (sl *StatsLogger) Log(stats RequestStats) error {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	// Check if we need to open/rotate to the correct daily log file
	currentDate := stats.Timestamp.Format("2006-01-02")
	if currentDate != sl.currentDate {
		// Close current file if open
		if sl.writer != nil {
			sl.writer.Flush()
		}
		if sl.logFile != nil {
			sl.logFile.Close()
		}

		// Open file for today
		logPath := filepath.Join(sl.logPath, fmt.Sprintf("stats-%s.jsonl", currentDate))
		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}

		sl.logFile = file
		sl.writer = bufio.NewWriter(file)
		sl.currentDate = currentDate
	}

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

func StatsMiddleware(logPath string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		logger, err := NewStatsLogger(logPath)
		if err != nil {
			fmt.Printf("Warning: Failed to create stats logger: %v\n", err)
		}

		gcClient := &http.Client{Timeout: 3 * time.Second}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     200,
				bytesWritten:   0,
			}

			next.ServeHTTP(rw, r)

			responseTime := time.Since(start).Milliseconds()

			ipAddress := getRealIP(r)

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

			if !shouldSkipPath(r.URL.Path) {
				sendToGoatCounter(gcClient, r, ipAddress)
			}
		})
	}
}

type GoatCounterHit struct {
	Path     string `json:"path"`
	Title    string `json:"title,omitempty"`
	Referrer string `json:"ref,omitempty"`
	Event    bool   `json:"event,omitempty"`
	Query    string `json:"query,omitempty"`
	Size     []int  `json:"size,omitempty"` // [width, height, scale]
}

type GoatCounterRequest struct {
	Hits       []GoatCounterHit `json:"hits"`
	NoSessions bool             `json:"no_sessions,omitempty"`
}

func sendToGoatCounter(client *http.Client, r *http.Request, ip string) {
	goatcounterURL := os.Getenv("GOATCOUNTER_URL")
	if goatcounterURL == "" {
		goatcounterURL = "http://goatcounter:8082"
	}

	token := os.Getenv("GOATCOUNTER_TOKEN")
	if token == "" {
		// Skip sending if no token is configured
		return
	}

	hit := GoatCounterHit{
		Path:     r.URL.Path,
		Referrer: r.Referer(),
	}

	request := GoatCounterRequest{
		Hits:       []GoatCounterHit{hit},
		NoSessions: true, // Allow tracking without sessions since we're on the backend
	}

	data, err := json.Marshal(request)
	if err != nil {
		return
	}

	req, err := http.NewRequest("POST",
		goatcounterURL+"/api/v0/count",
		bytes.NewBuffer(data))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", r.UserAgent())
	req.Header.Set("X-Forwarded-For", ip)

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// Log errors for debugging
	if resp.StatusCode >= 400 {
		fmt.Printf("GoatCounter API error: %d %s\n", resp.StatusCode, resp.Status)
	}
}

func getRealIP(r *http.Request) string {
	ipAddress := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ipAddress = strings.Split(forwarded, ",")[0]
	} else if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		ipAddress = realIP
	}
	if host, _, err := net.SplitHostPort(ipAddress); err == nil {
		ipAddress = host
	}
	return strings.TrimSpace(ipAddress)
}

func shouldSkipPath(path string) bool {
	skipPrefixes := []string{
		"/web/", "/favicon.ico",
		"/health", "/ping",
	}
	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
