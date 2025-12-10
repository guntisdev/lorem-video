## Getting started
First one build binary, then it runs it
```
task build:stats
./bin/stats
```

### Command Line Options
`--exclude-static` (default: true) - Exclude /web/ static file requests
`--exclude-partial` (default: true) - Exclude partial content (206) responses
`--min-date` - Filter from this date (YYYY-MM-DD format)
`--max-date` - Filter until this date (YYYY-MM-DD format)
`--top` (default: 20) - Number of top results to show
`--full-ua` - Show full user agent strings instead of browser summary

### Usage Examples
Basic Analysis (All Data)
`./bin/stats`
Recent Data Analysis
`./bin/stats -min-date 2025-12-07 -top 10`
Specific Date Range
`./bin/stats -min-date 2025-12-01 -max-date 2025-12-05 -top 10`
Show All Static Files Too
`./bin/stats -exclude-static=false`
Include Partial Content Requests
`./bin/stats -exclude-partial=false`
Comprehensive Analysis (Show Everything)
`./bin/stats -exclude-static=false -exclude-partial=false -top 50`