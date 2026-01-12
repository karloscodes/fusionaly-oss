package events

import (
	"fmt"
	"time"

	"log/slog"
	"gorm.io/gorm"

	"fusionaly/internal/config"
)

const eventsTableName = "events"

// getVisitorIncrement returns the increment value for visitors_count based on isNewVisitor.
func getVisitorIncrement(isNewVisitor bool) int {
	if isNewVisitor {
		return 1
	}
	return 0
}

// truncateToHalfHour truncates a timestamp to the nearest half-hour boundary (00 or 30 minutes),
// preserving the original timezone offset.
func truncateToHalfHour(timestamp time.Time) time.Time {
	_, offsetSeconds := timestamp.Zone()
	year, month, day, hour, minute := timestamp.Year(), timestamp.Month(), timestamp.Day(), timestamp.Hour(), timestamp.Minute()
	location := time.FixedZone("", offsetSeconds)
	if minute < 30 {
		return time.Date(year, month, day, hour, 0, 0, 0, location)
	}
	return time.Date(year, month, day, hour, 30, 0, 0, location)
}

// UpdateAllAggregatesBatch updates aggregates from processed events.
func UpdateAllAggregatesBatch(tx *gorm.DB, logger *slog.Logger, dataList []*EventProcessingData) error {
	sessionTimeout := config.GetConfig().SessionTimeoutSeconds
	for _, data := range dataList {
		// Bounce detection: Check if this is a single-page session within sessionTimeout
		isBounce := false
		if data.EventType == EventTypePageView && data.IsNewSession {
			// For test data, we can use IsBounce directly from the event processing data if it's set
			if data.IsBounce {
				isBounce = true
			} else {
				var sessionPageViews int64
				err := tx.Table(eventsTableName).
					Where("website_id = ? AND user_signature = ? AND event_type = ? AND timestamp >= ? AND timestamp <= ?",
						data.WebsiteID, data.UserSignature, EventTypePageView,
						data.Timestamp, data.Timestamp.Add(time.Duration(sessionTimeout)*time.Second)).
					Count(&sessionPageViews).Error
				if err != nil {
					logger.Warn("Failed to count session page views for bounce", slog.Any("error", err))
				} else {
					isBounce = sessionPageViews == 1
				}
			}
		}

		// Truncate timestamp to half-hour bucket for finer granularity
		hourTime := truncateToHalfHour(data.Timestamp)

		// Only update site stats for page views
		if data.EventType == EventTypePageView {
			if err := updateSiteStatForPageView(tx, data.WebsiteID, hourTime, data.IsNewVisitor, data.IsNewSession, isBounce); err != nil {
				return fmt.Errorf("failed to update site stats: %w", err)
			}
			if err := updatePageStat(tx, data.WebsiteID, data.Hostname, data.Pathname, hourTime, data.IsEntrance, data.IsExit, data.UserSignature, data.IsNewVisitor); err != nil {
				return fmt.Errorf("failed to update page stats: %w", err)
			}
			if err := updateRefStat(tx, data.WebsiteID, data.ReferrerHostname, data.ReferrerPathname, hourTime, data.IsNewVisitor); err != nil {
				return fmt.Errorf("failed to update ref stats: %w", err)
			}
			if err := updateDeviceStat(tx, data.WebsiteID, data.DeviceType, hourTime, data.IsNewVisitor); err != nil {
				return fmt.Errorf("failed to update device stats: %w", err)
			}
			if err := updateBrowserStat(tx, data.WebsiteID, data.Browser, hourTime, data.IsNewVisitor); err != nil {
				return fmt.Errorf("failed to update browser stats: %w", err)
			}
			if err := updateOSStat(tx, data.WebsiteID, data.OperatingSystem, hourTime, data.IsNewVisitor); err != nil {
				return fmt.Errorf("failed to update os stats: %w", err)
			}
			if err := updateCountryStat(tx, data.WebsiteID, data.Country, hourTime, data.IsNewVisitor); err != nil {
				return fmt.Errorf("failed to update country stats: %w", err)
			}
			if data.HasUTM {
				if err := updateUTMStat(tx, data.WebsiteID, data.UTMSource, data.UTMMedium, data.UTMCampaign, data.UTMTerm, data.UTMContent, hourTime, data.IsNewVisitor); err != nil {
					return fmt.Errorf("failed to update utm stats: %w", err)
				}
			}
			// Track ALL query parameters
			for paramName, paramValue := range data.QueryParams {
				if paramValue != "" {
					if err := updateQueryParamStat(tx, data.WebsiteID, paramName, paramValue, hourTime, data.IsNewVisitor); err != nil {
						return fmt.Errorf("failed to update query param stats for %s: %w", paramName, err)
					}
				}
			}
		}

		// Always process custom events regardless of event type
		if data.EventType == EventTypeCustomEvent && data.CustomEventName != "" {
			// Use event-specific IsNewVisitor for custom events
			if err := updateEventStat(tx, data.WebsiteID, data.CustomEventName, data.CustomEventKey, hourTime, data.IsNewVisitor); err != nil {
				return fmt.Errorf("failed to update event stats: %w", err)
			}
		}
	}

	logger.Info("Updated aggregates", slog.Int("count", len(dataList)))
	return nil
}

