package analytics

import (
	"fmt"
	"time"

	"log/slog"
	"gorm.io/gorm"

	"fusionaly/internal/config"
	"fusionaly/internal/events"
)

// GetVisitDurationInTimeFrame calculates the average visit duration
func GetVisitDurationInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) (float64, error) {
	sessionTimeoutSeconds := config.GetConfig().SessionTimeoutSeconds

	var result struct {
		AverageDuration float64
	}

	query := `
    WITH ranked_views AS (
        SELECT
            user_signature,
            timestamp,
            LAG(timestamp) OVER (
                PARTITION BY user_signature
                ORDER BY timestamp
            ) as prev_view_time
        FROM events
        WHERE timestamp BETWEEN ? AND ?
        AND event_type = ?
        AND website_id = ?
    ),
    session_breaks AS (
        SELECT
            user_signature,
            timestamp,
            CASE
                WHEN prev_view_time IS NULL OR 
                     CAST((JULIANDAY(timestamp) - JULIANDAY(prev_view_time)) * 86400 as INTEGER) > ?
                THEN 1
                ELSE 0
            END as is_new_session
        FROM ranked_views
    ),
    sessions AS (
        SELECT
            user_signature,
            timestamp,
            SUM(is_new_session) OVER (
                PARTITION BY user_signature
                ORDER BY timestamp
            ) as session_id
        FROM session_breaks
    ),
    session_durations AS (
        SELECT
            user_signature,
            session_id,
            MIN(timestamp) as session_start,
            MAX(timestamp) as session_end,
            COUNT(*) as event_count,
            CAST((JULIANDAY(MAX(timestamp)) - JULIANDAY(MIN(timestamp))) * 86400 as INTEGER) as duration_seconds
        FROM sessions
        GROUP BY user_signature, session_id
        HAVING event_count >= 3
        AND duration_seconds <= ?
    )
    SELECT COALESCE(AVG(duration_seconds), 0) as average_duration
    FROM session_durations
    WHERE duration_seconds > 0`

	err := db.Raw(query,
		params.TimeFrame.From.UTC(), params.TimeFrame.To.UTC(), events.EventTypePageView, params.WebsiteID,
		sessionTimeoutSeconds,
		sessionTimeoutSeconds,
	).Scan(&result).Error
	if err != nil {
		return 0, fmt.Errorf("error calculating visit duration: %w", err)
	}

	return result.AverageDuration, nil
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

// GetTotalVisitorsInTimeFrame calculates the total number of visitors in the time frame
func GetTotalVisitorsInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) (int64, error) {
	var result struct {
		TotalVisitors int64
	}

	query := `
    SELECT COALESCE(SUM(visitors), 0) as total_visitors
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
		return 0, fmt.Errorf("error calculating total visitors: %w", err)
	}

	return result.TotalVisitors, nil
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
