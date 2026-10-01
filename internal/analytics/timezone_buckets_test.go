package analytics_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/analytics"
	"fusionaly/internal/events"
	"fusionaly/internal/testsupport"
	"fusionaly/internal/timeframe"
)

// Daily charts bucket by the user's local day, including across a daylight
// saving switch. Europe/Madrid moves from UTC+1 to UTC+2 at 2026-03-29 01:00 UTC.
func TestDailyBucketsFollowTheUsersTimezone(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	testsupport.CleanAllAggregates(db)
	madrid, err := time.LoadLocation("Europe/Madrid")
	require.NoError(t, err)
	hours := map[string]int{
		"2026-03-27T23:00:00Z": 1, // Mar 28 00:00 CET: the UTC date is Mar 27
		"2026-03-28T23:00:00Z": 2, // Mar 29 00:00 CET
		"2026-03-29T22:00:00Z": 4, // Mar 30 00:00 CEST: a fixed +1h offset would say Mar 29
		"2026-03-30T21:00:00Z": 8, // Mar 30 23:00 CEST
	}
	for utc, count := range hours {
		h, err := time.Parse(time.RFC3339, utc)
		require.NoError(t, err)
		require.NoError(t, db.Create(&analytics.SiteStat{WebsiteID: 1, Visitors: count, PageViews: count, Hour: h}).Error)
		for i := 0; i < count; i++ {
			require.NoError(t, db.Create(&events.Event{WebsiteID: 1, UserSignature: fmt.Sprintf("%s-%d", utc, i),
				Hostname: "example.com", Pathname: "/", EventType: events.EventTypePageView, Timestamp: h}).Error)
		}
	}
	tf, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
		FromTime:      time.Date(2026, 3, 28, 0, 0, 0, 0, madrid),
		ToTime:        time.Date(2026, 3, 30, 23, 59, 59, 999999999, madrid),
		TimeFrameSize: timeframe.DailyTimeFrame,
	}, madrid)
	require.NoError(t, err)
	params := analytics.WebsiteScopedQueryParams{TimeFrame: tf, WebsiteID: 1}
	expected := map[string]int{"2026-03-28": 1, "2026-03-29": 2, "2026-03-30": 12}

	t.Run("page views from site_stats", func(t *testing.T) {
		series, err := analytics.AggregatedPageViewsInTimeFrame(db, params)

		require.NoError(t, err)
		assert.Equal(t, expected, countsByDate(series, 10))
	})

	t.Run("visitors from events", func(t *testing.T) {
		series, err := analytics.AggregatedVisitorsInTimeFrame(db, params)

		require.NoError(t, err)
		assert.Equal(t, expected, countsByDate(series, 10))
	})
}
