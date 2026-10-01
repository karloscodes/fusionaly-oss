package events

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"fusionaly/internal/config"
)

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

// VisitsCTE splits the events of a range into visits, with the rules of
// live processing (visitStatus):
//   - a visit ends after the session timeout without an event of any type;
//     the gap is compared in whole milliseconds, because JULIANDAY is a float
//   - it reads the visitors with a page view in [@from, @to], from the start
//     of @from's UTC day to the end of @to's: the signature rotates at
//     midnight UTC, so a visit never spans two UTC days and each visit is
//     read in full
//
// It defines visits(id, website_id, user_signature, event_type, timestamp,
// page, visit), where visit numbers a visitor's visits from 1. Parameters:
// @from, @to, @day_start, @day_end, @website_id (0 = all), @page_view,
// @timeout.
const VisitsCTE = `
visitor_events AS (
	SELECT id, website_id, user_signature, event_type, timestamp,
		hostname || pathname AS page,
		LAG(timestamp) OVER (
			PARTITION BY website_id, user_signature ORDER BY timestamp, id
		) AS previous_timestamp
	FROM events
	WHERE timestamp >= @day_start AND timestamp < @day_end
		AND (website_id, user_signature) IN (
			SELECT website_id, user_signature
			FROM events
			WHERE timestamp >= @from AND timestamp <= @to
				AND event_type = @page_view
				AND (@website_id = 0 OR website_id = @website_id)
		)
),
visits AS (
	SELECT id, website_id, user_signature, event_type, timestamp, page,
		SUM(CASE
			WHEN previous_timestamp IS NULL
				OR ROUND((JULIANDAY(timestamp) - JULIANDAY(previous_timestamp)) * 86400000) > @timeout * 1000
			THEN 1 ELSE 0
		END) OVER (
			PARTITION BY website_id, user_signature ORDER BY timestamp, id
		) AS visit
	FROM visitor_events
)`

// VisitsParams returns the parameters of VisitsCTE.
func VisitsParams(websiteID uint, from, to time.Time) map[string]any {
	from, to = from.UTC(), to.UTC()
	return map[string]any{
		"from":       from,
		"to":         to,
		"day_start":  from.Truncate(24 * time.Hour),
		"day_end":    to.Truncate(24 * time.Hour).Add(24 * time.Hour),
		"website_id": websiteID,
		"page_view":  EventTypePageView,
		"timeout":    config.GetConfig().SessionTimeoutSeconds,
	}
}

// flowTransitionsQuery finds the page-to-page moves whose target page view
// falls in [from, to). The flow rules, on top of VisitsCTE:
//   - views of the same page in a row are one step (a reload is no move)
//   - steps count from the visit's first page view, also before from
const flowTransitionsQuery = `
WITH ` + VisitsCTE + `,
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
	var rows []struct {
		WebsiteID    uint
		Hour         string
		StepPosition int
		SourcePage   string
		TargetPage   string
		Transitions  int
	}
	params := VisitsParams(websiteID, from, to)
	params["max_depth"] = maxDepth
	err := db.Raw(flowTransitionsQuery, params).Scan(&rows).Error
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
