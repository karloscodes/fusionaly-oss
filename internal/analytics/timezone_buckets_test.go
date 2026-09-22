package analytics_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/analytics"
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
	hour := func(utc string, visitors int) analytics.SiteStat {
		h, err := time.Parse(time.RFC3339, utc)
		require.NoError(t, err)
		return analytics.SiteStat{WebsiteID: 1, Visitors: visitors, PageViews: visitors, Hour: h}
	}
	require.NoError(t, db.Create(&[]analytics.SiteStat{
		hour("2026-03-27T23:00:00Z", 1), // Mar 28 00:00 CET: the UTC date is Mar 27
		hour("2026-03-28T23:00:00Z", 2), // Mar 29 00:00 CET
		hour("2026-03-29T22:00:00Z", 4), // Mar 30 00:00 CEST: a fixed +1h offset would say Mar 29
		hour("2026-03-30T21:00:00Z", 8), // Mar 30 23:00 CEST
	}).Error)
	tf, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
		FromTime:      time.Date(2026, 3, 28, 0, 0, 0, 0, madrid),
		ToTime:        time.Date(2026, 3, 30, 23, 59, 59, 999999999, madrid),
		TimeFrameSize: timeframe.DailyTimeFrame,
	}, madrid)
	require.NoError(t, err)

	series, err := analytics.AggregatedVisitorsInTimeFrame(db, analytics.WebsiteScopedQueryParams{TimeFrame: tf, WebsiteID: 1})

	require.NoError(t, err)
	byDay := map[string]int{}
	for _, point := range series {
		byDay[point.Date[:10]] = point.Count
	}
	assert.Equal(t, map[string]int{"2026-03-28": 1, "2026-03-29": 2, "2026-03-30": 12}, byDay)
}
