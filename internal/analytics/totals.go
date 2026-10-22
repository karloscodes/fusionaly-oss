package analytics

import (
	"fmt"
	"time"

	"log/slog"
	"gorm.io/gorm"

	"fusionaly/internal/events"
)

// GetVisitDurationInTimeFrame returns the average visit duration in seconds,
// over the visits that start in the range. A visit lasts from its first to
// its last event, of any type; a single page view with no other event lasts
// 0 seconds. The event processor sums it in site_stats (see
// events.addVisitTime).
func GetVisitDurationInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) (float64, error) {
	var seconds float64
	err := db.Raw(`
		SELECT COALESCE(SUM(visit_seconds) / NULLIF(SUM(sessions), 0), 0)
		FROM site_stats
		WHERE website_id = ? AND hour BETWEEN ? AND ?
	`, params.WebsiteID, params.TimeFrame.From.UTC(), params.TimeFrame.To.UTC()).Scan(&seconds).Error
	if err != nil {
		return 0, fmt.Errorf("error calculating visit duration: %w", err)
	}
	return seconds, nil
}

// GetBounceRateInTimeFrame calculates the bounce rate using SiteStat
func GetBounceRateInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) (float64, error) {
	var result struct {
		BounceRate float64
	}

	query := `
        SELECT 
            CAST(SUM(bounce_count) AS FLOAT) / 
            CAST(SUM(sessions) AS FLOAT) as bounce_rate
        FROM site_stats
        WHERE hour BETWEEN ? AND ?
        AND website_id = ?
    `

	err := db.Raw(query,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		params.WebsiteID,
	).Scan(&result).Error
	if err != nil {
		return 0, fmt.Errorf("error calculating bounce rate from SiteStat: %w", err)
	}

	return result.BounceRate, nil
}

// GetTotalEvents returns the total number of events for the given website and time frame.
func GetTotalEvents(db *gorm.DB, params WebsiteScopedQueryParams, logger *slog.Logger) (int64, error) {
	logger.Debug("GetTotalEvents called",
		slog.Int("websiteID", params.WebsiteID),
		slog.Time("timeFrom", params.TimeFrame.From),
		slog.Time("timeTo", params.TimeFrame.To),
		slog.String("timeFrom_formatted", params.TimeFrame.From.Format(time.RFC3339)),
		slog.String("timeTo_formatted", params.TimeFrame.To.Format(time.RFC3339)))

	var count int64
	query := db.Model(&events.Event{}).
		Where("website_id = ?", params.WebsiteID).
		Where("event_type != ?", events.EventTypePageHide).
		Where("timestamp >= ?", params.TimeFrame.From).
		Where("timestamp <= ?", params.TimeFrame.To)

	err := query.Count(&count).Error
	if err != nil {
		return 0, err
	}

	logger.Debug("GetTotalEvents result", slog.Int64("count", count))
	return count, nil
}

// GetTotalEntryCountInTimeFrame calculates the total number of entrances in the time frame
func GetTotalEntryCountInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) (int64, error) {
	var result struct {
		TotalEntries int64
	}

	query := `
    SELECT COALESCE(SUM(entrances), 0) as total_entries
    FROM page_stats
    WHERE hour BETWEEN ? AND ?
    AND website_id = ?
    `

	err := db.Raw(query,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		params.WebsiteID,
	).Scan(&result).Error
	if err != nil {
		return 0, fmt.Errorf("error calculating total entry count: %w", err)
	}

	return result.TotalEntries, nil
}

// GetTotalExitCountInTimeFrame calculates the total number of exits in the time frame
func GetTotalExitCountInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) (int64, error) {
	var result struct {
		TotalExits int64
	}

	query := `
    SELECT COALESCE(SUM(exits), 0) as total_exits
    FROM page_stats
    WHERE hour BETWEEN ? AND ?
    AND website_id = ?
    `

	err := db.Raw(query,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		params.WebsiteID,
	).Scan(&result).Error
	if err != nil {
		return 0, fmt.Errorf("error calculating total exit count: %w", err)
	}

	return result.TotalExits, nil
}

// GetTotalPageViewsInTimeFrame calculates the total number of page views in the time frame
func GetTotalPageViewsInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) (int64, error) {
	var result struct {
		TotalPageViews int64
	}

	query := `
    SELECT COALESCE(SUM(page_views), 0) as total_page_views
    FROM site_stats
    WHERE hour BETWEEN ? AND ?
    AND website_id = ?
    `

	err := db.Raw(query,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		params.WebsiteID,
	).Scan(&result).Error
	if err != nil {
		return 0, fmt.Errorf("error calculating total page views: %w", err)
	}

	return result.TotalPageViews, nil
}

// GetTotalPageViews calculates the total number of page views across all time
func GetTotalPageViews(db *gorm.DB) (int64, error) {
	var result struct {
		TotalPageViews int64
	}

	query := `
    SELECT COALESCE(SUM(page_views), 0) as total_page_views
    FROM site_stats
    `

	err := db.Raw(query).Scan(&result).Error
	if err != nil {
		return 0, fmt.Errorf("error calculating total page views: %w", err)
	}

	return result.TotalPageViews, nil
}

// GetTotalSessionsInTimeFrame calculates the total number of sessions in the time frame
func GetTotalSessionsInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) (int64, error) {
	var result struct {
		TotalSessions int64
	}

	query := `
    SELECT COALESCE(SUM(sessions), 0) as total_sessions
    FROM site_stats
    WHERE hour BETWEEN ? AND ?
    AND website_id = ?
    `

	err := db.Raw(query,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		params.WebsiteID,
	).Scan(&result).Error
	if err != nil {
		return 0, fmt.Errorf("error calculating total sessions: %w", err)
	}

	return result.TotalSessions, nil
}

// GetTotalCustomEventsInTimeFrame calculates the total number of unique visitors triggering custom events
func GetTotalCustomEventsInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) (int64, error) {
	var result struct {
		TotalEvents int64
	}

	query := `
    SELECT COALESCE(SUM(visitors_count), 0) as total_events
    FROM event_stats
    WHERE hour BETWEEN ? AND ?
    AND website_id = ?
    `

	err := db.Raw(query,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		params.WebsiteID,
	).Scan(&result).Error
	if err != nil {
		return 0, fmt.Errorf("error calculating total custom event visitors from event_stats: %w", err)
	}

	return result.TotalEvents, nil
}
