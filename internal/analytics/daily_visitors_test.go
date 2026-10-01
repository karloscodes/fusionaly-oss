package analytics_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"fusionaly/internal/analytics"
	"fusionaly/internal/events"
	"fusionaly/internal/testsupport"
)

// visitorsAt stores one page view for each of n visitors of a website.
func visitorsAt(t *testing.T, db *gorm.DB, websiteID uint, n int, at time.Time) {
	t.Helper()
	for i := 0; i < n; i++ {
		e := events.Event{WebsiteID: websiteID, UserSignature: fmt.Sprintf("w%d-%s-%d", websiteID, at.Format(time.RFC3339), i),
			Hostname: "example.com", Pathname: "/", EventType: events.EventTypePageView, Timestamp: at}
		require.NoError(t, db.Create(&e).Error)
	}
}

func TestDailyVisitorsForWebsites(t *testing.T) {
	now := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	yesterday := time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC)

	t.Run("returns one value per day, oldest first, ending yesterday", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		visitorsAt(t, db, 1, 7, yesterday.Add(9*time.Hour))
		visitorsAt(t, db, 1, 5, yesterday.Add(15*time.Hour))
		visitorsAt(t, db, 1, 4, yesterday.AddDate(0, 0, -2).Add(12*time.Hour))

		series, err := analytics.DailyVisitorsForWebsites(db, []uint{1}, 4, now, time.UTC)

		require.NoError(t, err)
		assert.Equal(t, []int64{0, 4, 0, 12}, series[1])
	})

	t.Run("counts a visitor once per day", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		for _, at := range []time.Time{yesterday.Add(9 * time.Hour), yesterday.Add(15 * time.Hour)} {
			e := events.Event{WebsiteID: 1, UserSignature: "same", Hostname: "example.com", Pathname: "/", EventType: events.EventTypePageView, Timestamp: at}
			require.NoError(t, db.Create(&e).Error)
		}

		series, err := analytics.DailyVisitorsForWebsites(db, []uint{1}, 1, now, time.UTC)

		require.NoError(t, err)
		assert.Equal(t, []int64{1}, series[1])
	})

	t.Run("leaves out today, which is not over yet", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		visitorsAt(t, db, 1, 9, now.Truncate(time.Hour))

		series, err := analytics.DailyVisitorsForWebsites(db, []uint{1}, 3, now, time.UTC)

		require.NoError(t, err)
		assert.Equal(t, []int64{0, 0, 0}, series[1])
	})

	t.Run("keeps websites apart and fills sites without traffic with zeros", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		visitorsAt(t, db, 2, 3, yesterday.Add(time.Hour))

		series, err := analytics.DailyVisitorsForWebsites(db, []uint{1, 2}, 2, now, time.UTC)

		require.NoError(t, err)
		assert.Equal(t, []int64{0, 0}, series[1])
		assert.Equal(t, []int64{0, 3}, series[2])
	})

	t.Run("returns an empty map for no websites", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)

		series, err := analytics.DailyVisitorsForWebsites(dbManager.GetConnection(), nil, 14, now, time.UTC)

		require.NoError(t, err)
		assert.Empty(t, series)
	})

	t.Run("counts days in the viewer's time zone", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		newYork, err := time.LoadLocation("America/New_York")
		require.NoError(t, err)
		visitorsAt(t, db, 1, 6, yesterday.Add(2*time.Hour))

		series, err := analytics.DailyVisitorsForWebsites(db, []uint{1}, 3, now, newYork)

		require.NoError(t, err)
		assert.Equal(t, []int64{0, 6, 0}, series[1], "02:00 UTC on May 23 is the evening of May 22 in New York")
	})

	t.Run("counts a visitor on each local day they came", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		newYork, err := time.LoadLocation("America/New_York")
		require.NoError(t, err)
		// One UTC day (May 23), two New York days: 21:30 May 22 and 10:00 May 23.
		for _, at := range []time.Time{yesterday.Add(90 * time.Minute), yesterday.Add(14 * time.Hour)} {
			e := events.Event{WebsiteID: 1, UserSignature: "same", Hostname: "example.com", Pathname: "/", EventType: events.EventTypePageView, Timestamp: at}
			require.NoError(t, db.Create(&e).Error)
		}

		series, err := analytics.DailyVisitorsForWebsites(db, []uint{1}, 3, now, newYork)

		require.NoError(t, err)
		assert.Equal(t, []int64{0, 1, 1}, series[1])
	})
}
