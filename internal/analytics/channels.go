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

// Channels group the source of each visit:
//
//   - Paid: utm_medium is cpc, ppc, display, cpm, ads, or starts with "paid".
//   - Email: utm_medium is email or newsletter.
//   - Direct: the visit has no source.
//   - Search, Social, AI, Email: the category of a known source (see
//     internal/pkg/referrers).
//   - Referral: any other source.
//
// The utm_medium rules apply only to visits with a utm_source. A visit with
// a utm_medium and no utm_source gets its channel from its source.
const (
	ChannelDirect = "Direct"
	ChannelPaid   = "Paid"
	ChannelEmail  = "Email"
)

var paidMediums = map[string]bool{"cpc": true, "ppc": true, "display": true, "cpm": true, "ads": true}
var emailMediums = map[string]bool{"email": true, "e-mail": true, "newsletter": true}

// ChannelOf returns the channel of a visit from its source and utm_medium.
func ChannelOf(source, medium string) string {
	if channel, ok := mediumChannel(medium); ok {
		return channel
	}
	return sourceChannel(source)
}

func mediumChannel(medium string) (string, bool) {
	medium = strings.ToLower(strings.TrimSpace(medium))
	if paidMediums[medium] || strings.HasPrefix(medium, "paid") {
		return ChannelPaid, true
	}
	if emailMediums[medium] {
		return ChannelEmail, true
	}
	return "", false
}

func sourceChannel(source string) string {
	if NormalizeReferrerHostname(source) == "Direct / Unknown" {
		return ChannelDirect
	}
	return string(referrers.CategoryOf(source))
}

// GetTopChannelsInTimeFrame counts visits per channel.
//
// ref_stats holds the visits per source, and for a tagged link the source is
// its utm_source. utm_stats holds the same tagged visits per utm_source and
// utm_medium, in the same buckets. So the tagged visits with a paid or email
// medium move from their source's channel to Paid or Email.
func GetTopChannelsInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) ([]MetricCountResult, error) {
	var website websites.Website
	if err := db.First(&website, params.WebsiteID).Error; err != nil {
		return nil, fmt.Errorf("failed to get website domain for self-referral filtering: %w", err)
	}

	var sources []struct {
		Hostname string
		Count    int64
	}
	err := db.Raw(`
		SELECT hostname, SUM(visitors_count) AS count
		FROM ref_stats
		WHERE hour BETWEEN ? AND ? AND website_id = ?
		GROUP BY hostname
		HAVING count > 0
	`, params.TimeFrame.From.UTC(), params.TimeFrame.To.UTC(), params.WebsiteID).Scan(&sources).Error
	if err != nil {
		return nil, fmt.Errorf("error fetching sources: %w", err)
	}

	var tagged []struct {
		UTMSource string
		UTMMedium string
		Count     int64
	}
	err = db.Raw(`
		SELECT utm_source, utm_medium, SUM(visitors_count) AS count
		FROM utm_stats
		WHERE hour BETWEEN ? AND ? AND website_id = ?
		AND utm_source != ? AND utm_medium != ?
		GROUP BY utm_source, utm_medium
		HAVING count > 0
	`, params.TimeFrame.From.UTC(), params.TimeFrame.To.UTC(), params.WebsiteID,
		events.EmptyUTMAttr, events.EmptyUTMAttr).Scan(&tagged).Error
	if err != nil {
		return nil, fmt.Errorf("error fetching utm mediums: %w", err)
	}

	visitsBySource := make(map[string]int64)
	for _, source := range sources {
		// Same rule as Top Referrers: a self-referral is not a source.
		if events.IsSelfReferral(source.Hostname, website.Domain) {
			continue
		}
		visitsBySource[source.Hostname] += source.Count
	}

	counts := make(map[string]int64)
	for _, row := range tagged {
		channel, ok := mediumChannel(row.UTMMedium)
		if !ok {
			continue
		}
		moved := min(row.Count, visitsBySource[row.UTMSource])
		counts[channel] += moved
		visitsBySource[row.UTMSource] -= moved
	}
	for source, visits := range visitsBySource {
		counts[sourceChannel(source)] += visits
	}

	results := []MetricCountResult{}
	for channel, count := range counts {
		if count > 0 {
			results = append(results, MetricCountResult{Name: channel, Count: count})
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Count != results[j].Count {
			return results[i].Count > results[j].Count
		}
		return results[i].Name < results[j].Name
	})
	return results, nil
}
