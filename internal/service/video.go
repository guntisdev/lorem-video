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

	"lorem.video/internal/config"
	"lorem.video/internal/parser"
)

type VideoService struct {
	pregenService *PregenerationService
}

func NewVideoService() *VideoService {
	s := &VideoService{}
	s.pregenService = NewPregenerationService(s)
	return s
}

// StartupPregeneration delegates to the pregeneration service
func (s *VideoService) StartupPregeneration() {
	s.pregenService.StartupPregeneration()
}

// GetOrGenerate is the universal method that ensures a video exists for given parameters
// It searches for existing videos first, then generates if not found
func (s *VideoService) GetOrGenerate(ctx context.Context, paramsStr string) (<-chan string, <-chan error) {
	resultCh := make(chan string, 1)
	errCh := make(chan error, 1)

	inputParams, err := parser.ParseFilename(paramsStr)
	if err != nil {
		go func() {
			defer close(errCh)
			defer close(resultCh)
			errCh <- fmt.Errorf("failed to parse filename parameters: %w", err)
		}()
		return resultCh, errCh
	}

	spec := config.ApplyDefaultVideoSpec(inputParams)
	filename := parser.GenerateFilename(&spec)

	existingPath := parser.FindExistingVideo(filename, &spec)
	if existingPath != "" {
		go func() {
			defer close(resultCh)
			defer close(errCh)
			resultCh <- existingPath
		}()
		return resultCh, errCh
	}

	// TODO harcdoded .mp4 extension for source video. should be improved later
	inputPath := filepath.Join(config.AppPaths.SourceVideo, spec.Name+".mp4")
	if _, err := os.Stat(inputPath); err != nil {
		go func() {
			defer close(errCh)
			defer close(resultCh)
			errCh <- fmt.Errorf("failed to find source video: %s", spec.Name)
		}()
		return resultCh, errCh
	}

	log.Printf("Video not found, generating: %s", filename)

	return s.Transcode(ctx, spec, inputPath, config.AppPaths.Tmp)
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
	inputPath := s.pregenService.GetDefaultSourceVideo()
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
			"-i", inputPath,
			"-t", fmt.Sprintf("%d", spec.Duration),
			"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d",
				spec.Width, spec.Height, spec.Width, spec.Height),
		}

		// streaming flags based on format
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

		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			log.Printf("FFmpeg failed with error: %v", err)
			log.Printf("FFmpeg stderr output: %s", stderr.String())
			errCh <- fmt.Errorf("ffmpeg failed: %w\nOutput: %s", err, stderr.String())
			return
		}

		log.Printf("Transcode success: %s", filepath.Base(fullOutputPath))

		resultCh <- fullOutputPath
	}()

	return resultCh, errCh

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
