package analytics_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/testsupport"
)

// Every dashboard panel reads a summary table for one website and a date
// range. Without an index on (website_id, hour), SQLite reads all of the
// website's rows for any range.
func TestSummaryTablesReadADateRangeThroughAnIndex(t *testing.T) {
	db := testsupport.SetupTestDB(t)
	tables := []string{
		"site_stats", "page_stats", "ref_stats", "browser_stats", "os_stats", "device_stats",
		"country_stats", "utm_stats", "event_stats", "query_param_stats", "flow_transition_stats",
	}

	for _, table := range tables {
		t.Run(table, func(t *testing.T) {
			var plan []struct{ Detail string }

			err := db.Raw("EXPLAIN QUERY PLAN SELECT * FROM " + table +
				" WHERE website_id = 1 AND hour >= '2026-01-01' AND hour < '2026-01-08'").Scan(&plan).Error

			require.NoError(t, err)
			require.NotEmpty(t, plan)
			assert.Contains(t, plan[0].Detail, "(website_id=? AND hour>? AND hour<?)")
		})
	}
}
