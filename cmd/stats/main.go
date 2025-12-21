package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"lorem.video/internal/config"
	"lorem.video/internal/stats"
)

func main() {
	// Default to last 7 days
	defaultMinDate := time.Now().AddDate(0, 0, -7).Format("2006-01-02")

	var (
		excludeStatic  = flag.Bool("exclude-static", true, "Exclude /web/... paths")
		excludePartial = flag.Bool("exclude-partial", true, "Exclude partial content (206 responses)")
		// TODO replace lorem.video with GetBaseURL() from config
		excludeReferer = flag.String("exclude-referer", "lorem.video", "Exclude referrers containing this domain (empty to include all)")
		minDate        = flag.String("min-date", defaultMinDate, "Minimum date YYYY-MM-DD (default: 7 days ago)")
		maxDate        = flag.String("max-date", "", "Maximum date YYYY-MM-DD (empty for all)")
		topN           = flag.Int("top", 20, "Number of top results to show")
		showFullUA     = flag.Bool("full-ua", false, "Show full user agent strings")
		showBots       = flag.Bool("bots", false, "Show stats from bots folder")
	)
	flag.Parse()

	analyzerConfig := stats.AnalyzerConfig{
		ExcludeStaticPaths: *excludeStatic,
		ExcludePartial:     *excludePartial,
		ExcludeReferer:     *excludeReferer,
		MinDate:            *minDate,
		MaxDate:            *maxDate,
		LogDir: func() string {
			if *showBots {
				return config.AppPaths.LogsBots
			} else {
				return config.AppPaths.LogsStats
			}
		}(),
	}

	fmt.Printf("🔍 Analyzing stats...\n\n")

	result, err := stats.AnalyzeStats(analyzerConfig)
	if err != nil {
		fmt.Printf("Error analyzing stats: %v\n", err)
		os.Exit(1)
	}

	printResults(result, *topN, *showFullUA)
}

