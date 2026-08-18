package config

import "testing"

func TestValidateContainerCompatibility(t *testing.T) {
	tests := []struct {
		name    string
		spec    VideoSpec
		wantErr bool
	}{
		{name: "h264/aac/mp4", spec: VideoSpec{Codec: "h264", AudioCodec: "aac", Container: "mp4"}},
		{name: "h265/aac/mp4", spec: VideoSpec{Codec: "h265", AudioCodec: "aac", Container: "mp4"}},
		{name: "vp9/opus/mp4", spec: VideoSpec{Codec: "vp9", AudioCodec: "opus", Container: "mp4"}},
		{name: "av1/opus/mp4", spec: VideoSpec{Codec: "av1", AudioCodec: "opus", Container: "mp4"}},
		{name: "vp8/opus/webm", spec: VideoSpec{Codec: "vp8", AudioCodec: "opus", Container: "webm"}},
		{name: "vp9/vorbis/webm", spec: VideoSpec{Codec: "vp9", AudioCodec: "vorbis", Container: "webm"}},
		{name: "av1/opus/webm", spec: VideoSpec{Codec: "av1", AudioCodec: "opus", Container: "webm"}},
		{name: "novideo in webm", spec: VideoSpec{Codec: "novideo", AudioCodec: "opus", Container: "webm"}},
		{name: "noaudio in webm", spec: VideoSpec{Codec: "vp8", AudioCodec: "noaudio", Container: "webm"}},

		{name: "vp8 in mp4", spec: VideoSpec{Codec: "vp8", AudioCodec: "aac", Container: "mp4"}, wantErr: true},
		{name: "h264 in webm", spec: VideoSpec{Codec: "h264", AudioCodec: "opus", Container: "webm"}, wantErr: true},
		{name: "h265 in webm", spec: VideoSpec{Codec: "h265", AudioCodec: "opus", Container: "webm"}, wantErr: true},
		{name: "aac in webm", spec: VideoSpec{Codec: "vp9", AudioCodec: "aac", Container: "webm"}, wantErr: true},
		{name: "unknown container", spec: VideoSpec{Codec: "h264", AudioCodec: "aac", Container: "mkv"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateContainerCompatibility(&tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateContainerCompatibility() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// A codec with no container is listed on the docs page but cannot be used.
func TestEveryCodecHasAContainer(t *testing.T) {
	for _, codec := range ValidVideoCodecs {
		found := false
		for _, container := range ValidContainers {
			for _, supported := range ContainerVideoCodecs[container] {
				if supported == codec {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("video codec %s is not supported by any container", codec)
		}
	}

	for _, codec := range ValidAudioCodecs {
		found := false
		for _, container := range ValidContainers {
			for _, supported := range ContainerAudioCodecs[container] {
				if supported == codec {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("audio codec %s is not supported by any container", codec)
		}
	}
}

// Container tables must not list a codec missing from the encoder maps.
func TestCompatibilityTablesOnlyListKnownCodecs(t *testing.T) {
	for container, codecs := range ContainerVideoCodecs {
		for _, codec := range codecs {
			if _, ok := VideoCodecNameMap[codec]; !ok {
				t.Errorf("%s lists unknown video codec %s", container, codec)
			}
		}
	}

	for container, codecs := range ContainerAudioCodecs {
		for _, codec := range codecs {
			if _, ok := AudioCodecNameMap[codec]; !ok {
				t.Errorf("%s lists unknown audio codec %s", container, codec)
			}
		}
	}
}
