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

// These tests process events one run at a time, the way they arrive in
// production: the processor never sees a visit's later page views early.

type siteTotals struct{ PageViews, Visitors, Sessions, BounceCount int }

type pageTotals struct {
	Pathname                              string
	Visitors, PageViews, Entrances, Exits int
}

func siteTotalsFor(t *testing.T, db *gorm.DB, websiteID uint) siteTotals {
	t.Helper()
	var s siteTotals
	require.NoError(t, db.Raw(`SELECT COALESCE(SUM(page_views),0) page_views, COALESCE(SUM(visitors),0) visitors,
		COALESCE(SUM(sessions),0) sessions, COALESCE(SUM(bounce_count),0) bounce_count
		FROM site_stats WHERE website_id = ?`, websiteID).Scan(&s).Error)
	return s
}

func pageTotalsFor(t *testing.T, db *gorm.DB, websiteID uint) map[string]pageTotals {
	t.Helper()
	var rows []pageTotals
	require.NoError(t, db.Raw(`SELECT pathname, SUM(visitors_count) visitors, SUM(page_views_count) page_views,
		SUM(entrances) entrances, SUM(exits) exits
		FROM page_stats WHERE website_id = ? GROUP BY pathname`, websiteID).Scan(&rows).Error)
	byPath := map[string]pageTotals{}
	for _, r := range rows {
		byPath[r.Pathname] = r
	}
	return byPath
}

// arrive ingests one event and runs the processor, like a live request.
func arrive(t *testing.T, dbm *testsupport.TestDBManager, websiteID uint, visitor, path string, at time.Time, eventType events.EventType) {
	t.Helper()
	db := dbm.GetConnection()
	event := events.IngestedEvent{WebsiteID: websiteID, UserSignature: visitor, Hostname: "visits.test", Pathname: path,
		EventType: eventType, CustomEventName: map[bool]string{true: "signup"}[eventType == events.EventTypeCustomEvent],
		Timestamp: at, CreatedAt: at, UserAgent: "Mozilla/5.0 (Macintosh) Safari/605.1.15"}
	require.NoError(t, db.Create(&event).Error)
	_, err := events.ProcessUnprocessedEvents(dbm, testsupport.GetLogger(), 100)
	require.NoError(t, err)
}

