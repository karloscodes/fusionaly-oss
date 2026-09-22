package feed_test

import (
	"math"
	"testing"
	"time"

	"fusionaly/internal/feed"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

var siteVisitors = feed.Metric{Table: "site_stats", Column: "visitors"}

func insertVisitors(db *gorm.DB, day time.Time, visitors int) {
	db.Exec(`INSERT INTO site_stats (website_id, visitors, hour) VALUES (1, ?, ?)`, visitors, day.Add(12*time.Hour))
}

func yesterdayUTC() time.Time {
	return time.Now().UTC().AddDate(0, 0, -1).Truncate(24 * time.Hour)
}

func TestBaselineFor(t *testing.T) {
	t.Run("compares with the same weekday once the site has 3+ weeks", func(t *testing.T) {
		db := setupTestDB(t)
		yesterday := yesterdayUTC()
		weekdayValues := []int{900, 1000, 1100, 1000, 900, 1000, 1100, 1000}
		for w, v := range weekdayValues {
			for d := 0; d < 7; d++ {
				day := yesterday.AddDate(0, 0, -7*(w+1)+d)
				if d == 0 {
					insertVisitors(db, day, v) // same weekday as yesterday
				} else {
					insertVisitors(db, day, 100)
				}
			}
		}

		b := feed.BaselineFor(db, 1, siteVisitors, yesterday)

		assert.InDelta(t, 1000, b.Typical, 0.001)
		assert.InDelta(t, 50*1.4826, b.Spread, 0.001)
	})

	t.Run("leaves out weeks before the site's first day", func(t *testing.T) {
		db := setupTestDB(t)
		yesterday := yesterdayUTC()
		for i := 1; i <= 21; i++ { // exactly 3 weeks of history
			insertVisitors(db, yesterday.AddDate(0, 0, -i), 300)
		}

		b := feed.BaselineFor(db, 1, siteVisitors, yesterday)

		assert.InDelta(t, 300, b.Typical, 0.001, "zero weeks before launch must not drag the baseline down")
	})

	t.Run("ignores a viral day in the weekday history", func(t *testing.T) {
		db := setupTestDB(t)
		yesterday := yesterdayUTC()
		weekdayValues := []int{900, 1000, 1100, 5000, 900, 1000, 1100, 1000}
		for w, v := range weekdayValues {
			insertVisitors(db, yesterday.AddDate(0, 0, -7*(w+1)), v)
		}

		b := feed.BaselineFor(db, 1, siteVisitors, yesterday)

		assert.InDelta(t, 1000, b.Typical, 0.001)
		assert.True(t, b.IsSpike(1500), "a past viral day must not hide the next surge")
	})

	t.Run("ignores a viral day during cold start", func(t *testing.T) {
		db := setupTestDB(t)
		yesterday := yesterdayUTC()
		for i, v := range []int{100, 100, 1500, 100, 100, 100} {
			insertVisitors(db, yesterday.AddDate(0, 0, -(i+1)), v)
		}

		b := feed.BaselineFor(db, 1, siteVisitors, yesterday)

		assert.InDelta(t, 100, b.Typical, 0.001)
		assert.True(t, b.IsSpike(300))
	})

	t.Run("falls back to the last 7 days on a young site", func(t *testing.T) {
		db := setupTestDB(t)
		yesterday := yesterdayUTC()
		for i := 1; i <= 7; i++ {
			insertVisitors(db, yesterday.AddDate(0, 0, -i), 100)
		}

		b := feed.BaselineFor(db, 1, siteVisitors, yesterday)

		assert.InDelta(t, 100, b.Typical, 0.001)
		assert.InDelta(t, 100*feed.ColdStartVariance, b.Spread, 0.001)
	})

	t.Run("averages only the days since a brand-new site started", func(t *testing.T) {
		db := setupTestDB(t)
		yesterday := yesterdayUTC()
		insertVisitors(db, yesterday.AddDate(0, 0, -1), 100)
		insertVisitors(db, yesterday.AddDate(0, 0, -2), 100)

		b := feed.BaselineFor(db, 1, siteVisitors, yesterday)

		assert.InDelta(t, 100, b.Typical, 0.001)
	})

	t.Run("floors stddev at Poisson noise for a perfectly steady metric", func(t *testing.T) {
		db := setupTestDB(t)
		yesterday := yesterdayUTC()
		for i := 1; i <= 56; i++ {
			insertVisitors(db, yesterday.AddDate(0, 0, -i), 400)
		}

		b := feed.BaselineFor(db, 1, siteVisitors, yesterday)

		assert.InDelta(t, math.Sqrt(400), b.Spread, 0.001)
		assert.False(t, b.IsSpike(430), "a +30 wobble on 400/day is noise")
		assert.True(t, b.IsSpike(441))
	})

	t.Run("reports no history when the metric never happened", func(t *testing.T) {
		db := setupTestDB(t)
		yesterday := yesterdayUTC()
		insertVisitors(db, yesterday.AddDate(0, 0, -3), 50)
		signups := feed.Metric{Table: "event_stats", Column: "visitors_count", Filter: "event_name = ?", Args: []any{"signup"}}
		db.Exec(`INSERT INTO event_stats (website_id, event_name, visitors_count, hour) VALUES (1, 'purchase', 5, ?)`, yesterday.AddDate(0, 0, -3))

		b := feed.BaselineFor(db, 1, signups, yesterday)

		assert.Zero(t, b.Total, "other events must not count toward this goal")
	})
}
