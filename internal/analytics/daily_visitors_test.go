package analytics_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/analytics"
	"fusionaly/internal/testsupport"
)

func TestDailyVisitorsForWebsites(t *testing.T) {
	now := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	yesterday := time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC)

	t.Run("returns one value per day, oldest first, ending yesterday", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		require.NoError(t, db.Create(&[]analytics.SiteStat{
			{WebsiteID: 1, Visitors: 7, Hour: yesterday.Add(9 * time.Hour)},
			{WebsiteID: 1, Visitors: 5, Hour: yesterday.Add(15 * time.Hour)},
			{WebsiteID: 1, Visitors: 4, Hour: yesterday.AddDate(0, 0, -2).Add(12 * time.Hour)},
		}).Error)

		series, err := analytics.DailyVisitorsForWebsites(db, []uint{1}, 4, now)

		require.NoError(t, err)
		assert.Equal(t, []int64{0, 4, 0, 12}, series[1])
	})

	t.Run("leaves out today, which is not over yet", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		require.NoError(t, db.Create(&analytics.SiteStat{WebsiteID: 1, Visitors: 99, Hour: now.Truncate(time.Hour)}).Error)

		series, err := analytics.DailyVisitorsForWebsites(db, []uint{1}, 3, now)

		require.NoError(t, err)
		assert.Equal(t, []int64{0, 0, 0}, series[1])
	})

	t.Run("keeps websites apart and fills sites without traffic with zeros", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		require.NoError(t, db.Create(&analytics.SiteStat{WebsiteID: 2, Visitors: 3, Hour: yesterday.Add(time.Hour)}).Error)

		series, err := analytics.DailyVisitorsForWebsites(db, []uint{1, 2}, 2, now)

		require.NoError(t, err)
		assert.Equal(t, []int64{0, 0}, series[1])
		assert.Equal(t, []int64{0, 3}, series[2])
	})

	t.Run("returns an empty map for no websites", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)

		series, err := analytics.DailyVisitorsForWebsites(dbManager.GetConnection(), nil, 14, now)

		require.NoError(t, err)
		assert.Empty(t, series)
	})
}
