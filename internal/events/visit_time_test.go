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

// visitTimeByBucket reads site_stats.visit_seconds by bucket, as "15:04" UTC.
func visitTimeByBucket(t *testing.T, db *gorm.DB) map[string]float64 {
	t.Helper()
	var rows []struct {
		Hour         time.Time
		VisitSeconds float64
	}
	require.NoError(t, db.Table("site_stats").Select("hour, visit_seconds").Scan(&rows).Error)
	byBucket := map[string]float64{}
	for _, r := range rows {
		byBucket[r.Hour.UTC().Format("15:04")] = r.VisitSeconds
	}
	return byBucket
}

func TestVisitTime(t *testing.T) {
	day := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	clock := func(hour, minute int) time.Time {
		return day.Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute)
	}

	// Alice: a custom event, then a page view, a click, and a page hide;
	// after an hour away, a second visit of two page views.
	// Bob: one page view. Carol: a page hide with no visit.
	collect := []struct {
		ip        string
		eventType events.EventType
		path      string
		at        time.Time
	}{
		{"1.1.1.1", events.EventTypeCustomEvent, "/", clock(10, 0)},
		{"1.1.1.1", events.EventTypePageView, "/", clock(10, 5)},
		{"1.1.1.1", events.EventTypeCustomEvent, "/", clock(10, 20)},
		{"1.1.1.1", events.EventTypePageHide, "/", clock(10, 22)},
		{"1.1.1.1", events.EventTypePageView, "/pricing", clock(11, 30)},
		{"1.1.1.1", events.EventTypePageView, "/signup", clock(11, 31)},
		{"2.2.2.2", events.EventTypePageView, "/", clock(10, 10)},
		{"3.3.3.3", events.EventTypePageHide, "/", clock(12, 0)},
	}
	want := map[string]float64{
		"10:00": 22 * 60, // Alice's first visit, 10:00 to 10:22, and Bob's 0 seconds
		"11:30": 60,      // Alice's second visit
	}

	dbManager, logger := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	testsupport.CleanAllTables(db)
	website := testsupport.CreateTestWebsite(db, "example.com")
	for _, e := range collect {
		_, err := createIngestedEvent(db, website.ID, &events.CollectEventInput{
			IPAddress: e.ip, UserAgent: "TestAgent", RawUrl: "https://example.com" + e.path,
			EventType: e.eventType, CustomEventName: "click", Timestamp: e.at,
		})
		require.NoError(t, err)
	}

	t.Run("live processing sums each visit in the bucket of its first page view", func(t *testing.T) {
		require.NoError(t, testsupport.ProcessAllTestEvents(dbManager, logger))

		assert.Equal(t, want, visitTimeByBucket(t, db))
	})

	t.Run("a rebuild from the events stores the same time", func(t *testing.T) {
		require.NoError(t, db.Exec("UPDATE site_stats SET visit_seconds = 12345").Error)

		require.NoError(t, events.RebuildVisitTime(dbManager, website.ID))

		assert.Equal(t, want, visitTimeByBucket(t, db))
	})

	t.Run("a second rebuild changes nothing", func(t *testing.T) {
		require.NoError(t, events.RebuildVisitTime(dbManager, website.ID))

		assert.Equal(t, want, visitTimeByBucket(t, db))
	})
}
