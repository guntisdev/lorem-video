package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lorem.video/internal/config"
	"lorem.video/internal/parser"
	"lorem.video/internal/service"
)

type Rest struct {
	videoService *service.VideoService
	appVersion   string // Cache-busting version generated at startup
}

func New() *Rest {
	return &Rest{
		videoService: service.NewVideoService(),
		appVersion:   fmt.Sprintf("%d", time.Now().Unix()),
	}
}

func (rest *Rest) ServeStaticFiles(w http.ResponseWriter, r *http.Request) {
	// Set cache headers - long cache since we use version parameters for cache busting
	if r.URL.Query().Get("v") != "" {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable") // 1 year
	} else {
		// Non-versioned resources get shorter cache
		w.Header().Set("Cache-Control", "public, max-age=3600") // 1 hour
	}

	fs := http.StripPrefix("/web/", http.FileServer(http.Dir("web/dist/")))
	fs.ServeHTTP(w, r)
}

func (rest *Rest) GetVideoInfo(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	info, err := rest.videoService.GetInfo(name)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(info); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (rest *Rest) Transcode(w http.ResponseWriter, r *http.Request) {
	params := r.PathValue("params")
	resultCh, errCh := rest.videoService.TranscodeFromParams(r.Context(), params)

	select {
	case result := <-resultCh:
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"output": result})
	case err := <-errCh:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	case <-r.Context().Done():
		http.Error(w, "request cancelled", http.StatusRequestTimeout)
	}
}

func (rest *Rest) ServeVideo(w http.ResponseWriter, r *http.Request) {
	params := r.PathValue("params")
	inputParams, err := parser.ParseFilename(params)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to parse filename parameters: %v", err), http.StatusBadRequest)
		return
	}

	spec := config.ApplyDefaultVideoSpec(inputParams)
	filename := parser.GenerateFilename(&spec)

	// Check for existing video
	existingPath := parser.FindExistingVideo(filename, &spec)
	if existingPath != "" {
		ext := strings.TrimPrefix(filepath.Ext(existingPath), ".")
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Type", "video/"+ext)
		http.ServeFile(w, r, existingPath)
		return
	}

	// Video not found, need to generate and stream it
	log.Printf("Video not found, generating and streaming: %s", filename)

	// TODO hardcoded .mp4 extension for source video. should be improved later
	inputPath := filepath.Join(config.AppPaths.SourceVideo, spec.Name+".mp4")
	if _, err := os.Stat(inputPath); err != nil {
		http.Error(w, fmt.Sprintf("failed to find source video: %s", spec.Name), http.StatusNotFound)
		return
	}

	// Start transcoding in background with independent context
	// This ensures transcoding continues even if HTTP request is canceled
	backgroundCtx := context.Background()
	fullOutputPath := filepath.Join(config.AppPaths.Tmp, filename)
	resultCh, errCh := rest.videoService.Transcode(backgroundCtx, spec, inputPath, config.AppPaths.Tmp)

	// Set headers for partial streaming (no caching to avoid incomplete content)
	ext := strings.TrimPrefix(filepath.Ext(fullOutputPath), ".")
	w.Header().Set("Content-Type", "video/"+ext)
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.WriteHeader(http.StatusOK)

	// Stream the partial file while it's being generated
	// Use request context for streaming, but transcoding uses background context
	rest.streamPartialFile(w, r.Context(), fullOutputPath, resultCh, errCh)
}

// streamPartialFile streams a file that's being written by FFmpeg
// Uses requestCtx to detect client disconnection, but allows transcoding to continue
func (rest *Rest) streamPartialFile(w http.ResponseWriter, requestCtx context.Context, filePath string, resultCh <-chan string, errCh <-chan error) {
	// Wait a moment for FFmpeg to start writing the file
	time.Sleep(100 * time.Millisecond)

	var file *os.File
	var lastPos int64 = 0

	// Try to open the file, wait if it doesn't exist yet
	for i := 0; i < 50; i++ { // Wait up to 5 seconds
		if f, err := os.Open(filePath); err == nil {
			file = f
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if file == nil {
		log.Printf("Failed to open partial file for streaming: %s", filePath)
		return
	}
	defer file.Close()

	buffer := make([]byte, 64*1024) // 64KB buffer

	for {
		// Check if request was canceled (client disconnected)
		select {
		case <-requestCtx.Done():
			log.Printf("Client disconnected, but transcoding continues in background")
			file.Close()
			return
		default:
		}

		// Check if transcoding is complete or failed
		select {
		case <-resultCh:
			// Transcoding completed, stream the rest of the file if client still connected
			select {
			case <-requestCtx.Done():
				log.Printf("Client disconnected after transcoding completed")
				file.Close()
				return
			default:
				rest.streamRemainingFile(w, file, &lastPos, buffer)
				return
			}
		case err := <-errCh:
			log.Printf("Transcoding failed during streaming: %v", err)
			return
		default:
			// Continue streaming partial content
		}

		// Read new content from the current position
		stat, err := file.Stat()
		if err != nil {
			log.Printf("Failed to stat file during streaming: %v", err)
			return
		}

		if stat.Size() > lastPos {
			// New content available
			file.Seek(lastPos, 0)
			n, err := file.Read(buffer)
			if err != nil && err != io.EOF {
				log.Printf("Failed to read file during streaming: %v", err)
				return
			}
			if n > 0 {
				// Check if client is still connected before writing
				select {
				case <-requestCtx.Done():
					log.Printf("Client disconnected during write")
					file.Close()
					return
				default:
				}

				if _, err := w.Write(buffer[:n]); err != nil {
					log.Printf("Failed to write to response during streaming (client likely disconnected): %v", err)
					return
				}
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
				lastPos += int64(n)
			}
		}

		// Small delay to avoid busy waiting
		time.Sleep(50 * time.Millisecond)
	}
}

// streamRemainingFile streams any remaining content after transcoding completes
func (rest *Rest) streamRemainingFile(w http.ResponseWriter, file *os.File, lastPos *int64, buffer []byte) {
	file.Seek(*lastPos, 0)
	for {
		n, err := file.Read(buffer)
		if n > 0 {
			if _, writeErr := w.Write(buffer[:n]); writeErr != nil {
				log.Printf("Failed to write remaining content: %v", writeErr)
				return
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Error reading remaining content: %v", err)
			return
		}
	}
}