// Incremental update functions

func updateSiteStatForPageView(tx *gorm.DB, websiteID uint, hour time.Time, isNewVisitor, isNewSession, isBounce bool) error {
	visitorInc := getVisitorIncrement(isNewVisitor)
	sessionInc := 0
	if isNewSession {
		sessionInc = 1
	}
	bounceInc := 0
	if isBounce {
		bounceInc = 1
	}
	now := time.Now().UTC()
	query := `
		INSERT INTO site_stats (website_id, hour, page_views, visitors, sessions, bounce_count, created_at, updated_at)
		VALUES (?, ?, 1, ?, ?, ?, ?, ?)
		ON CONFLICT (website_id, hour) DO UPDATE SET
			page_views = site_stats.page_views + 1,
			visitors = site_stats.visitors + ?,
			sessions = site_stats.sessions + ?,
			bounce_count = site_stats.bounce_count + ?,
			updated_at = ?
	`
	return tx.Exec(query, websiteID, hour, visitorInc, sessionInc, bounceInc, now, now, visitorInc, sessionInc, bounceInc, now).Error
}

func updatePageStat(tx *gorm.DB, websiteID uint, hostname, pathname string, hour time.Time, isEntrance, isExit bool, userSignature string, isNewVisitor bool) error {
	visitorInc := getVisitorIncrement(isNewVisitor)
	now := time.Now().UTC()
	query := `
		INSERT INTO page_stats (website_id, hostname, pathname, hour, page_views_count, visitors_count, entrances, exits, created_at, updated_at)
		VALUES (?, ?, ?, ?, 1, ?, ?, ?, ?, ?)
		ON CONFLICT (website_id, hostname, pathname, hour) DO UPDATE SET
			page_views_count = page_stats.page_views_count + 1,
			visitors_count = page_stats.visitors_count + ?,
			entrances = page_stats.entrances + ?,
			exits = page_stats.exits + ?,
			updated_at = ?
	`
	return tx.Exec(query,
		websiteID, hostname, pathname, hour,
		visitorInc, isEntrance, isExit, now, now,
		visitorInc, isEntrance, isExit, now).Error
}

func updateRefStat(tx *gorm.DB, websiteID uint, hostname, pathname string, hour time.Time, isNewVisitor bool) error {
	visitorInc := getVisitorIncrement(isNewVisitor)
	now := time.Now().UTC()
	query := `
		INSERT INTO ref_stats (website_id, hostname, pathname, hour, visitors_count, page_views_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 1, ?, ?)
		ON CONFLICT (website_id, hostname, pathname, hour) DO UPDATE SET
			visitors_count = ref_stats.visitors_count + ?,
			page_views_count = ref_stats.page_views_count + 1,
			updated_at = ?
	`
	return tx.Exec(query, websiteID, hostname, pathname, hour, visitorInc, now, now, visitorInc, now).Error
}

