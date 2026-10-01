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
		assert.Equal(t, 1, wantSite.BounceCount, "a's later visit is a bounce; b's signup is engagement")
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

// zeroVisitCounts clears every counter the rebuild owns, so only the
// rebuild can restore them. Source visits (ref_stats) are not the
// rebuild's: it leaves them as they are.
func zeroVisitCounts(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec("UPDATE page_stats SET exits = 0, visitors_count = 0, entrances = 0").Error)
	require.NoError(t, db.Exec("UPDATE site_stats SET bounce_count = 0, visitors = 0, sessions = 0").Error)
}

func TestRebuildVisitCountsMatchesLiveCounting(t *testing.T) {
	start := time.Now().UTC().Add(-3 * time.Hour).Truncate(time.Hour)
	dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
	db := dbm.GetConnection()
	process(t, dbm, ingested(site.ID, "a", "/", "", start, start, events.EventTypeCustomEvent))
	process(t, dbm, ingested(site.ID, "a", "/signup", "", start.Add(time.Minute), start.Add(time.Minute), events.EventTypePageView))
	process(t, dbm, ingested(site.ID, "a", "/welcome", "", start.Add(2*time.Minute), start.Add(2*time.Minute), events.EventTypePageView))
	process(t, dbm, ingested(site.ID, "a", "/", "news.ycombinator.com", start.Add(2*time.Hour), start.Add(2*time.Hour), events.EventTypePageView))
	process(t, dbm, ingested(site.ID, "b", "/docs", "google.com", start.Add(40*time.Minute), start.Add(40*time.Minute), events.EventTypePageView))
	process(t, dbm, ingested(site.ID, "b", "/", "", start.Add(41*time.Minute), start.Add(41*time.Minute), events.EventTypeCustomEvent))
	wantSite, wantPages, wantRefs := siteTotalsFor(t, db, site.ID), pageTotalsFor(t, db, site.ID), refVisitorsFor(t, db, site.ID)
	zeroVisitCounts(t, db)

	require.NoError(t, events.RebuildVisitCounts(db, site.ID))

	assert.Equal(t, wantSite, siteTotalsFor(t, db, site.ID))
	assert.Equal(t, wantPages, pageTotalsFor(t, db, site.ID))
	assert.Equal(t, wantRefs, refVisitorsFor(t, db, site.ID))
	assert.Equal(t, siteTotals{PageViews: 4, Visitors: 2, Sessions: 3, BounceCount: 1}, wantSite, "a's last visit; b's custom event is engagement")
}

func TestRebuildKeepsSourceVisits(t *testing.T) {
	start := time.Now().UTC().Add(-3 * time.Hour).Truncate(time.Hour)
	dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
	db := dbm.GetConnection()
	tagged := ingested(site.ID, "a", "/", "", start, start, events.EventTypePageView)
	tagged.RawURL = "https://visits.test/?utm_source=newsletter"
	process(t, dbm, tagged)

	require.NoError(t, events.RebuildVisitCounts(db, site.ID))

	assert.Equal(t, map[string]int{"newsletter": 1}, refVisitorsFor(t, db, site.ID),
		"events keep the referrer, not the utm_source, so the rebuild cannot recount sources")
}
