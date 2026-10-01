package analytics_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"fusionaly/internal/analytics"
	"fusionaly/internal/events"
	"fusionaly/internal/timeframe"
)

func averageVisitDuration(t *testing.T, db *gorm.DB, from, to time.Time) float64 {
	t.Helper()
	params := analytics.WebsiteScopedQueryParams{WebsiteID: 1, TimeFrame: &timeframe.TimeFrame{From: from, To: to}}
	seconds, err := analytics.GetVisitDurationInTimeFrame(db, params)
	require.NoError(t, err)
	return seconds
}

func TestVisitDuration(t *testing.T) {
	endOfDay := at("24h").Add(-time.Nanosecond)

	t.Run("a two-page visit lasts from its first to its last page view", func(t *testing.T) {
		db := setupFlowDB(t,
			pageView("u1", "/", at("10h00m")),
			pageView("u1", "/pricing", at("10h02m")),
		)

		assert.InDelta(t, 120, averageVisitDuration(t, db, at("0h"), endOfDay), 0.01)
	})

	t.Run("a single-page visit lasts 0 seconds and counts in the average", func(t *testing.T) {
		db := setupFlowDB(t,
			pageView("u1", "/", at("10h00m")),
			pageView("u1", "/pricing", at("10h02m")),
			pageView("u2", "/", at("11h00m")),
		)

		assert.InDelta(t, 60, averageVisitDuration(t, db, at("0h"), endOfDay), 0.01)
	})

	t.Run("a visit longer than the session timeout keeps its full length", func(t *testing.T) {
		db := setupFlowDB(t,
			pageView("u1", "/a", at("10h00m")),
			pageView("u1", "/b", at("10h20m")),
			pageView("u1", "/c", at("10h40m")),
		)

		assert.InDelta(t, 2400, averageVisitDuration(t, db, at("0h"), endOfDay), 0.01)
	})

	t.Run("a custom event keeps the visit going", func(t *testing.T) {
		db := setupFlowDB(t,
			pageView("u1", "/", at("10h00m")),
			customEvent("u1", at("10h20m")),
			pageView("u1", "/pricing", at("10h40m")),
		)

		assert.InDelta(t, 2400, averageVisitDuration(t, db, at("0h"), endOfDay), 0.01)
	})

	t.Run("a visit counts in the range of its first page view", func(t *testing.T) {
		db := setupFlowDB(t,
			pageView("u1", "/", at("09h55m")),
			pageView("u1", "/pricing", at("10h05m")),
		)

		assert.InDelta(t, 600, averageVisitDuration(t, db, at("09h00m"), at("10h00m")), 0.01)
		assert.Zero(t, averageVisitDuration(t, db, at("10h00m"), endOfDay))
	})

	t.Run("a single-page visit lasts until the page is hidden", func(t *testing.T) {
		db := setupFlowDB(t,
			pageView("u1", "/", at("10h00m")),
			pageHide("u1", at("10h03m")),
		)

		assert.InDelta(t, 180, averageVisitDuration(t, db, at("0h"), endOfDay), 0.01)
	})

	t.Run("a page hide without a page view is no visit", func(t *testing.T) {
		db := setupFlowDB(t,
			pageView("u1", "/", at("10h00m")),
			pageHide("u1", at("10h03m")),
			pageHide("u2", at("11h00m")),
		)

		assert.InDelta(t, 180, averageVisitDuration(t, db, at("0h"), endOfDay), 0.01)
	})
}

func pageHide(user string, ts time.Time) events.Event {
	return events.Event{WebsiteID: 1, UserSignature: user, Hostname: "example.com", Pathname: "/", EventType: events.EventTypePageHide, Timestamp: ts}
}
