package analytics

import (
	"time"

	"fusionaly/internal/timeframe"
)

// WebsiteScopedQueryParams contains common parameters for website-scoped queries
type WebsiteScopedQueryParams struct {
	TimeFrame *timeframe.TimeFrame
	WebsiteID int
	Limit     int               // Number of records to return
	Filters   map[string]string // Dynamic filters (e.g., {"country": "US", "browser": "Chrome"})
}

// NewWebsiteScopedQueryParams creates a new query params object with the specified time frame and website ID
func NewWebsiteScopedQueryParams(timeFrame *timeframe.TimeFrame, websiteID int) WebsiteScopedQueryParams {
	// Ensure timeFrame is not nil to prevent panics
	if timeFrame == nil {
		// Use a default time frame of last 7 days if none provided
		now := time.Now().UTC()
		defaultTimeFrame := &timeframe.TimeFrame{
			From:       now.AddDate(0, 0, -7),
			To:         now,
			BucketSize: timeframe.TimeFrameBucketSizeDay,
		}
		return WebsiteScopedQueryParams{
			TimeFrame: defaultTimeFrame,
			WebsiteID: websiteID,
			Limit:     50, // Default limit
			Filters:   make(map[string]string),
		}
	}

	return WebsiteScopedQueryParams{
		TimeFrame: timeFrame,
		WebsiteID: websiteID,
		Limit:     50, // Default limit
		Filters:   make(map[string]string),
	}
}
