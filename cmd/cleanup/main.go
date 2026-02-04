package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"lorem.video/internal/config"
	"lorem.video/internal/parser"
)

type InvalidVideo struct {
	Path     string
	Reason   string
	FileSize int64
	ModTime  time.Time
}

type CleanupService struct {
	dryRun bool
}

func main() {
	var (
		dryRun  = flag.Bool("dry-run", true, "List invalid videos without deleting them")
		delete  = flag.Bool("delete", false, "Delete invalid videos (overrides dry-run)")
		verbose = flag.Bool("v", false, "Verbose output with detailed analysis")
		maxAge  = flag.Duration("max-age", 365*24*time.Hour, "Maximum age for temporary files before considering them abandoned")
		minSize = flag.Int64("min-size", 1024, "Minimum file size in bytes (smaller files are considered invalid)")
	)
	flag.Parse()

	// If --delete is specified, turn off dry-run
	if *delete {
		*dryRun = false
	}

	service := &CleanupService{dryRun: *dryRun}

	fmt.Printf("Lorem Video Cleanup Tool\n")
	fmt.Printf("Scanning: %s\n", config.AppPaths.Tmp)
	fmt.Printf("Mode: %s\n", map[bool]string{true: "DRY RUN", false: "DELETE"}[*dryRun])
	fmt.Printf("Max age: %v\n", *maxAge)
	fmt.Printf("Min size: %d bytes\n", *minSize)
	fmt.Println()

	invalidVideos, err := service.scanInvalidVideos(*maxAge, *minSize, *verbose)
	if err != nil {
		log.Fatalf("Error scanning videos: %v", err)
	}

	if len(invalidVideos) == 0 {
		fmt.Println("No invalid videos found!")
		return
	}

	fmt.Printf("Found %d invalid video(s):\n\n", len(invalidVideos))

	var totalSize int64
	for _, video := range invalidVideos {
		totalSize += video.FileSize
		fmt.Printf("%s\n", filepath.Base(video.Path))
		fmt.Printf("   Reason: %s\n", video.Reason)
		fmt.Printf("   Size: %s\n", formatBytes(video.FileSize))
		fmt.Printf("   Modified: %s (%s ago)\n",
			video.ModTime.Format("2006-01-02 15:04:05"),
			time.Since(video.ModTime).Round(time.Minute))
		if *verbose {
			fmt.Printf("   Full path: %s\n", video.Path)
		}
		fmt.Println()
	}

	fmt.Printf("Total size: %s\n\n", formatBytes(totalSize))

	if !*dryRun {
		fmt.Printf("Deleting %d invalid video(s)...\n", len(invalidVideos))
		deleted, failed := service.deleteInvalidVideos(invalidVideos)
		fmt.Printf("Deleted: %d files\n", deleted)
		if failed > 0 {
			fmt.Printf("Failed to delete: %d files\n", failed)
		}
	} else {
		fmt.Printf("Run with --delete to remove these files\n")
	}
}

func (s *CleanupService) scanInvalidVideos(maxAge time.Duration, minSize int64, verbose bool) ([]InvalidVideo, error) {
	var invalidVideos []InvalidVideo

	err := filepath.Walk(config.AppPaths.Tmp, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
		if !slices.Contains(config.ValidContainers, ext) {
			return nil
		}

		if verbose {
			fmt.Printf("Analyzing: %s\n", filepath.Base(path))
		}

		reasons := s.analyzeVideo(path, info, maxAge, minSize, verbose)

		if len(reasons) > 0 {
			invalidVideos = append(invalidVideos, InvalidVideo{
				Path:     path,
				Reason:   strings.Join(reasons, "; "),
				FileSize: info.Size(),
				ModTime:  info.ModTime(),
			})
		}

		return nil
	})

	return invalidVideos, err
}