func updateDeviceStat(tx *gorm.DB, websiteID uint, deviceType string, hour time.Time, isNewVisitor bool) error {
	visitorInc := getVisitorIncrement(isNewVisitor)
	now := time.Now().UTC()
	query := `
		INSERT INTO device_stats (website_id, device_type, hour, visitors_count, page_views_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, 1, ?, ?)
		ON CONFLICT (website_id, device_type, hour) DO UPDATE SET
			visitors_count = device_stats.visitors_count + ?,
			page_views_count = device_stats.page_views_count + 1,
			updated_at = ?
	`
	return tx.Exec(query, websiteID, deviceType, hour, visitorInc, now, now, visitorInc, now).Error
}

func updateBrowserStat(tx *gorm.DB, websiteID uint, browser string, hour time.Time, isNewVisitor bool) error {
	visitorInc := getVisitorIncrement(isNewVisitor)
	now := time.Now().UTC()
	query := `
		INSERT INTO browser_stats (website_id, browser, hour, visitors_count, page_views_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, 1, ?, ?)
		ON CONFLICT (website_id, browser, hour) DO UPDATE SET
			visitors_count = browser_stats.visitors_count + ?,
			page_views_count = browser_stats.page_views_count + 1,
			updated_at = ?
	`
	return tx.Exec(query, websiteID, browser, hour, visitorInc, now, now, visitorInc, now).Error
}

func updateOSStat(tx *gorm.DB, websiteID uint, os string, hour time.Time, isNewVisitor bool) error {
	visitorInc := getVisitorIncrement(isNewVisitor)
	now := time.Now().UTC()
	query := `
		INSERT INTO os_stats (website_id, operating_system, hour, visitors_count, page_views_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, 1, ?, ?)
		ON CONFLICT (website_id, operating_system, hour) DO UPDATE SET
			visitors_count = os_stats.visitors_count + ?,
			page_views_count = os_stats.page_views_count + 1,
			updated_at = ?
	`
	return tx.Exec(query, websiteID, os, hour, visitorInc, now, now, visitorInc, now).Error
}

func updateCountryStat(tx *gorm.DB, websiteID uint, country string, hour time.Time, isNewVisitor bool) error {
	visitorInc := getVisitorIncrement(isNewVisitor)
	now := time.Now().UTC()
	query := `
		INSERT INTO country_stats (website_id, country, hour, visitors_count, page_views_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, 1, ?, ?)
		ON CONFLICT (website_id, country, hour) DO UPDATE SET
			visitors_count = country_stats.visitors_count + ?,
			page_views_count = country_stats.page_views_count + 1,
			updated_at = ?
	`
	return tx.Exec(query, websiteID, country, hour, visitorInc, now, now, visitorInc, now).Error
}

func updateUTMStat(tx *gorm.DB, websiteID uint, source, medium, campaign, term, content string, hour time.Time, isNewVisitor bool) error {
	visitorInc := getVisitorIncrement(isNewVisitor)
	now := time.Now().UTC()
	query := `
		INSERT INTO utm_stats (website_id, utm_source, utm_medium, utm_campaign, utm_term, utm_content, hour, visitors_count, page_views_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)
		ON CONFLICT (website_id, utm_source, utm_medium, utm_campaign, utm_term, utm_content, hour) DO UPDATE SET
			visitors_count = utm_stats.visitors_count + ?,
			page_views_count = utm_stats.page_views_count + 1,
			updated_at = ?
	`
	return tx.Exec(query, websiteID, source, medium, campaign, term, content, hour, visitorInc, now, now, visitorInc, now).Error
}

func updateEventStat(tx *gorm.DB, websiteID uint, eventName, eventKey string, hour time.Time, isNewVisitor bool) error {
	visitorInc := getVisitorIncrement(isNewVisitor)
	now := time.Now().UTC()
	query := `
		INSERT INTO event_stats (website_id, event_name, event_key, hour, visitors_count, page_views_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 1, ?, ?)
		ON CONFLICT (website_id, event_name, event_key, hour) DO UPDATE SET
			visitors_count = event_stats.visitors_count + ?,
			page_views_count = event_stats.page_views_count + 1,
			updated_at = ?
	`
	return tx.Exec(query, websiteID, eventName, eventKey, hour, visitorInc, now, now, visitorInc, now).Error
}