func printResults(result *stats.AnalysisResult, topN int, showFullUA bool) {
	fmt.Printf("+++++++++++++++++++++++++++++++++++++++++++++++++++++++\n")
	fmt.Printf("+++++++++++++++++++++++++++++++++++++++++++++++++++++++\n")
	fmt.Printf("📊 OVERVIEW\n")
	fmt.Printf("═══════════════════════════════════════\n")
	fmt.Printf("Date Range:         %s\n", result.DateRange)
	fmt.Printf("Total Requests:     %s\n", formatNumber(result.TotalRequests))
	fmt.Printf("Unique Visitors:    %s\n", formatNumber(result.UniqueVisitors))
	fmt.Printf("Total Bytes:        %s\n", formatBytes(result.TotalBytes))
	fmt.Printf("Video Requests:     %s\n", formatNumber(result.VideoRequests))
	fmt.Printf("Static Requests:    %s\n", formatNumber(result.StaticRequests))
	fmt.Printf("Partial Requests:   %s\n", formatNumber(result.PartialRequests))
	fmt.Printf("Error Requests:     %s\n", formatNumber(result.ErrorRequests))
	fmt.Printf("\n")

	if len(result.TopEndpoints) > 0 {
		fmt.Printf("🎯 TOP ENDPOINTS (Top %d)\n", topN)
		fmt.Printf("═══════════════════════════════════════\n")
		fmt.Printf("%-50s %10s %12s\n", "Path", "Requests", "Bytes")
		fmt.Printf("%-50s %10s %12s\n", strings.Repeat("-", 50), strings.Repeat("-", 10), strings.Repeat("-", 12))
		for i, ep := range result.TopEndpoints {
			if i >= topN {
				break
			}
			path := ep.Path
			if len(path) > 47 {
				path = path[:44] + "..."
			}
			fmt.Printf("%-50s %10d %12s\n", path, ep.Count, formatBytes(ep.Bytes))
		}
		fmt.Printf("\n")
	}

	if len(result.TopVisitors) > 0 {
		fmt.Printf("👥 TOP VISITORS (Top %d)\n", topN)
		fmt.Printf("═══════════════════════════════════════\n")
		fmt.Printf("%-15s %-15s %10s %12s\n", "IP", "Browser", "Requests", "Bytes")
		fmt.Printf("%-15s %-15s %10s %12s\n", strings.Repeat("-", 15), strings.Repeat("-", 15), strings.Repeat("-", 10), strings.Repeat("-", 12))
		for i, visitor := range result.TopVisitors {
			if i >= topN {
				break
			}
			browser := visitor.Browser
			if len(browser) > 12 {
				browser = browser[:9] + "..."
			}
			fmt.Printf("%-15s %-15s %10d %12s\n", visitor.IP, browser, visitor.Requests, formatBytes(visitor.Bytes))
		}
		fmt.Printf("\n")
	}

	if len(result.TopReferrers) > 0 {
		fmt.Printf("🔗 TOP REFERRER DOMAINS (Top %d)\n", topN)
		fmt.Printf("═══════════════════════════════════════\n")
		fmt.Printf("%-40s %10s %20s\n", "Domain", "Count", "Last Seen")
		fmt.Printf("%-40s %10s %20s\n", strings.Repeat("-", 40), strings.Repeat("-", 10), strings.Repeat("-", 20))
		for i, ref := range result.TopReferrers {
			if i >= topN {
				break
			}
			domain := ref.Domain
			if len(domain) > 37 {
				domain = domain[:34] + "..."
			}
			fmt.Printf("%-40s %10d %20s\n", domain, ref.Count, ref.LastSeen.Format("2006-01-02 15:04"))
		}
		fmt.Printf("\n")
	}

	if len(result.FullReferrerURLs) > 0 {
		fmt.Printf("📄 FULL REFERRER URLS (Top %d)\n", topN)
		fmt.Printf("═══════════════════════════════════════\n")
		fmt.Printf("%-80s %10s\n", "Full URL", "Count")
		fmt.Printf("%-80s %10s\n", strings.Repeat("-", 80), strings.Repeat("-", 10))
		for i, ref := range result.FullReferrerURLs {
			if i >= topN {
				break
			}
			url := ref.FullURL
			if len(url) > 77 {
				url = url[:74] + "..."
			}
			fmt.Printf("%-80s %10d\n", url, ref.Count)
		}
		fmt.Printf("\n")
	}

	if len(result.UserAgents) > 0 {
		browsers := summarizeBrowsers(result.UserAgents)
		fmt.Printf("🌐 BROWSER SUMMARY (Top %d)\n", topN)
		fmt.Printf("═══════════════════════════════════════\n")
		fmt.Printf("%-30s %10s\n", "Browser", "Count")
		fmt.Printf("%-30s %10s\n", strings.Repeat("-", 30), strings.Repeat("-", 10))
		for i, browser := range browsers {
			if i >= topN {
				break
			}
			fmt.Printf("%-30s %10d\n", browser.Name, browser.Count)
		}
		fmt.Printf("\n")
	}

	if len(result.Bots) > 0 {
		fmt.Printf("🤖 BOTS & CRAWLERS (Top %d)\n", topN)
		fmt.Printf("═══════════════════════════════════════\n")
		fmt.Printf("%-60s %10s\n", "Bot/Crawler", "Count")
		fmt.Printf("%-60s %10s\n", strings.Repeat("-", 60), strings.Repeat("-", 10))
		for i, bot := range result.Bots {
			if i >= topN {
				break
			}
			name := stats.ExtractBotName(bot.UserAgent)
			fmt.Printf("%-60s %10d\n", name, bot.Count)
		}
		fmt.Printf("\n")
	}

	fmt.Printf("🚦 RATE LIMITING INSIGHTS\n")
	fmt.Printf("═══════════════════════════════════════\n")
	heavyUsers := 0
	for _, visitor := range result.TopVisitors {
		if visitor.Requests > 100 {
			heavyUsers++
		}
	}
	fmt.Printf("Heavy users (>100 requests): %d\n", heavyUsers)
	if len(result.TopVisitors) > 0 {
		fmt.Printf("Top user requests: %d\n", result.TopVisitors[0].Requests)
		fmt.Printf("Top user bytes: %s\n", formatBytes(result.TopVisitors[0].Bytes))
	}
}

type BrowserSummary struct {
	Name  string
	Count int
}

func summarizeBrowsers(userAgents []stats.UserAgentStat) []BrowserSummary {
	browsers := make(map[string]int)

	for _, ua := range userAgents {
		browser := stats.ExtractBrowserName(ua.UserAgent)
		browsers[browser] += ua.Count
	}

	var result []BrowserSummary
	for name, count := range browsers {
		result = append(result, BrowserSummary{Name: name, Count: count})
	}

	// Sort by count (descending)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	return result
}

func formatNumber(n int) string {
	str := strconv.Itoa(n)
	if len(str) <= 3 {
		return str
	}

	var result []string
	for i := len(str); i > 0; i -= 3 {
		start := i - 3
		if start < 0 {
			start = 0
		}
		result = append([]string{str[start:i]}, result...)
	}

	return strings.Join(result, ",")
}

func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
	}
	if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1fMB", float64(bytes)/(1024*1024))
	}
	return fmt.Sprintf("%.1fGB", float64(bytes)/(1024*1024*1024))
}
