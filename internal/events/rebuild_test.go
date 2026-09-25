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

// countLikeOldVersions stores what versions before v2.4.3 stored: every page
// view an exit, every visit a bounce, and a page's visitors only on entry.
func countLikeOldVersions(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec("UPDATE page_stats SET exits = page_views_count, visitors_count = entrances").Error)
	require.NoError(t, db.Exec("UPDATE site_stats SET bounce_count = sessions").Error)
}

func TestRebuildVisitCountsOnce(t *testing.T) {
	start := time.Now().UTC().Add(-3 * time.Hour).Truncate(time.Hour)

	t.Run("restores the counts live processing makes", func(t *testing.T) {
		dbm, logger, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		db := dbm.GetConnection()
		arrive(t, dbm, site.ID, "a", "/", start, events.EventTypePageView)
		arrive(t, dbm, site.ID, "a", "/pricing", start.Add(time.Minute), events.EventTypePageView)
		arrive(t, dbm, site.ID, "a", "/", start.Add(2*time.Minute), events.EventTypePageView)
		arrive(t, dbm, site.ID, "b", "/docs", start.Add(40*time.Minute), events.EventTypePageView)
		arrive(t, dbm, site.ID, "b", "/docs", start.Add(41*time.Minute), events.EventTypeCustomEvent)
		arrive(t, dbm, site.ID, "a", "/blog", start.Add(2*time.Hour), events.EventTypePageView)
		wantSite, wantPages := siteTotalsFor(t, db, site.ID), pageTotalsFor(t, db, site.ID)
		countLikeOldVersions(t, db)
		require.NotEqual(t, wantPages, pageTotalsFor(t, db, site.ID), "the old counts differ")

		require.NoError(t, events.RebuildVisitCountsOnce(db, logger))

		assert.Equal(t, wantSite, siteTotalsFor(t, db, site.ID))
		assert.Equal(t, wantPages, pageTotalsFor(t, db, site.ID))
		assert.Equal(t, 2, wantSite.BounceCount, "b's visit and a's later visit are bounces")
	})

	t.Run("runs one time per install", func(t *testing.T) {
		dbm, logger, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		db := dbm.GetConnection()
		arrive(t, dbm, site.ID, "a", "/", start, events.EventTypePageView)
		arrive(t, dbm, site.ID, "a", "/pricing", start.Add(time.Minute), events.EventTypePageView)
		require.NoError(t, events.RebuildVisitCountsOnce(db, logger))
		countLikeOldVersions(t, db)

		require.NoError(t, events.RebuildVisitCountsOnce(db, logger))

		assert.Equal(t, 1, siteTotalsFor(t, db, site.ID).BounceCount, "the second call changes nothing")
	})

	t.Run("does nothing on an install without events", func(t *testing.T) {
		dbm, logger, _ := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")

		err := events.RebuildVisitCountsOnce(dbm.GetConnection(), logger)

		assert.NoError(t, err)
	})
}
