package service

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"lorem.video/internal/config"
)

const wsEncodePeriods = 3

type wsBitrateKbps struct {
	Video int
	Audio int
}

var wsBitrates = map[string]wsBitrateKbps{
	"360p":  {Video: 500, Audio: 96},
	"480p":  {Video: 800, Audio: 96},
	"720p":  {Video: 2000, Audio: 128},
	"1080p": {Video: 4000, Audio: 128},
}

func SliceLoopPeriod(clusters []WebMCluster, periodMs, clusterMs uint64) ([]WebMCluster, error) {
	if len(clusters) == 0 {
		return nil, fmt.Errorf("no clusters")
	}

	if end := clusters[len(clusters)-1].TimecodeMs + clusterMs; end < 3*periodMs {
		return nil, fmt.Errorf("input covers %dms, need %dms", end, 3*periodMs)
	}

	var kept []WebMCluster
	for _, c := range clusters {
		if c.TimecodeMs >= periodMs && c.TimecodeMs < 2*periodMs {
			kept = append(kept, c)
		}
	}

	want := int(periodMs / clusterMs)
	if len(kept) != want {
		return nil, fmt.Errorf("sliced %d clusters, want %d", len(kept), want)
	}

	base := kept[0].TimecodeMs
	out := make([]WebMCluster, len(kept))

	for i, c := range kept {
		rel := c.TimecodeMs - base
		if rel != uint64(i)*clusterMs {
			return nil, fmt.Errorf("cluster %d at %dms, want %dms", i, rel, uint64(i)*clusterMs)
		}
		if err := PatchClusterTimecode(c.Data, rel); err != nil {
			return nil, err
		}
		c.TimecodeMs = rel
		out[i] = c
	}

	return out, nil
}

func transcodeLoopSource(ctx context.Context, res config.Resolution, inputPath, outputPath string) error {
	args := []string{
		"-y",
		"-loglevel", "warning",
		"-i", inputPath,
		"-t", strconv.Itoa(config.WSLoopMs / 1000),
		"-r", strconv.Itoa(config.WSFPS),
		"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d",
			res.Width, res.Height, res.Width, res.Height),
		"-c:v", "libx264",
		"-crf", "18",
		"-c:a", "pcm_s16le",
		"-ar", "48000",
		"-ac", "2",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w\nOutput: %s", err, stderr.String())
	}

	return countFrames(ctx, outputPath, config.WSLoopMs/1000*config.WSFPS)
}

func countFrames(ctx context.Context, path string, want int) error {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-count_packets",
		"-show_entries", "stream=nb_read_packets",
		"-of", "csv=p=0",
		path,
	)

	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("ffprobe failed: %w", err)
	}

	got, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(string(out)), ",")))
	if err != nil {
		return fmt.Errorf("failed to parse frame count %q: %w", out, err)
	}

	if got != want {
		return fmt.Errorf("%d frames, want %d, source may be shorter than %dms", got, want, config.WSLoopMs)
	}

	return nil
}

func transcodeLoopWebM(ctx context.Context, bitrate wsBitrateKbps, inputPath, outputPath string) error {
	gop := config.WSFPS * config.WSClusterMs / 1000
	seconds := wsEncodePeriods * config.WSLoopMs / 1000

	args := []string{
		"-y",
		"-loglevel", "warning",
		"-stream_loop", strconv.Itoa(wsEncodePeriods - 1),
		"-i", inputPath,
		"-t", strconv.Itoa(seconds),
		"-c:v", "libvpx",
		"-b:v", strconv.Itoa(bitrate.Video) + "k",
		"-deadline", "realtime",
		"-cpu-used", "8",
		"-r", strconv.Itoa(config.WSFPS),
		"-g", strconv.Itoa(gop),
		"-keyint_min", strconv.Itoa(gop),
		"-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%d)", config.WSClusterMs/1000),
		"-c:a", "libopus",
		"-b:a", strconv.Itoa(bitrate.Audio) + "k",
		"-ac", "2",
		"-f", "webm",
		"-live", "1",
		"-cluster_time_limit", strconv.Itoa(config.WSClusterMs),
		"-cluster_size_limit", "100000000", // large so only cluster_time_limit splits clusters
		outputPath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w\nOutput: %s", err, stderr.String())
	}

	return countFrames(ctx, outputPath, seconds*config.WSFPS)
}
