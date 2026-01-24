package analytics

import (
	"fmt"
	"sort"
	"strings"

	"fusionaly/internal/events"
	"fusionaly/internal/pkg/referrers"
	"fusionaly/internal/websites"

	"gorm.io/gorm"
)

// ReferrerMappings defines how to normalize referrer hostnames
var ReferrerMappings = map[string][]string{
	"Google": {
		"google.", "www.google.", "com.google.android.googlequicksearchbox", "com.google.android.gm",
		"news.google.com",
	},
	"Facebook": {
		"facebook.com", "fb.com", "m.facebook.com", "l.facebook.com", "com.facebook.katana", "com.facebook.Facebook",
	},
	"Twitter": {
		"twitter.com", "t.co", "com.twitter.android",
	},
	"LinkedIn": {
		"linkedin.com", "com.linkedin.android",
	},
	"YouTube": {
		"youtube.com", "youtu.be", "m.youtube.com", "com.google.android.youtube",
	},
	"Reddit": {
		"reddit.com", "com.reddit.frontpage", "com.reddit.Redditswe",
	},
	"Instagram": {
		"instagram.com", "com.instagram.android",
	},
	"Pinterest": {
		"pinterest.com",
	},
	"GitHub": {
		"github.com",
	},
	"Stack Overflow": {
		"stackoverflow.com",
	},
	"Medium": {
		"medium.com", "com.medium.reader",
	},
	"Bing": {
		"bing.com",
	},
	"DuckDuckGo": {
		"duckduckgo.com", "com.duckduckgo.mobile.android",
	},
	"Yahoo": {
		"yahoo.com",
	},
	"Amazon": {
		"amazon.", "www.amazon.",
	},
	"TikTok": {
		"tiktok.com", "com.zhiliaoapp.musically",
	},
	"Discord": {
		"discord.com", "discord.gg",
	},
	"WhatsApp": {
		"whatsapp.com", "com.whatsapp",
	},
}

// DirectReferrerKeywords defines patterns that should be treated as direct traffic
var DirectReferrerKeywords = []string{
	"", "direct / unknown", "(direct)", "unknown", events.DirectOrUnknownReferrer,
}

// NormalizeReferrerHostname cleans and normalizes a referrer hostname
func NormalizeReferrerHostname(hostname string) string {
	if hostname == "" {
		return "Direct / Unknown"
	}

	// Convert to lowercase for consistent matching
	lowerHostname := strings.ToLower(hostname)

	// Check for direct traffic patterns
	for _, keyword := range DirectReferrerKeywords {
		if lowerHostname == keyword {
			return "Direct / Unknown"
		}
	}

	// Check against known mappings
	for serviceName, patterns := range ReferrerMappings {
		for _, pattern := range patterns {
			if strings.Contains(lowerHostname, pattern) {
				return serviceName
			}
		}
	}

	// Clean up common prefixes for unknown hostnames
	cleaned := lowerHostname
	prefixes := []string{"www.", "m.", "mobile.", "l.", "amp."}
	for _, prefix := range prefixes {
		cleaned = strings.TrimPrefix(cleaned, prefix)
	}

	return cleaned
}

// GetTopReferrersInTimeFrame fetches top referrers from RefStat with proper normalization
func GetTopReferrersInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) ([]MetricCountResult, error) {
	// Get the website domain for self-referral filtering
	var website websites.Website
	if err := db.First(&website, params.WebsiteID).Error; err != nil {
		return nil, fmt.Errorf("failed to get website domain for self-referral filtering: %w", err)
	}

	// Simple query to get raw hostname data
	query := `
		SELECT hostname, SUM(visitors_count) as count
		FROM ref_stats
		WHERE hour BETWEEN ? AND ?
		AND website_id = ?
		GROUP BY hostname
		HAVING count > 0
		ORDER BY count DESC
	`

	type RawReferrerResult struct {
		Hostname string
		Count    int64
	}

	var rawResults []RawReferrerResult
	err := db.Raw(query,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		params.WebsiteID,
	).Scan(&rawResults).Error
	if err != nil {
		return nil, fmt.Errorf("error fetching raw referrer data: %w", err)
	}

	// Process results in Go - normalize and filter
	normalizedCounts := make(map[string]int64)

	for _, result := range rawResults {
		// Skip self-referrals
		if events.IsSelfReferral(result.Hostname, website.Domain) {
			continue
		}

		// Normalize the hostname
		normalized := NormalizeReferrerHostname(result.Hostname)
		normalizedCounts[normalized] += result.Count
	}

	// Convert to final results with friendly names and sort
	var results []MetricCountResult
	for hostname, count := range normalizedCounts {
		results = append(results, MetricCountResult{
			Name:  referrers.FriendlyName(hostname),
			Count: count,
		})
	}

	// Sort by count (descending) and limit
	sort.Slice(results, func(i, j int) bool {
		return results[i].Count > results[j].Count
	})

	if len(results) > params.Limit {
		results = results[:params.Limit]
	}

	return results, nil
}
