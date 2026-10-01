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

func pageHidesFor(t *testing.T, db *gorm.DB, websiteID uint) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&events.Event{}).
		Where("website_id = ? AND event_type = ?", websiteID, events.EventTypePageHide).
		Count(&count).Error)
	return count
}

func eventStatsFor(t *testing.T, db *gorm.DB, websiteID uint) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Table("event_stats").Where("website_id = ?", websiteID).Count(&count).Error)
	return count
}

func TestPageHide(t *testing.T) {
	start := time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)

	t.Run("is stored and counts in no table", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		db := dbm.GetConnection()
		arrive(t, dbm, site.ID, "v1", "/", start, events.EventTypePageView)

		arrive(t, dbm, site.ID, "v1", "/", start.Add(3*time.Minute), events.EventTypePageHide)

		assert.Equal(t, int64(1), pageHidesFor(t, db, site.ID))
		assert.Equal(t, siteTotals{PageViews: 1, Visitors: 1, Sessions: 1, BounceCount: 1}, siteTotalsFor(t, db, site.ID), "a page hide is no engagement")
		assert.Equal(t, pageTotals{Pathname: "/", Visitors: 1, PageViews: 1, Entrances: 1, Exits: 1}, pageTotalsFor(t, db, site.ID)["/"])
		assert.Zero(t, eventStatsFor(t, db, site.ID), "a page hide is no custom event")
	})

	t.Run("keeps the visit alive", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		db := dbm.GetConnection()
		arrive(t, dbm, site.ID, "v1", "/", start, events.EventTypePageView)
		arrive(t, dbm, site.ID, "v1", "/", start.Add(25*time.Minute), events.EventTypePageHide)

		arrive(t, dbm, site.ID, "v1", "/pricing", start.Add(50*time.Minute), events.EventTypePageView)

		assert.Equal(t, siteTotals{PageViews: 2, Visitors: 1, Sessions: 1, BounceCount: 0}, siteTotalsFor(t, db, site.ID))
	})

	t.Run("is dropped when it would open a visit", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		db := dbm.GetConnection()
		arrive(t, dbm, site.ID, "v1", "/", start, events.EventTypePageView)

		arrive(t, dbm, site.ID, "v1", "/", start.Add(45*time.Minute), events.EventTypePageHide)
		arrive(t, dbm, site.ID, "v2", "/", start, events.EventTypePageHide)
		arrive(t, dbm, site.ID, "v1", "/pricing", start.Add(50*time.Minute), events.EventTypePageView)

		assert.Zero(t, pageHidesFor(t, db, site.ID))
		assert.Equal(t, siteTotals{PageViews: 2, Visitors: 1, Sessions: 2, BounceCount: 2}, siteTotalsFor(t, db, site.ID), "the second page view opens a new visit")
		var unprocessed int64
		require.NoError(t, db.Model(&events.IngestedEvent{}).Where("processed = 0").Count(&unprocessed).Error)
		assert.Zero(t, unprocessed, "a dropped page hide is marked processed")
	})

	t.Run("keeps the bounce in a rebuild", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		db := dbm.GetConnection()
		arrive(t, dbm, site.ID, "v1", "/", start, events.EventTypePageView)
		arrive(t, dbm, site.ID, "v1", "/", start.Add(3*time.Minute), events.EventTypePageHide)
		countLikeOldVersions(t, db)
		require.NoError(t, db.Exec("UPDATE site_stats SET bounce_count = 0").Error)

		require.NoError(t, events.RebuildVisitCounts(db, site.ID))

		assert.Equal(t, 1, siteTotalsFor(t, db, site.ID).BounceCount)
	})

	t.Run("is left out of the events list and the event count", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		db := dbm.GetConnection()
		arrive(t, dbm, site.ID, "v1", "/", start, events.EventTypePageView)
		arrive(t, dbm, site.ID, "v1", "/", start.Add(3*time.Minute), events.EventTypePageHide)

		list, err := events.GetFilteredEvents(db, events.EventFilters{
			WebsiteID: site.ID, FromDate: start.Add(-time.Hour), ToDate: start.Add(time.Hour), Limit: 50,
		})
		require.NoError(t, err)
		count, err := events.GetEventCountInTimeRange(db, site.ID, start.Add(-time.Hour), start.Add(time.Hour))
		require.NoError(t, err)

		assert.Equal(t, int64(1), list.Total)
		require.Len(t, list.Events, 1)
		assert.Equal(t, events.EventTypePageView, list.Events[0].EventType)
		assert.Equal(t, int64(1), count)
	})
}
