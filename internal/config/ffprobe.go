package config

type FFProbeOutput struct {
	Streams []FFprobeStream `json:"streams"`
	Format  FFprobeFormat   `json:"format"`
}

type FFprobeStream struct {
	Index         int    `json:"index"`
	CodecName     string `json:"codec_name"`
	CodecLongName string `json:"codec_long_name"`
	CodecType     string `json:"codec_type"`
	Width         int    `json:"width,omitempty"`
	Height        int    `json:"height,omitempty"`
	RFrameRate    string `json:"r_frame_rate,omitempty"`
	AvgFrameRate  string `json:"avg_frame_rate,omitempty"`
	BitRate       string `json:"bit_rate,omitempty"`
	Duration      string `json:"duration,omitempty"`
	SampleRate    string `json:"sample_rate,omitempty"`
	Channels      int    `json:"channels,omitempty"`
	ChannelLayout string `json:"channel_layout,omitempty"`
}

type FFprobeFormat struct {
	Filename       string `json:"filename"`
	NbStreams      int    `json:"nb_streams"`
	FormatName     string `json:"format_name"`
	FormatLongName string `json:"format_long_name"`
	Duration       string `json:"duration"`
	Size           string `json:"size"`
	BitRate        string `json:"bit_rate"`
}
