package events

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"fusionaly/internal/config"
	"fusionaly/internal/settings"
)

// keyFlowTransitionsRebuilt marks that flow_transition_stats was rebuilt
// with visit-based flows. Versions before it cut a visit at each clock hour.
const keyFlowTransitionsRebuilt = "flow_transitions_rebuilt_v1"

// FlowTransition counts the visitors who moved from one page to the next at
// one step of their visit, in one hour.
type FlowTransition struct {
	WebsiteID    uint
	Hour         time.Time // the hour of the page view the visitors moved to
	StepPosition int       // the source page's position in the visit, from 1
	SourcePage   string    // hostname + pathname
	TargetPage   string    // hostname + pathname
	Transitions  int
}

// flowTransitionsQuery finds the page-to-page moves whose target page view
// falls in [from, to). The flow rules:
//   - a visit ends after the session timeout without an event of any type,
//     the same rule as live processing (visitStatus); the gap is compared in
//     whole milliseconds, because JULIANDAY is a float
//   - views of the same page in a row are one step (a reload is no move)
//   - steps count from the visit's first page view, also before from
//
// The signature rotates at midnight UTC, so a visit never spans two UTC days
// and the events from the start of from's UTC day give each visit in full.
const flowTransitionsQuery = `
WITH visitor_events AS (
	SELECT id, website_id, user_signature, event_type, timestamp,
		hostname || pathname AS page,
		LAG(timestamp) OVER (
			PARTITION BY website_id, user_signature ORDER BY timestamp, id
		) AS previous_timestamp
	FROM events
	WHERE timestamp >= @day_start AND timestamp < @to
		AND (website_id, user_signature) IN (
			SELECT website_id, user_signature
			FROM events
			WHERE timestamp >= @from AND timestamp < @to
				AND event_type = @page_view
				AND (@website_id = 0 OR website_id = @website_id)
		)
),
visits AS (
	SELECT *,
		SUM(CASE
			WHEN previous_timestamp IS NULL
				OR ROUND((JULIANDAY(timestamp) - JULIANDAY(previous_timestamp)) * 86400000) > @timeout * 1000
			THEN 1 ELSE 0
		END) OVER (
			PARTITION BY website_id, user_signature ORDER BY timestamp, id
		) AS visit
	FROM visitor_events
),
page_views AS (
	SELECT id, website_id, user_signature, visit, page, timestamp,
		LAG(page) OVER (
			PARTITION BY website_id, user_signature, visit ORDER BY timestamp, id
		) AS previous_page
	FROM visits
	WHERE event_type = @page_view
),
steps AS (
	SELECT website_id, page, timestamp,
		ROW_NUMBER() OVER visit_order AS step,
		LEAD(page) OVER visit_order AS next_page,
		LEAD(timestamp) OVER visit_order AS next_timestamp
	FROM page_views
	WHERE previous_page IS NULL OR previous_page != page
	WINDOW visit_order AS (PARTITION BY website_id, user_signature, visit ORDER BY timestamp, id)
)
SELECT
	website_id,
	strftime('%Y-%m-%d %H:00:00', next_timestamp) AS hour,
	step AS step_position,
	page AS source_page,
	next_page AS target_page,
	COUNT(*) AS transitions
FROM steps
WHERE next_page IS NOT NULL
	AND next_timestamp >= @from AND next_timestamp < @to
	AND step <= @max_depth
GROUP BY website_id, hour, step, page, next_page
`

// QueryFlowTransitions returns the page-to-page moves whose target page view
// falls in [from, to), for one website or, with websiteID 0, for all.
func QueryFlowTransitions(db *gorm.DB, websiteID uint, from, to time.Time, maxDepth int) ([]FlowTransition, error) {
	from, to = from.UTC(), to.UTC()
	dayStart := from.Truncate(24 * time.Hour)

	var rows []struct {
		WebsiteID    uint
		Hour         string
		StepPosition int
		SourcePage   string
		TargetPage   string
		Transitions  int
	}
	err := db.Raw(flowTransitionsQuery, map[string]any{
		"from":       from,
		"to":         to,
		"day_start":  dayStart,
		"website_id": websiteID,
		"page_view":  EventTypePageView,
		"timeout":    config.GetConfig().SessionTimeoutSeconds,
		"max_depth":  maxDepth,
	}).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query flow transitions: %w", err)
	}

	transitions := make([]FlowTransition, 0, len(rows))
	for _, r := range rows {
		hour, err := time.Parse(time.DateTime, r.Hour)
		if err != nil {
			return nil, fmt.Errorf("failed to parse flow hour %q: %w", r.Hour, err)
		}
		transitions = append(transitions, FlowTransition{
			WebsiteID:    r.WebsiteID,
			Hour:         hour,
			StepPosition: r.StepPosition,
			SourcePage:   r.SourcePage,
			TargetPage:   r.TargetPage,
			Transitions:  r.Transitions,
		})
	}
	return transitions, nil
}