func updateQueryParamStat(tx *gorm.DB, websiteID uint, paramName, paramValue string, hour time.Time, isNewVisitor bool) error {
	visitorInc := getVisitorIncrement(isNewVisitor)
	now := time.Now().UTC()
	query := `
		INSERT INTO query_param_stats (website_id, param_name, param_value, hour, visitors_count, page_views_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 1, ?, ?)
		ON CONFLICT (website_id, param_name, param_value, hour) DO UPDATE SET
			visitors_count = query_param_stats.visitors_count + ?,
			page_views_count = query_param_stats.page_views_count + 1,
			updated_at = ?
	`
	return tx.Exec(query, websiteID, paramName, paramValue, hour, visitorInc, now, now, visitorInc, now).Error
}

// FlowTransitionResult holds a single flow transition from the computation query
type FlowTransitionResult struct {
	WebsiteID    uint
	StepPosition int
	SourcePage   string
	TargetPage   string
	Hour         time.Time
	Transitions  int
}

// ComputeFlowTransitionsForHour computes page-to-page transitions for a specific hour
// and stores them in flow_transition_stats. This replaces on-the-fly computation.
// maxDepth controls how many steps into the session to track (default 5)
func ComputeFlowTransitionsForHour(db *gorm.DB, logger *slog.Logger, hour time.Time, maxDepth int) error {
	if maxDepth <= 0 {
		maxDepth = 5
	}

	hourStart := hour.Truncate(time.Hour)
	hourEnd := hourStart.Add(time.Hour)

	// Query to compute all transitions within the hour
	// Groups by website, step position, source page, target page
	query := `
	WITH session_windows AS (
		SELECT
			website_id,
			user_signature,
			hostname || pathname AS page,
			timestamp,
			strftime('%Y-%m-%d %H', timestamp) AS session_window
		FROM events
		WHERE
			timestamp >= ? AND timestamp < ?
			AND event_type = ?
	),
	ranked_events AS (
		SELECT
			website_id,
			user_signature,
			page,
			timestamp,
			session_window,
			ROW_NUMBER() OVER (
				PARTITION BY website_id, user_signature, session_window
				ORDER BY timestamp
			) AS page_position,
			LEAD(page) OVER (
				PARTITION BY website_id, user_signature, session_window
				ORDER BY timestamp
			) AS next_page
		FROM session_windows
	)
	SELECT
		website_id,
		page_position AS step_position,
		page AS source_page,
		next_page AS target_page,
		COUNT(*) AS transitions
	FROM ranked_events
	WHERE next_page IS NOT NULL
		AND page != next_page
		AND page_position <= ?
	GROUP BY website_id, page_position, page, next_page
	HAVING transitions > 0
	`

	var results []FlowTransitionResult
	err := db.Raw(query, hourStart, hourEnd, EventTypePageView, maxDepth).Scan(&results).Error
	if err != nil {
		return fmt.Errorf("failed to compute flow transitions: %w", err)
	}

	if len(results) == 0 {
		logger.Debug("No flow transitions to aggregate for hour", slog.Time("hour", hourStart))
		return nil
	}

	// Batch insert/update the results
	now := time.Now().UTC()
	for _, r := range results {
		upsertQuery := `
			INSERT INTO flow_transition_stats (website_id, step_position, source_page, target_page, hour, transitions, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (website_id, step_position, source_page, target_page, hour) DO UPDATE SET
				transitions = ?,
				updated_at = ?
		`
		if err := db.Exec(upsertQuery, r.WebsiteID, r.StepPosition, r.SourcePage, r.TargetPage, hourStart, r.Transitions, now, now, r.Transitions, now).Error; err != nil {
			logger.Warn("Failed to upsert flow transition", slog.Any("error", err))
		}
	}

	logger.Info("Aggregated flow transitions for hour",
		slog.Time("hour", hourStart),
		slog.Int("transitions_count", len(results)))

	return nil
}

