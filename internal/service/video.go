package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"lorem.video/internal/config"
	"lorem.video/internal/parser"
)

type VideoService struct {
}

func NewVideoService() *VideoService {
	return &VideoService{}
}

func (s *VideoService) GetInfo(name string) (*config.FFProbeOutput, error) {
	// TODO convert name to spec and chek data/video first and then data/tmp
	// HACK "bunny" is hardoded for now
	videoPath := filepath.Join(config.AppPaths.Video, "bunny", name)

	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("video not found: %s", name)
	}

	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		videoPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var info config.FFProbeOutput
	if err := json.Unmarshal(output, &info); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	return &info, nil
}

// TranscodeFromParams parses parameters and calls Transcode with appropriate paths
func (s *VideoService) TranscodeFromParams(ctx context.Context, paramsStr string) (<-chan string, <-chan error) {
	inputParams, err := parser.ParseFilename(paramsStr)
	if err != nil {
		errCh := make(chan error, 1)
		errCh <- err
		close(errCh)
		return nil, errCh
	}

	spec := config.ApplyDefaultVideoSpec(inputParams)

	// Operates only with default source video for now
	inputPath := config.AppPaths.DefaultSourceVideo
	outputPath := config.AppPaths.Video

	return s.Transcode(ctx, spec, inputPath, outputPath)
}

// Transcode performs video transcoding with the given VideoSpec and paths
func (s *VideoService) Transcode(ctx context.Context, spec config.VideoSpec, inputPath, outputPath string) (<-chan string, <-chan error) {
	resultCh := make(chan string, 1)
	errCh := make(chan error, 1)

	// Generate proper filename from the VideoSpec
	filename := parser.GenerateFilename(&spec)
	fullOutputPath := filepath.Join(outputPath, filename)

	// Check if file already exists
	if _, err := os.Stat(fullOutputPath); err == nil {
		go func() {
			defer close(resultCh)
			defer close(errCh)
			resultCh <- fullOutputPath
		}()
		return resultCh, errCh
	}

	go func() {
		defer close(resultCh)
		defer close(errCh)

		args := []string{
			"-y",                   // overwrite output files
			"-loglevel", "warning", // reduce log verbosity
			"-threads", "2", // limit CPU threads for VPS
			"-i", inputPath,
			"-t", fmt.Sprintf("%d", spec.Duration),
			"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d",
				spec.Width, spec.Height, spec.Width, spec.Height),
		}

		// minimal header for streaming/progressive playback (To not download whole file)
		// not to confuse with live streaming HLS, it's chunked differently
		switch spec.Container {
		case "mp4":
			args = append(args, "-movflags", "frag_keyframe+empty_moov")
		case "webm":
			args = append(args, "-f", "webm")
		}

		videoCodec := config.VideoCodecNameMap[spec.Codec]

		if videoCodec != "none" {
			args = append(args,
				"-c:v", videoCodec,
				"-r", fmt.Sprintf("%d", spec.FPS),
			)

			if codecArgs, ok := config.VideoCodecArgs[videoCodec]; ok {
				args = append(args, codecArgs...)
			}
		} else {
			args = append(args, "-vn") // no video
		}

		// Bitrate handling
		if strings.HasSuffix(spec.Bitrate, "crf") {
			crf := strings.TrimSuffix(spec.Bitrate, "crf")
			args = append(args, "-crf", crf)
		} else if strings.HasSuffix(spec.Bitrate, "cbr") {
			bitrate := strings.TrimSuffix(spec.Bitrate, "cbr")
			args = append(args, "-b:v", bitrate+"k", "-maxrate", bitrate+"k", "-bufsize", bitrate+"k")
		} else if strings.HasSuffix(spec.Bitrate, "vbr") {
			bitrate := strings.TrimSuffix(spec.Bitrate, "vbr")
			args = append(args, "-b:v", bitrate+"k")
		}

		audioCodec := config.AudioCodecNameMap[spec.AudioCodec]
		if audioCodec != "none" {
			args = append(args,
				"-c:a", audioCodec, // audio codec
				"-b:a", fmt.Sprintf("%dk", spec.AudioBitrate), // audio bitrate
				"-ac", "2", // force 2 channels (stereo)
			)
		} else {
			args = append(args, "-an") // no audio
		}

		args = append(args, fullOutputPath)

		cmd := exec.CommandContext(ctx, "ffmpeg", args...)

		// Add resource limits for VPS environments
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true, // Create new process group for better cleanup
		}

		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			log.Printf("FFmpeg failed with error: %v", err)
			log.Printf("FFmpeg stderr output: %s", stderr.String())

			// Clean up partial file on failure
			if _, statErr := os.Stat(fullOutputPath); statErr == nil {
				if removeErr := os.Remove(fullOutputPath); removeErr != nil {
					log.Printf("Failed to clean up partial file: %v", removeErr)
				}
			}

			errCh <- fmt.Errorf("ffmpeg failed: %w\nOutput: %s", err, stderr.String())
			return
		}

		log.Printf("Transcode success: %s", filepath.Base(fullOutputPath))

		resultCh <- fullOutputPath
	}()

	return resultCh, errCh

}

func (s *VideoService) TranscodeHLS(ctx context.Context, res config.Resolution, inputPath, outputPath string) (<-chan string, <-chan error) {
	resultCh := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		defer close(resultCh)
		defer close(errCh)

		playlistPath := filepath.Join(outputPath, config.HLSMediaPlaylist)

		args := []string{
			"-i", inputPath,
			"-t", "60", // max duration 60sec
			"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d",
				res.Width, res.Height, res.Width, res.Height),
			"-c:v", "libx264",
			"-preset", "fast",
			"-r", "25",
			"-g", "25",
			"-keyint_min", "25",
			"-sc_threshold", "0", // Disable scene change detection (keeps GOP exactly 25)
			"-crf", "23",
			"-c:a", "aac",
			"-b:a", "128k",
			"-ac", "2",
			"-f", "hls",
			"-hls_time", "1",
			"-hls_list_size", "0",
			"-hls_segment_type", "fmp4",
			"-hls_flags", "independent_segments", // Each segment independently decodable
			"-hls_fmp4_init_filename", config.HLSInit,
			"-hls_segment_filename", filepath.Join(outputPath, config.HLSChunkFormat),
			playlistPath,
		}

		cmd := exec.CommandContext(ctx, "ffmpeg", args...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			errCh <- fmt.Errorf("ffmpeg failed: %w\nOutput: %s", err, stderr.String())
			return
		}

		resultCh <- playlistPath
	}()

	return resultCh, errCh
}
