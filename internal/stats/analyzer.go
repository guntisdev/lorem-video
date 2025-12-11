package stats

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mileusna/useragent"
	"lorem.video/internal/config"
)

type AnalyzerConfig struct {
	ExcludeStaticPaths bool   // Filter out /web/... paths
	ExcludePartial     bool   // Filter out partial content (206 status)
	ExcludeReferer     string // Filter out referrers containing this domain
	MinDate            string // YYYY-MM-DD format, empty for all
	MaxDate            string // YYYY-MM-DD format, empty for all
}

type EndpointStat struct {
	Path  string
	Count int
	Bytes int64
}

type VisitorStat struct {
	IP        string
	UserAgent string
	Browser   string // Detected browser/bot name
	Requests  int
	Bytes     int64
	FirstSeen time.Time
	LastSeen  time.Time
}

type ReferrerStat struct {
	Domain   string
	FullURL  string
	Count    int
	LastSeen time.Time
}

type UserAgentStat struct {
	UserAgent string
	Count     int
	IsBot     bool
}

type AnalysisResult struct {
	TotalRequests  int
	UniqueVisitors int
	TotalBytes     int64
	DateRange      string

	TopEndpoints     []EndpointStat
	TopVisitors      []VisitorStat
	TopReferrers     []ReferrerStat
	FullReferrerURLs []ReferrerStat
	UserAgents       []UserAgentStat
	Bots             []UserAgentStat

	// Quick insights
	VideoRequests   int
	StaticRequests  int
	PartialRequests int
	ErrorRequests   int
}

func AnalyzeStats(analyzerConfig AnalyzerConfig) (*AnalysisResult, error) {
	logDir := config.AppPaths.Logs

	// Find all log files in date range
	files, err := findLogFiles(logDir, analyzerConfig)
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return &AnalysisResult{}, nil
	}

	result := &AnalysisResult{
		TopEndpoints:     make([]EndpointStat, 0),
		TopVisitors:      make([]VisitorStat, 0),
		TopReferrers:     make([]ReferrerStat, 0),
		FullReferrerURLs: make([]ReferrerStat, 0),
		UserAgents:       make([]UserAgentStat, 0),
		Bots:             make([]UserAgentStat, 0),
	}

	// Maps for aggregation
	endpoints := make(map[string]*EndpointStat)
	visitors := make(map[string]*VisitorStat) // key: IP+UA
	referrers := make(map[string]*ReferrerStat)
	fullReferrers := make(map[string]*ReferrerStat)
	userAgents := make(map[string]*UserAgentStat)

	var minDate, maxDate time.Time

	// Process all log files
	for _, file := range files {
		err := processLogFile(file, analyzerConfig, result, endpoints, visitors, referrers, fullReferrers, userAgents, &minDate, &maxDate)
		if err != nil {
			fmt.Printf("Warning: Error processing %s: %v\n", file, err)
			continue
		}
	}

	// Convert maps to sorted slices
	result.TopEndpoints = sortEndpoints(endpoints)
	result.TopVisitors = sortVisitors(visitors)
	result.TopReferrers = sortReferrers(referrers)
	result.FullReferrerURLs = sortReferrers(fullReferrers)
	result.UserAgents, result.Bots = sortUserAgents(userAgents)

	result.UniqueVisitors = len(visitors)
	if !minDate.IsZero() && !maxDate.IsZero() {
		result.DateRange = fmt.Sprintf("%s to %s", minDate.Format("2006-01-02"), maxDate.Format("2006-01-02"))
	}

	return result, nil
}

func findLogFiles(logDir string, config AnalyzerConfig) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(logDir, "stats-*.jsonl"))
	if err != nil {
		return nil, err
	}

	if config.MinDate == "" && config.MaxDate == "" {
		return files, nil
	}

	var filtered []string
	for _, file := range files {
		base := filepath.Base(file)
		if strings.HasPrefix(base, "stats-") && strings.HasSuffix(base, ".jsonl") {
			dateStr := strings.TrimPrefix(base, "stats-")
			dateStr = strings.TrimSuffix(dateStr, ".jsonl")

			if config.MinDate != "" && dateStr < config.MinDate {
				continue
			}
			if config.MaxDate != "" && dateStr > config.MaxDate {
				continue
			}
			filtered = append(filtered, file)
		}
	}

	return filtered, nil
}

