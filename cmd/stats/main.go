package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"lorem.video/internal/stats"
)

func main() {
	var (
		excludeStatic  = flag.Bool("exclude-static", true, "Exclude /web/... paths")
		excludePartial = flag.Bool("exclude-partial", true, "Exclude partial content (206 responses)")
		minDate        = flag.String("min-date", "", "Minimum date YYYY-MM-DD (empty for all)")
		maxDate        = flag.String("max-date", "", "Maximum date YYYY-MM-DD (empty for all)")
		topN           = flag.Int("top", 20, "Number of top results to show")
		showFullUA     = flag.Bool("full-ua", false, "Show full user agent strings")
	)
	flag.Parse()

	analyzerConfig := stats.AnalyzerConfig{
		ExcludeStaticPaths: *excludeStatic,
		ExcludePartial:     *excludePartial,
		MinDate:            *minDate,
		MaxDate:            *maxDate,
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
	// Overview
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

	// Top Endpoints
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

	// Top Visitors
	if len(result.TopVisitors) > 0 {
		fmt.Printf("👥 TOP VISITORS (Top %d)\n", topN)
		fmt.Printf("═══════════════════════════════════════\n")
		fmt.Printf("%-15s %10s %12s\n", "IP", "Requests", "Bytes")
		fmt.Printf("%-15s %10s %12s\n", strings.Repeat("-", 15), strings.Repeat("-", 10), strings.Repeat("-", 12))
		for i, visitor := range result.TopVisitors {
			if i >= topN {
				break
			}
			fmt.Printf("%-15s %10d %12s\n", visitor.IP, visitor.Requests, formatBytes(visitor.Bytes))
		}
		fmt.Printf("\n")
	}

	// Top Referrers
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

	// Full Referrer URLs
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

	// Browser summary
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

	// Bots and Crawlers
	if len(result.Bots) > 0 {
		fmt.Printf("🤖 BOTS & CRAWLERS (Top %d)\n", topN)
		fmt.Printf("═══════════════════════════════════════\n")
		fmt.Printf("%-60s %10s\n", "Bot/Crawler", "Count")
		fmt.Printf("%-60s %10s\n", strings.Repeat("-", 60), strings.Repeat("-", 10))
		for i, bot := range result.Bots {
			if i >= topN {
				break
			}
			name := bot.UserAgent
			if len(name) > 57 {
				name = name[:54] + "..."
			}
			fmt.Printf("%-60s %10d\n", name, bot.Count)
		}
		fmt.Printf("\n")
	}

	// Rate limiting insights
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
		browser := detectBrowser(ua.UserAgent)
		browsers[browser] += ua.Count
	}

	var result []BrowserSummary
	for name, count := range browsers {
		result = append(result, BrowserSummary{Name: name, Count: count})
	}

	// Sort by count
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Count > result[i].Count {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

func detectBrowser(ua string) string {
	ua = strings.ToLower(ua)

	if strings.Contains(ua, "chrome") && !strings.Contains(ua, "edg") {
		return "Chrome"
	}
	if strings.Contains(ua, "firefox") {
		return "Firefox"
	}
	if strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome") {
		return "Safari"
	}
	if strings.Contains(ua, "edg") {
		return "Edge"
	}
	if strings.Contains(ua, "opera") {
		return "Opera"
	}
	if strings.Contains(ua, "curl") {
		return "cURL"
	}
	if strings.Contains(ua, "wget") {
		return "wget"
	}
	if strings.Contains(ua, "python") {
		return "Python"
	}
	if strings.Contains(ua, "go-http") {
		return "Go HTTP Client"
	}

	return "Other"
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
