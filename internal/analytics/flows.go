package analytics

import (
	"fmt"
	"sort"

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

// GetUserFlowDataFromEvents computes page-to-page transitions from the events
// table, with the same rules as the stored flows (events.QueryFlowTransitions).
// It is the fallback when pre-aggregated data is not available.
func GetUserFlowDataFromEvents(db *gorm.DB, params WebsiteScopedQueryParams, maxDepth int) ([]UserFlowLink, error) {
	if maxDepth <= 0 {
		maxDepth = 5
	}

	transitions, err := events.QueryFlowTransitions(db, uint(params.WebsiteID), params.TimeFrame.From, params.TimeFrame.To, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("error fetching user flow data from events: %w", err)
	}

	// Sum each move across hours.
	totals := map[UserFlowLink]int64{}
	for _, t := range transitions {
		link := UserFlowLink{
			Source: fmt.Sprintf("step%d:%s", t.StepPosition, t.SourcePage),
			Target: fmt.Sprintf("step%d:%s", t.StepPosition+1, t.TargetPage),
		}
		totals[link] += int64(t.Transitions)
	}

	results := make([]UserFlowLink, 0, len(totals))
	for link, value := range totals {
		link.Value = value
		results = append(results, link)
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Value != results[j].Value {
			return results[i].Value > results[j].Value
		}
		return results[i].Source+results[i].Target < results[j].Source+results[j].Target
	})
	if len(results) > 200 {
		results = results[:200]
	}
	return results, nil
}