func processLogFile(filename string, config AnalyzerConfig, result *AnalysisResult,
	endpoints map[string]*EndpointStat, visitors map[string]*VisitorStat,
	referrers map[string]*ReferrerStat, fullReferrers map[string]*ReferrerStat,
	userAgents map[string]*UserAgentStat, minDate *time.Time, maxDate *time.Time) error {

	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var stat RequestStats
		if err := json.Unmarshal(scanner.Bytes(), &stat); err != nil {
			continue // Skip malformed lines
		}

		// Apply filters
		if config.ExcludeStaticPaths && strings.HasPrefix(stat.Path, "/web/") {
			continue
		}
		if config.ExcludePartial && stat.Status == 206 {
			continue
		}
		if config.ExcludeReferer != "" && stat.Referer != "" {
			referrerDomain := extractDomain(stat.Referer)
			if strings.Contains(referrerDomain, config.ExcludeReferer) {
				continue
			}
		}

		// Track date range
		if minDate.IsZero() || stat.Timestamp.Before(*minDate) {
			*minDate = stat.Timestamp
		}
		if maxDate.IsZero() || stat.Timestamp.After(*maxDate) {
			*maxDate = stat.Timestamp
		}

		result.TotalRequests++
		result.TotalBytes += stat.ResponseSize

		// Categorize requests
		categorizeRequest(&stat, result)

		// Track endpoints
		if ep, exists := endpoints[stat.Path]; exists {
			ep.Count++
			ep.Bytes += stat.ResponseSize
		} else {
			endpoints[stat.Path] = &EndpointStat{
				Path:  stat.Path,
				Count: 1,
				Bytes: stat.ResponseSize,
			}
		}

		// Track visitors (by IP + UA combination for better uniqueness)
		visitorKey := stat.IP + "|" + stat.UserAgent
		if visitor, exists := visitors[visitorKey]; exists {
			visitor.Requests++
			visitor.Bytes += stat.ResponseSize
			visitor.LastSeen = stat.Timestamp
		} else {
			visitors[visitorKey] = &VisitorStat{
				IP:        stat.IP,
				UserAgent: stat.UserAgent,
				Browser:   ExtractBrowserName(stat.UserAgent),
				Requests:  1,
				Bytes:     stat.ResponseSize,
				FirstSeen: stat.Timestamp,
				LastSeen:  stat.Timestamp,
			}
		}

		// Track referrers
		if stat.Referer != "" {
			// Full URL tracking
			if ref, exists := fullReferrers[stat.Referer]; exists {
				ref.Count++
				ref.LastSeen = stat.Timestamp
			} else {
				fullReferrers[stat.Referer] = &ReferrerStat{
					Domain:   extractDomain(stat.Referer),
					FullURL:  stat.Referer,
					Count:    1,
					LastSeen: stat.Timestamp,
				}
			}

			// Domain aggregation
			domain := extractDomain(stat.Referer)
			if domain != "" {
				if ref, exists := referrers[domain]; exists {
					ref.Count++
					ref.LastSeen = stat.Timestamp
				} else {
					referrers[domain] = &ReferrerStat{
						Domain:   domain,
						FullURL:  domain,
						Count:    1,
						LastSeen: stat.Timestamp,
					}
				}
			}
		}

		// Track user agents
		if ua, exists := userAgents[stat.UserAgent]; exists {
			ua.Count++
		} else {
			userAgents[stat.UserAgent] = &UserAgentStat{
				UserAgent: stat.UserAgent,
				Count:     1,
				IsBot:     isBot(stat.UserAgent),
			}
		}
	}

	return scanner.Err()
}

func categorizeRequest(stat *RequestStats, result *AnalysisResult) {
	if strings.HasPrefix(stat.Path, "/web/") {
		result.StaticRequests++
	} else if strings.HasPrefix(stat.Path, "/") && !strings.HasPrefix(stat.Path, "/info/") {
		result.VideoRequests++
	}

	if stat.Status == 206 {
		result.PartialRequests++
	}

	if stat.Status >= 400 {
		result.ErrorRequests++
	}
}

func extractDomain(referrer string) string {
	u, err := url.Parse(referrer)
	if err != nil {
		return referrer
	}
	return u.Host
}

func ExtractBrowserName(uaString string) string {
	ua := useragent.Parse(uaString)
	if ua.Name != "" {
		return ua.Name
	}

	return "Other"
}

func ExtractBotName(uaString string) string {
	if len(uaString) > 57 {
		uaString = uaString[:54] + "..."
	}
	return uaString
}

func isBot(uaString string) bool {
	ua := useragent.Parse(uaString)
	return ua.Bot
}

func sortEndpoints(endpoints map[string]*EndpointStat) []EndpointStat {
	var result []EndpointStat
	for _, ep := range endpoints {
		result = append(result, *ep)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})
	return result
}

func sortVisitors(visitors map[string]*VisitorStat) []VisitorStat {
	var result []VisitorStat
	for _, visitor := range visitors {
		result = append(result, *visitor)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Requests > result[j].Requests
	})
	return result
}

func sortReferrers(referrers map[string]*ReferrerStat) []ReferrerStat {
	var result []ReferrerStat
	for _, ref := range referrers {
		result = append(result, *ref)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})
	return result
}

func sortUserAgents(userAgents map[string]*UserAgentStat) ([]UserAgentStat, []UserAgentStat) {
	var regular []UserAgentStat
	var bots []UserAgentStat

	for _, ua := range userAgents {
		if ua.IsBot {
			bots = append(bots, *ua)
		} else {
			regular = append(regular, *ua)
		}
	}

	sort.Slice(regular, func(i, j int) bool {
		return regular[i].Count > regular[j].Count
	})

	sort.Slice(bots, func(i, j int) bool {
		return bots[i].Count > bots[j].Count
	})

	return regular, bots
}
