package analytics_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"fusionaly/internal/analytics"
	"fusionaly/internal/events"
	"fusionaly/internal/testsupport"
	"fusionaly/internal/timeframe"
)

// visit records one event of a visitor at a UTC time.
func visit(t *testing.T, db *gorm.DB, websiteID uint, signature string, at string, eventType events.EventType) {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, at)
	require.NoError(t, err)
	require.NoError(t, db.Create(&events.Event{
		WebsiteID:     websiteID,
		UserSignature: signature,
		Hostname:      "example.com",
		Pathname:      "/",
		EventType:     eventType,
		Timestamp:     ts,
	}).Error)
}

func visitorParams(t *testing.T, websiteID int, from, to time.Time, size timeframe.TimeFrameSize, tz *time.Location) analytics.WebsiteScopedQueryParams {
	t.Helper()
	tf, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{FromTime: from, ToTime: to, TimeFrameSize: size}, tz)
	require.NoError(t, err)
	return analytics.WebsiteScopedQueryParams{TimeFrame: tf, WebsiteID: websiteID}
}

func countsByDate(series []timeframe.DateStat, keyLen int) map[string]int {
	byDate := map[string]int{}
	for _, point := range series {
		byDate[point.Date[:keyLen]] = point.Count
	}
	return byDate
}

func TestVisitorsCountPerLocalDay(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	la, err := time.LoadLocation("America/Los_Angeles")
	require.NoError(t, err)

	t.Run("a visitor active on two local days of one UTC day counts on both days", func(t *testing.T) {
		// 21:00 PDT on Jul 1 and 10:00 PDT on Jul 2 are both Jul 2 in UTC.
		visit(t, db, 1, "alice", "2024-07-02T04:00:00Z", events.EventTypePageView)
		visit(t, db, 1, "alice", "2024-07-02T17:00:00Z", events.EventTypePageView)
		params := visitorParams(t, 1,
			time.Date(2024, 7, 1, 0, 0, 0, 0, la), time.Date(2024, 7, 2, 23, 59, 59, 0, la),
			timeframe.DailyTimeFrame, la)

		total, err := analytics.GetTotalVisitorsInTimeFrame(db, params)
		require.NoError(t, err)
		series, err := analytics.AggregatedVisitorsInTimeFrame(db, params)
		require.NoError(t, err)

		assert.Equal(t, int64(2), total)
		assert.Equal(t, map[string]int{"2024-07-01": 1, "2024-07-02": 1}, countsByDate(series, 10))
	})

	t.Run("a visitor with many page views on one day counts once", func(t *testing.T) {
		visit(t, db, 2, "bob", "2024-07-01T09:00:00Z", events.EventTypePageView)
		visit(t, db, 2, "bob", "2024-07-01T15:00:00Z", events.EventTypePageView)
		visit(t, db, 2, "carol", "2024-07-01T15:10:00Z", events.EventTypePageView)
		params := visitorParams(t, 2,
			time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 7, 1, 23, 59, 59, 0, time.UTC),
			timeframe.DailyTimeFrame, time.UTC)

		total, err := analytics.GetTotalVisitorsInTimeFrame(db, params)

		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
	})

	t.Run("a range of several days adds the visitors of each day", func(t *testing.T) {
		visit(t, db, 3, "dave", "2024-07-01T09:00:00Z", events.EventTypePageView)
		visit(t, db, 3, "dave", "2024-07-02T09:00:00Z", events.EventTypePageView)
		params := visitorParams(t, 3,
			time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 7, 2, 23, 59, 59, 0, time.UTC),
			timeframe.DailyTimeFrame, time.UTC)

		total, err := analytics.GetTotalVisitorsInTimeFrame(db, params)

		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
	})

	t.Run("custom events alone do not make a visitor", func(t *testing.T) {
		visit(t, db, 4, "erin", "2024-07-01T09:00:00Z", events.EventTypeCustomEvent)
		params := visitorParams(t, 4,
			time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 7, 1, 23, 59, 59, 0, time.UTC),
			timeframe.DailyTimeFrame, time.UTC)

		total, err := analytics.GetTotalVisitorsInTimeFrame(db, params)

		require.NoError(t, err)
		assert.Equal(t, int64(0), total)
	})

	t.Run("months add the visitors of their days", func(t *testing.T) {
		visit(t, db, 5, "frank", "2024-07-01T09:00:00Z", events.EventTypePageView)
		visit(t, db, 5, "frank", "2024-07-02T09:00:00Z", events.EventTypePageView)
		visit(t, db, 5, "frank", "2024-08-01T09:00:00Z", events.EventTypePageView)
		params := visitorParams(t, 5,
			time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 8, 31, 23, 59, 59, 0, time.UTC),
			timeframe.MonthlyTimeFrame, time.UTC)

		series, err := analytics.AggregatedVisitorsInTimeFrame(db, params)

		require.NoError(t, err)
		assert.Equal(t, map[string]int{"2024-07": 2, "2024-08": 1}, countsByDate(series, 7))
	})
}

