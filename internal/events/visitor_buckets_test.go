package events_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"fusionaly/internal/events"
	"fusionaly/internal/testsupport"
)

// visitorBuckets reads visitors, last_visitors, and hour_visitors by bucket,
// as "15:04" UTC.
func visitorBuckets(t *testing.T, db *gorm.DB) map[string][3]int {
	t.Helper()
	var rows []struct {
		Hour                                 time.Time
		Visitors, LastVisitors, HourVisitors int
	}
	require.NoError(t, db.Table("site_stats").Select("hour, visitors, last_visitors, hour_visitors").Scan(&rows).Error)
	byBucket := map[string][3]int{}
	for _, r := range rows {
		byBucket[r.Hour.UTC().Format("15:04")] = [3]int{r.Visitors, r.LastVisitors, r.HourVisitors}
	}
	return byBucket
}

func TestVisitorBuckets(t *testing.T) {
	day := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	clock := func(hour, minute int) time.Time {
		return day.Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute)
	}

	// Alice: page views at 9:05, 9:40, and 15:20. Bob: a page view at 9:10,
	// then a custom event. Carol: two page views in the 9:30 bucket.
	collect := []struct {
		ip        string
		eventType events.EventType
		at        time.Time
	}{
		{"1.1.1.1", events.EventTypePageView, clock(9, 5)},
		{"2.2.2.2", events.EventTypePageView, clock(9, 10)},
		{"3.3.3.3", events.EventTypePageView, clock(9, 31)},
		{"1.1.1.1", events.EventTypePageView, clock(9, 40)},
		{"3.3.3.3", events.EventTypePageView, clock(9, 50)},
		{"2.2.2.2", events.EventTypeCustomEvent, clock(10, 0)},
		{"1.1.1.1", events.EventTypePageView, clock(15, 20)},
	}
	// visitors (first page view of the day), last_visitors, hour_visitors
	want := map[string][3]int{
		"09:00": {2, 1, 2}, // Alice and Bob arrive; Bob leaves; both start hour 9
		"09:30": {1, 1, 1}, // Carol arrives and leaves; she starts hour 9
		"15:00": {0, 1, 1}, // Alice leaves; she starts hour 15
	}

	dbManager, logger := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	testsupport.CleanAllTables(db)
	website := testsupport.CreateTestWebsite(db, "example.com")
	for _, e := range collect {
		_, err := createIngestedEvent(db, website.ID, &events.CollectEventInput{
			IPAddress: e.ip, UserAgent: "TestAgent", RawUrl: "https://example.com/",
			EventType: e.eventType, CustomEventName: "click", Timestamp: e.at,
		})
		require.NoError(t, err)
	}

	t.Run("live processing counts each visitor at their first and last page view and each hour", func(t *testing.T) {
		require.NoError(t, testsupport.ProcessAllTestEvents(dbManager, logger))

		assert.Equal(t, want, visitorBuckets(t, db))
	})

	t.Run("a rebuild from the events stores the same counts", func(t *testing.T) {
		require.NoError(t, db.Exec("UPDATE site_stats SET last_visitors = 7, hour_visitors = 7").Error)

		require.NoError(t, events.RebuildVisitorBuckets(dbManager, website.ID))

		assert.Equal(t, want, visitorBuckets(t, db))
	})
}