func (s *CleanupService) analyzeVideo(path string, info os.FileInfo, maxAge time.Duration, minSize int64, verbose bool) []string {
	var reasons []string

	if info.Size() < minSize {
		reasons = append(reasons, fmt.Sprintf("file too small (%s)", formatBytes(info.Size())))
	}

	if time.Since(info.ModTime()) > maxAge {
		reasons = append(reasons, fmt.Sprintf("abandoned file (age: %v)", time.Since(info.ModTime()).Round(time.Minute)))
	}

	filename := filepath.Base(path)
	filenameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))

	spec, err := parser.ParseFilename(filenameWithoutExt)
	if err != nil {
		reasons = append(reasons, "unparseable filename")
		return reasons
	}

	if verbose {
		fmt.Printf("Expected duration: %ds\n", spec.Duration)
	}

	probeResult := s.probeVideo(path, verbose)
	if probeResult.Error != nil {
		reasons = append(reasons, fmt.Sprintf("ffprobe failed: %v", probeResult.Error))
		return reasons
	}

	// Check if duration matches expected (allow 10% tolerance)
	if spec.Duration > 0 && probeResult.Duration > 0 {
		expectedDuration := float64(spec.Duration)
		actualDuration := probeResult.Duration
		tolerance := expectedDuration * 0.1 // 10% tolerance

		if actualDuration < expectedDuration-tolerance {
			reasons = append(reasons, fmt.Sprintf("duration too short (expected: %.1fs, actual: %.1fs)",
				expectedDuration, actualDuration))
		}
	}

	if !probeResult.HasVideoStream && spec.Codec != "novideo" {
		reasons = append(reasons, "missing video stream")
	}

	if !probeResult.HasAudioStream && spec.AudioCodec != "noaudio" {
		reasons = append(reasons, "missing audio stream")
	}

	if probeResult.Width > 0 && probeResult.Height > 0 && spec.Width > 0 && spec.Height > 0 {
		if probeResult.Width != spec.Width || probeResult.Height != spec.Height {
			reasons = append(reasons, fmt.Sprintf("resolution mismatch (expected: %dx%d, actual: %dx%d)",
				spec.Width, spec.Height, probeResult.Width, probeResult.Height))
		}
	}

	return reasons
}

type ProbeResult struct {
	Duration       float64
	Width          int
	Height         int
	HasVideoStream bool
	HasAudioStream bool
	Error          error
}

func (s *CleanupService) probeVideo(path string, verbose bool) ProbeResult {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)

	output, err := cmd.Output()
	if err != nil {
		return ProbeResult{Error: err}
	}

	var probe config.FFProbeOutput
	if err := json.Unmarshal(output, &probe); err != nil {
		return ProbeResult{Error: fmt.Errorf("failed to parse ffprobe output: %w", err)}
	}

	result := ProbeResult{}

	if probe.Format.Duration != "" {
		if duration, err := parseFloat(probe.Format.Duration); err == nil {
			result.Duration = duration
		}
	}

	for _, stream := range probe.Streams {
		switch stream.CodecType {
		case "video":
			result.HasVideoStream = true
			if stream.Width > 0 {
				result.Width = stream.Width
			}
			if stream.Height > 0 {
				result.Height = stream.Height
			}
		case "audio":
			result.HasAudioStream = true
		}
	}

	if verbose && result.Error == nil {
		fmt.Printf("   Actual duration: %.1fs, Resolution: %dx%d, Video: %v, Audio: %v\n",
			result.Duration, result.Width, result.Height, result.HasVideoStream, result.HasAudioStream)
	}

	return result
}

func (s *CleanupService) deleteInvalidVideos(videos []InvalidVideo) (deleted, failed int) {
	for _, video := range videos {
		if err := os.Remove(video.Path); err != nil {
			log.Printf("Failed to delete %s: %v", video.Path, err)
			failed++
		} else {
			log.Printf("Deleted: %s", filepath.Base(video.Path))
			deleted++
		}
	}
	return
}

func parseFloat(s string) (float64, error) {
	// Handle the case where FFprobe might return scientific notation or other formats
	var f float64
	n, err := fmt.Sscanf(s, "%f", &f)
	if err != nil || n != 1 {
		return 0, fmt.Errorf("invalid float: %s", s)
	}
	return f, nil
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
