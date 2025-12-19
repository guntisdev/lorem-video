
# Lorem Video 📹

A video placeholder service for developers. Generate test videos with custom dimensions, codecs, fps, and durations on-demand.

## 🚀 Features

- **On-demand video generation** - Custom dimensions, codecs, and durations
- **Multiple codecs/containers** - MP4, WebM, AV1, VP9, H.264, HEVC support
- **Partial streaming** - Videos start playing while being generated
- **Caching** - Popular combinations are pregenerated, user specific videos cached after first request
- **Developer friendly** - CORS `"Access-Control-Allow-Origin", "*"`
- **Built-in analytics** - Stats from rest endpoints

## 🛠️ Quick Start

### Docker (Recommended)
```bash
docker compose up --build -d
```

### Local Development
```bash
# Install dependencies
go mod download

# Run development server
task run

# Build production binary
task build
```

## 📊 API Usage

### Generate Video
```
GET /{width}x{height}_{duration}s_{codec}_{quality}
GET /800x600_30s_h264_25crf        # H.264 video
GET /1920x1080_60s_vp9_23crf       # VP9 video
GET /bunny                         # Default test video
```

### Get Video Info
```
GET /getInfo/{filename}
```

### Static Files
```
GET /
GET /web/*
GET /sitemap.xml
GET /robots.txt
```

## 🧰 Development

### Data Directories
- `/data/video/` - Generated video cache
- `/data/logs/` - Daily stats logs (JSONL format)
- `/data/errors/` - Error logs (created only when errors occur)
- `/data/tmp/` - Temporary transcoding files
- `/data/sourceVideo/` - Source video files (bunny.mp4)

### Task Commands
```bash
task run              # Development server with auto-reload
task build            # Build server binary
task build:stats      # Build stats analyzer
task test             # Run tests
task test:integration # Run integration tests (requires FFmpeg)
task deps             # Download and tidy dependencies
```

### FFmpeg commands
```bash
// source file duration
ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 input.mp4
// add padding to get GOP to round second
ffmpeg -i input.mp4 -vf "tpad=stop_mode=clone:stop_duration=0.544" output.mp4
```

### Project Structure
```
├── cmd/
│   ├── server/       # Main application
│   └── stats/        # Analytics CLI tool
├── internal/
│   ├── config/       # Configuration and paths
│   ├── parser/       # Video parameter parsing
│   ├── rest/         # HTTP handlers and middleware
│   ├── service/      # Video transcoding logic
│   └── stats/        # Request logging and analysis
├── web/dist/         # Static files and documentation
└── data/             # Runtime data (mounted in Docker)
```

### Pregenerated Cache
Common combinations are generated at startup:
- Multiple resolutions (480p, 720p, 1080p)
- Different codecs (H.264, VP9, av1)
- Duration 20s


## 📈 Statistics

### Getting started
Build binary and then run it
```
task build:stats
./bin/stats
```

### Command Line Options
`--exclude-static` (default: true) - Exclude /web/ static file requests\
`--exclude-partial` (default: true) - Exclude partial content (206) responses\
`exclude-referer` (default: "lorem.video") - Eclude domain from referer\
`--min-date` - (default: date 7 days ago) Filter from this date (YYYY-MM-DD format)\
`--max-date` - Filter until this date (YYYY-MM-DD format)\
`--top` (default: 20) - Number of top results to show\
`--full-ua` - Show full user agent strings instead of browser summary

### Usage Examples
Basic Analysis (All Data)\
`./bin/stats`\
Recent Data Analysis\
`./bin/stats -min-date 2025-12-07 -top 10`\
Specific Date Range\
`./bin/stats -min-date 2025-12-01 -max-date 2025-12-05 -top 10`\
Show All Static Files Too\
`./bin/stats -exclude-static=false`\
Include Partial Content Requests\
`./bin/stats -exclude-partial=false`\
Comprehensive Analysis (Show Everything)\
`./bin/stats -exclude-static=false -exclude-partial=false -top 50`

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `task test`
5. Submit a pull request

## 📄 License

MIT License - see LICENSE file for details

## 🌐 Live Service

Visit [lorem.video](https://lorem.video) for the hosted version with full documentation and examples.