func TestVisitCounting(t *testing.T) {
	start := time.Now().UTC().Add(-3 * time.Hour).Truncate(time.Hour)

	t.Run("a single-page visit is a bounce and its page is the exit", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		db := dbm.GetConnection()

		arrive(t, dbm, site.ID, "v1", "/", start, events.EventTypePageView)

		assert.Equal(t, siteTotals{PageViews: 1, Visitors: 1, Sessions: 1, BounceCount: 1}, siteTotalsFor(t, db, site.ID))
		assert.Equal(t, pageTotals{Pathname: "/", Visitors: 1, PageViews: 1, Entrances: 1, Exits: 1}, pageTotalsFor(t, db, site.ID)["/"])
	})

	t.Run("a second page view takes back the bounce and the first page's exit", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		db := dbm.GetConnection()

		arrive(t, dbm, site.ID, "v1", "/", start, events.EventTypePageView)
		arrive(t, dbm, site.ID, "v1", "/pricing", start.Add(time.Minute), events.EventTypePageView)

		assert.Equal(t, siteTotals{PageViews: 2, Visitors: 1, Sessions: 1, BounceCount: 0}, siteTotalsFor(t, db, site.ID))
		pages := pageTotalsFor(t, db, site.ID)
		assert.Equal(t, pageTotals{Pathname: "/", Visitors: 1, PageViews: 1, Entrances: 1, Exits: 0}, pages["/"])
		assert.Equal(t, pageTotals{Pathname: "/pricing", Visitors: 1, PageViews: 1, Entrances: 0, Exits: 1}, pages["/pricing"],
			"a page reached by navigation counts its visitor")
	})

	t.Run("a longer visit has one exit, on its last page, and no bounce", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		db := dbm.GetConnection()

		arrive(t, dbm, site.ID, "v1", "/", start, events.EventTypePageView)
		arrive(t, dbm, site.ID, "v1", "/docs", start.Add(time.Minute), events.EventTypePageView)
		arrive(t, dbm, site.ID, "v1", "/pricing", start.Add(2*time.Minute), events.EventTypePageView)

		assert.Equal(t, 0, siteTotalsFor(t, db, site.ID).BounceCount, "the bounce is taken back once, never below zero")
		pages := pageTotalsFor(t, db, site.ID)
		assert.Equal(t, 0, pages["/"].Exits)
		assert.Equal(t, 0, pages["/docs"].Exits)
		assert.Equal(t, 1, pages["/pricing"].Exits)
	})

	t.Run("viewing the same page again counts one visitor for it", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		db := dbm.GetConnection()

		arrive(t, dbm, site.ID, "v1", "/", start, events.EventTypePageView)
		arrive(t, dbm, site.ID, "v1", "/pricing", start.Add(time.Minute), events.EventTypePageView)
		arrive(t, dbm, site.ID, "v1", "/", start.Add(2*time.Minute), events.EventTypePageView)

		home := pageTotalsFor(t, db, site.ID)["/"]
		assert.Equal(t, 1, home.Visitors)
		assert.Equal(t, 2, home.PageViews)
		assert.Equal(t, 1, home.Exits, "the second view of / is the visit's last page")
	})

	t.Run("a new visit after the session timeout is counted on its own", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		db := dbm.GetConnection()

		arrive(t, dbm, site.ID, "v1", "/", start, events.EventTypePageView)
		arrive(t, dbm, site.ID, "v1", "/pricing", start.Add(time.Minute), events.EventTypePageView)
		arrive(t, dbm, site.ID, "v1", "/blog", start.Add(2*time.Hour), events.EventTypePageView)

		assert.Equal(t, siteTotals{PageViews: 3, Visitors: 1, Sessions: 2, BounceCount: 1}, siteTotalsFor(t, db, site.ID),
			"the first visit had two pages; the second, one page, is a bounce")
		pages := pageTotalsFor(t, db, site.ID)
		assert.Equal(t, 1, pages["/pricing"].Exits, "the first visit keeps its exit")
		assert.Equal(t, 1, pages["/blog"].Exits)
	})

	t.Run("a visit opened by a custom event is never a bounce", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		db := dbm.GetConnection()

		arrive(t, dbm, site.ID, "v1", "/", start, events.EventTypeCustomEvent)
		arrive(t, dbm, site.ID, "v1", "/", start.Add(time.Minute), events.EventTypePageView)
		arrive(t, dbm, site.ID, "v1", "/pricing", start.Add(2*time.Minute), events.EventTypePageView)

		totals := siteTotalsFor(t, db, site.ID)
		assert.Equal(t, 0, totals.BounceCount)
		assert.Equal(t, 1, pageTotalsFor(t, db, site.ID)["/pricing"].Exits)
	})

	t.Run("several visitors in one processing run are counted like live ones", func(t *testing.T) {
		dbm, logger, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		db := dbm.GetConnection()
		batch := []events.IngestedEvent{}
		for i, e := range []struct {
			visitor, path string
			minute        int
		}{{"a", "/", 0}, {"b", "/", 0}, {"a", "/pricing", 1}, {"b", "/docs", 2}, {"a", "/signup", 3}} {
			at := start.Add(time.Duration(e.minute) * time.Minute).Add(time.Duration(i) * time.Second)
			batch = append(batch, events.IngestedEvent{WebsiteID: site.ID, UserSignature: e.visitor, Hostname: "visits.test", Pathname: e.path,
				EventType: events.EventTypePageView, Timestamp: at, CreatedAt: at, UserAgent: "Mozilla/5.0 (Macintosh) Safari/605.1.15"})
		}
		require.NoError(t, db.Create(&batch).Error)

		_, err := events.ProcessUnprocessedEvents(dbm, logger, 100)

		require.NoError(t, err)
		assert.Equal(t, siteTotals{PageViews: 5, Visitors: 2, Sessions: 2, BounceCount: 0}, siteTotalsFor(t, db, site.ID))
		pages := pageTotalsFor(t, db, site.ID)
		assert.Equal(t, 0, pages["/"].Exits)
		assert.Equal(t, 1, pages["/docs"].Exits)
		assert.Equal(t, 1, pages["/signup"].Exits)
	})
}
