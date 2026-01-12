package analytics

import (
	"fmt"

	"fusionaly/internal/events"

	"gorm.io/gorm"
)

// UserFlowLink represents a connection between two pages in the user flow
type UserFlowLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Value  int64  `json:"value"`
}

// GetUserFlowData retrieves page-to-page transitions from pre-aggregated flow_transition_stats
// Returns links showing direct page-to-page transitions:
// - First column: Entry pages (first page visited in session)
// - Middle columns: Pages visited during navigation
// - Transitions show how visitors move between pages
// Pages are prefixed with their step number to create proper flow columns
func GetUserFlowData(db *gorm.DB, params WebsiteScopedQueryParams, maxDepth int) ([]UserFlowLink, error) {
	if maxDepth <= 0 {
		maxDepth = 5 // Default to showing 5 levels of depth
	}

	var results []UserFlowLink

	// Query from pre-aggregated flow_transition_stats table
	// Sum transitions across hours within the time range
	query := `
	SELECT
		'step' || step_position || ':' || source_page AS source,
		'step' || (step_position + 1) || ':' || target_page AS target,
		SUM(transitions) AS value
	FROM flow_transition_stats
	WHERE
		website_id = ?
		AND hour >= ? AND hour < ?
		AND step_position <= ?
	GROUP BY step_position, source_page, target_page
	HAVING value > 0
	ORDER BY value DESC
	LIMIT 200
	`

	err := db.Raw(query,
		params.WebsiteID,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		maxDepth,
	).Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("error fetching user flow data: %w", err)
	}

	// If no pre-aggregated data, fall back to raw events query
	if len(results) == 0 {
		return GetUserFlowDataFromEvents(db, params, maxDepth)
	}

	return results, nil
}

// GetUserFlowDataFromEvents calculates page-to-page transitions directly from events table
// This is used as a fallback when pre-aggregated data is not available
func GetUserFlowDataFromEvents(db *gorm.DB, params WebsiteScopedQueryParams, maxDepth int) ([]UserFlowLink, error) {
	if maxDepth <= 0 {
		maxDepth = 5
	}

	var results []UserFlowLink

	query := `
	WITH session_windows AS (
		SELECT
			user_signature,
			hostname || pathname AS page,
			timestamp,
			strftime('%Y-%m-%d %H', timestamp,
				CASE
					WHEN strftime('%M', timestamp) < '30' THEN '-0 hours'
					ELSE '-0 hours'
				END
			) AS session_window
		FROM events
		WHERE
			timestamp BETWEEN ? AND ?
			AND website_id = ?
			AND event_type = ?
	),
	ranked_events AS (
		SELECT
			user_signature,
			page,
			timestamp,
			session_window,
			ROW_NUMBER() OVER (
				PARTITION BY user_signature, session_window
				ORDER BY timestamp
			) AS page_position,
			LEAD(page) OVER (
				PARTITION BY user_signature, session_window
				ORDER BY timestamp
			) AS next_page
		FROM session_windows
	),
	page_transitions AS (
		SELECT
			'step' || page_position || ':' || page AS source,
			'step' || (page_position + 1) || ':' || next_page AS target,
			COUNT(*) AS value
		FROM ranked_events
		WHERE next_page IS NOT NULL
			AND page != next_page
			AND page_position <= ?
		GROUP BY page_position, page, next_page
		HAVING value > 0
	)
	SELECT source, target, value FROM page_transitions
	ORDER BY value DESC
	LIMIT 200
	`

	err := db.Raw(query,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		params.WebsiteID,
		events.EventTypePageView,
		maxDepth,
	).Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("error fetching user flow data from events: %w", err)
	}

	return results, nil
}