// ComputeFlowTransitionsForHour replaces the hour's rows in
// flow_transition_stats with the moves made in that hour.
func ComputeFlowTransitionsForHour(db *gorm.DB, logger *slog.Logger, hour time.Time, maxDepth int) error {
	hourStart := hour.UTC().Truncate(time.Hour)
	transitions, err := QueryFlowTransitions(db, 0, hourStart, hourStart.Add(time.Hour), maxDepth)
	if err != nil {
		return err
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM flow_transition_stats WHERE hour = ?", hourStart).Error; err != nil {
			return fmt.Errorf("failed to clear flow transitions: %w", err)
		}
		return insertFlowTransitions(tx, transitions)
	})
	if err != nil {
		return err
	}

	logger.Debug("Aggregated flow transitions for hour",
		slog.Time("hour", hourStart),
		slog.Int("transitions_count", len(transitions)))
	return nil
}

// ComputeFlowTransitionsForEvents recomputes the hours of the page views
// just processed. A new page view is always its visit's latest (see
// visitStatus), so it only adds a move in its own hour. The event processor
// calls it after each run, so a backlog from earlier hours is stored too.
func ComputeFlowTransitionsForEvents(db *gorm.DB, logger *slog.Logger, processed []*Event, maxDepth int) {
	hours := map[time.Time]bool{}
	for _, e := range processed {
		if e.EventType == EventTypePageView {
			hours[e.Timestamp.UTC().Truncate(time.Hour)] = true
		}
	}

	for hour := range hours {
		if err := ComputeFlowTransitionsForHour(db, logger, hour, maxDepth); err != nil {
			logger.Warn("Failed to compute flow transitions for hour",
				slog.Time("hour", hour),
				slog.Any("error", err))
		}
	}
}

// RebuildFlowTransitionsOnce rebuilds flow_transition_stats from the events
// table, one time per install, one UTC day at a time.
func RebuildFlowTransitionsOnce(db *gorm.DB, logger *slog.Logger, maxDepth int) error {
	if done, _ := settings.GetSetting(db, keyFlowTransitionsRebuilt); done != "" {
		return nil
	}

	var first Event
	err := db.Order("timestamp").Limit(1).Find(&first).Error
	if err != nil {
		return fmt.Errorf("failed to find the first event: %w", err)
	}

	started := time.Now()
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM flow_transition_stats").Error; err != nil {
			return fmt.Errorf("failed to clear flow transitions: %w", err)
		}
		if first.ID == 0 {
			return nil
		}
		end := time.Now().UTC()
		for day := first.Timestamp.UTC().Truncate(24 * time.Hour); day.Before(end); day = day.Add(24 * time.Hour) {
			transitions, err := QueryFlowTransitions(tx, 0, day, day.Add(24*time.Hour), maxDepth)
			if err != nil {
				return err
			}
			if err := insertFlowTransitions(tx, transitions); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	logger.Info("Rebuilt flow transitions", slog.Duration("took", time.Since(started)))
	return settings.CreateOrUpdateSetting(db, keyFlowTransitionsRebuilt, time.Now().UTC().Format(time.RFC3339))
}

func insertFlowTransitions(tx *gorm.DB, transitions []FlowTransition) error {
	now := time.Now().UTC()
	for _, t := range transitions {
		err := tx.Exec(`
			INSERT INTO flow_transition_stats (website_id, step_position, source_page, target_page, hour, transitions, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			t.WebsiteID, t.StepPosition, t.SourcePage, t.TargetPage, t.Hour, t.Transitions, now, now,
		).Error
		if err != nil {
			return fmt.Errorf("failed to store flow transition: %w", err)
		}
	}
	return nil
}