// ComputeFlowTransitionsForRecentHours computes flow transitions for the last N hours
// This should be called periodically by the background job
func ComputeFlowTransitionsForRecentHours(db *gorm.DB, logger *slog.Logger, hoursBack int, maxDepth int) error {
	now := time.Now().UTC().Truncate(time.Hour)

	for i := 0; i < hoursBack; i++ {
		hour := now.Add(-time.Duration(i) * time.Hour)
		if err := ComputeFlowTransitionsForHour(db, logger, hour, maxDepth); err != nil {
			logger.Warn("Failed to compute flow transitions for hour",
				slog.Time("hour", hour),
				slog.Any("error", err))
			// Continue with other hours
		}
	}

	return nil
}

// GetDistinctCustomEventNames retrieves unique custom event names from event_stats for a website within the last 90 days.
func GetDistinctCustomEventNames(db *gorm.DB, websiteID uint) ([]string, error) {
	var eventNames []string

	// Look back 90 days for relevant events
	timeLimit := time.Now().UTC().AddDate(0, 0, -90)

	query := db.Table("event_stats").
		Where("website_id = ? AND event_name != '' AND hour >= ?", websiteID, timeLimit).
		Distinct("event_name").
		Order("event_name ASC").
		Pluck("event_name", &eventNames)

	if query.Error != nil {
		return nil, fmt.Errorf("failed to get distinct custom event names: %w", query.Error)
	}

	return eventNames, nil
}

// GetEventCountForWebsite returns the count of events for a website within the specified number of days
func GetEventCountForWebsite(db *gorm.DB, websiteID uint, daysBack int) (int64, error) {
	var count int64
	timeLimit := time.Now().UTC().AddDate(0, 0, -daysBack)
	err := db.Model(&Event{}).
		Where("website_id = ? AND timestamp >= ?", websiteID, timeLimit).
		Count(&count).Error
	return count, err
}

// EventNameInfo holds event name and associated website info
type EventNameInfo struct {
	EventName string `json:"event_name"`
	WebsiteID uint   `json:"website_id"`
	Domain    string `json:"domain"`
}

// GetDistinctEventNamesForWebsite returns distinct custom event names with website info for a specific website
func GetDistinctEventNamesForWebsite(db *gorm.DB, websiteID uint, daysBack int) ([]EventNameInfo, error) {
	var results []EventNameInfo
	timeLimit := time.Now().UTC().AddDate(0, 0, -daysBack)

	err := db.Table("events e").
		Select("e.custom_event_name as event_name, e.website_id, w.domain").
		Joins("JOIN websites w ON w.id = e.website_id").
		Where("e.website_id = ? AND e.custom_event_name != '' AND e.timestamp >= ?", websiteID, timeLimit).
		Group("e.custom_event_name, e.website_id, w.domain").
		Order("e.custom_event_name ASC").
		Scan(&results).Error

	if err != nil {
		return []EventNameInfo{}, fmt.Errorf("failed to get distinct events for website: %w", err)
	}

	return results, nil
}

// GetDistinctEventNamesAllWebsites returns distinct custom event names with website info across all websites
func GetDistinctEventNamesAllWebsites(db *gorm.DB, daysBack int) ([]EventNameInfo, error) {
	var results []EventNameInfo
	timeLimit := time.Now().UTC().AddDate(0, 0, -daysBack)

	err := db.Table("events e").
		Select("e.custom_event_name as event_name, e.website_id, w.domain").
		Joins("JOIN websites w ON w.id = e.website_id").
		Where("e.custom_event_name != '' AND e.timestamp >= ?", timeLimit).
		Group("e.custom_event_name, e.website_id, w.domain").
		Order("w.domain ASC, e.custom_event_name ASC").
		Scan(&results).Error

	if err != nil {
		return []EventNameInfo{}, fmt.Errorf("failed to get distinct events for all websites: %w", err)
	}

	return results, nil
}
