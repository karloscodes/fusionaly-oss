package events_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/events"
	"fusionaly/internal/testsupport"
)

// Revenue reads the events with a price. idx_events_priced holds only those,
// so a revenue query must read it, not every event in the range.
func TestPriceQueriesReadThePricedEventsIndex(t *testing.T) {
	db := testsupport.SetupTestDB(t)
	var plan []struct{ Detail string }

	err := db.Raw(`EXPLAIN QUERY PLAN SELECT custom_event_meta FROM events
		WHERE website_id = 1 AND timestamp BETWEEN '2026-01-01' AND '2026-02-01'
		AND event_type = 2 AND ` + events.PricedEventSQL + `
		AND LOWER(custom_event_name) = 'revenue:purchased'`).Scan(&plan).Error

	require.NoError(t, err)
	require.NotEmpty(t, plan)
	assert.Contains(t, plan[0].Detail, "USING INDEX idx_events_priced")
}