func TestVisitorsPerHourCountEveryActiveHour(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	visit(t, db, 1, "alice", "2024-07-01T09:05:00Z", events.EventTypePageView)
	visit(t, db, 1, "alice", "2024-07-01T09:40:00Z", events.EventTypePageView)
	visit(t, db, 1, "alice", "2024-07-01T15:20:00Z", events.EventTypePageView)
	params := visitorParams(t, 1,
		time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 7, 1, 23, 59, 59, 0, time.UTC),
		timeframe.HourlyTimeFrame, time.UTC)

	series, err := analytics.AggregatedVisitorsInTimeFrame(db, params)

	require.NoError(t, err)
	byHour := countsByDate(series, 13)
	assert.Equal(t, 1, byHour["2024-07-01T09"])
	assert.Equal(t, 1, byHour["2024-07-01T15"])
	assert.Equal(t, 0, byHour["2024-07-01T12"])
	assert.Len(t, series, 24)
}

func TestVisitorSeriesShape(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()

	t.Run("weeks start on Monday and add their days", func(t *testing.T) {
		week27 := testsupport.GetFirstDayOfISOWeek(2024, 27)
		week28 := testsupport.GetFirstDayOfISOWeek(2024, 28)
		visit(t, db, 1, "alice", week27.Add(25*time.Hour).Format(time.RFC3339), events.EventTypePageView)
		visit(t, db, 1, "alice", week27.Add(49*time.Hour).Format(time.RFC3339), events.EventTypePageView)
		visit(t, db, 1, "bob", week28.Add(49*time.Hour).Format(time.RFC3339), events.EventTypePageView)
		params := visitorParams(t, 1, week27, week28.Add(7*24*time.Hour-time.Second), timeframe.WeeklyTimeFrame, time.UTC)

		series, err := analytics.AggregatedVisitorsInTimeFrame(db, params)

		require.NoError(t, err)
		assert.Equal(t, map[string]int{week27.Format("2006-01-02"): 2, week28.Format("2006-01-02"): 1}, countsByDate(series, 10))
	})

	t.Run("an empty range has a zero for every day", func(t *testing.T) {
		params := visitorParams(t, 2,
			time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 7, 3, 23, 59, 59, 0, time.UTC),
			timeframe.DailyTimeFrame, time.UTC)

		series, err := analytics.AggregatedVisitorsInTimeFrame(db, params)

		require.NoError(t, err)
		assert.Equal(t, map[string]int{"2024-07-01": 0, "2024-07-02": 0, "2024-07-03": 0}, countsByDate(series, 10))
	})

	t.Run("visitors of another website do not count", func(t *testing.T) {
		visit(t, db, 3, "carol", "2024-07-01T12:00:00Z", events.EventTypePageView)
		visit(t, db, 4, "dave", "2024-07-01T12:00:00Z", events.EventTypePageView)
		params := visitorParams(t, 3,
			time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 7, 1, 23, 59, 59, 0, time.UTC),
			timeframe.DailyTimeFrame, time.UTC)

		total, err := analytics.GetTotalVisitorsInTimeFrame(db, params)

		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
	})
}
