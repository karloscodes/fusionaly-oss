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

// refVisitorsFor returns each referrer's credited visitors.
func refVisitorsFor(t *testing.T, db *gorm.DB, websiteID uint) map[string]int {
	t.Helper()
	var rows []struct {
		Hostname string
		Visitors int
	}
	require.NoError(t, db.Raw(`SELECT hostname, SUM(visitors_count) visitors FROM ref_stats
		WHERE website_id = ? GROUP BY hostname`, websiteID).Scan(&rows).Error)
	byHost := map[string]int{}
	for _, r := range rows {
		byHost[r.Hostname] = r.Visitors
	}
	return byHost
}

func ingested(websiteID uint, visitor, path, referrer string, at, received time.Time, eventType events.EventType) events.IngestedEvent {
	if referrer == "" {
		referrer = events.DirectOrUnknownReferrer
	}
	return events.IngestedEvent{WebsiteID: websiteID, UserSignature: visitor, Hostname: "visits.test", Pathname: path,
		ReferrerHostname: referrer, EventType: eventType,
		CustomEventName: map[bool]string{true: "signup"}[eventType == events.EventTypeCustomEvent],
		Timestamp:       at, CreatedAt: received, UserAgent: "Mozilla/5.0 (Macintosh) Safari/605.1.15"}
}

// process stores the events as one arrival and runs the processor once.
func process(t *testing.T, dbm *testsupport.TestDBManager, evts ...events.IngestedEvent) {
	t.Helper()
	require.NoError(t, dbm.GetConnection().Create(&evts).Error)
	_, err := events.ProcessUnprocessedEvents(dbm, testsupport.GetLogger(), 100)
	require.NoError(t, err)
}

func TestVisitRules(t *testing.T) {
	start := time.Now().UTC().Add(-3 * time.Hour).Truncate(time.Hour)

	t.Run("a visit that opens with a custom event counts at its first page view", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		db := dbm.GetConnection()

		arrive(t, dbm, site.ID, "v1", "/", start, events.EventTypeCustomEvent)
		arrive(t, dbm, site.ID, "v1", "/signup", start.Add(time.Minute), events.EventTypePageView)

		assert.Equal(t, siteTotals{PageViews: 1, Visitors: 1, Sessions: 1, BounceCount: 0}, siteTotalsFor(t, db, site.ID), "the custom event is engagement: no bounce")
		assert.Equal(t, pageTotals{Pathname: "/signup", Visitors: 1, PageViews: 1, Entrances: 1, Exits: 1}, pageTotalsFor(t, db, site.ID)["/signup"])
	})

	t.Run("a second page view of a visit that opened with a custom event takes nothing back", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		db := dbm.GetConnection()

		arrive(t, dbm, site.ID, "v1", "/", start, events.EventTypeCustomEvent)
		arrive(t, dbm, site.ID, "v1", "/signup", start.Add(time.Minute), events.EventTypePageView)
		arrive(t, dbm, site.ID, "v1", "/welcome", start.Add(2*time.Minute), events.EventTypePageView)

		assert.Equal(t, siteTotals{PageViews: 2, Visitors: 1, Sessions: 1, BounceCount: 0}, siteTotalsFor(t, db, site.ID))
	})

	t.Run("each visit credits its referrer, also a visitor's later visit", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		db := dbm.GetConnection()

		process(t, dbm, ingested(site.ID, "v1", "/", "", start, start, events.EventTypePageView))
		process(t, dbm, ingested(site.ID, "v1", "/about", "", start.Add(time.Minute), start.Add(time.Minute), events.EventTypePageView))
		later := start.Add(2 * time.Hour)
		process(t, dbm, ingested(site.ID, "v1", "/", "news.ycombinator.com", later, later, events.EventTypePageView))

		assert.Equal(t, map[string]int{events.DirectOrUnknownReferrer: 1, "news.ycombinator.com": 1}, refVisitorsFor(t, db, site.ID))
	})

	t.Run("events that arrive out of order count as one visit", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		db := dbm.GetConnection()
		received := start.Add(11 * time.Second)

		process(t, dbm,
			ingested(site.ID, "v1", "/pricing", "", start.Add(10*time.Second), received, events.EventTypePageView),
			ingested(site.ID, "v1", "/", "", start, received.Add(time.Millisecond), events.EventTypePageView),
		)

		assert.Equal(t, siteTotals{PageViews: 2, Visitors: 1, Sessions: 1, BounceCount: 0}, siteTotalsFor(t, db, site.ID))
		assert.Equal(t, pageTotals{Pathname: "/", Visitors: 1, PageViews: 1, Entrances: 1, Exits: 0}, pageTotalsFor(t, db, site.ID)["/"])
	})

	t.Run("an event that arrives after a later event of its visit is processed counts once", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		db := dbm.GetConnection()

		process(t, dbm, ingested(site.ID, "v1", "/pricing", "", start.Add(10*time.Second), start.Add(11*time.Second), events.EventTypePageView))
		process(t, dbm, ingested(site.ID, "v1", "/", "", start, start.Add(2*time.Minute), events.EventTypePageView))

		assert.Equal(t, siteTotals{PageViews: 2, Visitors: 1, Sessions: 1, BounceCount: 0}, siteTotalsFor(t, db, site.ID))
	})
}

func TestQueryParameterStats(t *testing.T) {
	t.Run("keeps only the source parameters ref, source, and via", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		now := time.Now().UTC()
		event := ingested(site.ID, "v1", "/", "", now, now, events.EventTypePageView)
		event.RawURL = "https://visits.test/?ref=hn&source=newsletter&via=partner&token=secret&email=a@b.c&fbclid=xyz"

		process(t, dbm, event)

		var names []string
		require.NoError(t, dbm.GetConnection().Table("query_param_stats").Order("param_name").Pluck("param_name", &names).Error)
		assert.Equal(t, []string{"ref", "source", "via"}, names)
	})
}

func customNamed(websiteID uint, visitor, name string, at time.Time) events.IngestedEvent {
	e := ingested(websiteID, visitor, "/", "", at, at, events.EventTypeCustomEvent)
	e.CustomEventName = name
	return e
}

func pageViewAt(websiteID uint, visitor, path string, at time.Time) events.IngestedEvent {
	return ingested(websiteID, visitor, path, "", at, at, events.EventTypePageView)
}

func TestEngagementTakesBackTheBounce(t *testing.T) {
	start := time.Now().UTC().Add(-3 * time.Hour).Truncate(time.Hour)

	t.Run("a click after the only page view takes back the bounce", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")

		process(t, dbm, pageViewAt(site.ID, "v1", "/", start))
		process(t, dbm, customNamed(site.ID, "v1", "click:signup", start.Add(time.Minute)))

		assert.Equal(t, 0, siteTotalsFor(t, dbm.GetConnection(), site.ID).BounceCount)
	})

	t.Run("an automatic scroll event keeps the bounce", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")

		process(t, dbm, pageViewAt(site.ID, "v1", "/", start))
		process(t, dbm, customNamed(site.ID, "v1", "scroll:depth", start.Add(time.Minute)))

		assert.Equal(t, 1, siteTotalsFor(t, dbm.GetConnection(), site.ID).BounceCount)
	})

	t.Run("a second page view after a click takes nothing back twice", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		process(t, dbm, pageViewAt(site.ID, "other", "/", start))

		process(t, dbm, pageViewAt(site.ID, "v1", "/", start.Add(time.Minute)))
		process(t, dbm, customNamed(site.ID, "v1", "click:signup", start.Add(2*time.Minute)))
		process(t, dbm, pageViewAt(site.ID, "v1", "/welcome", start.Add(3*time.Minute)))

		assert.Equal(t, 1, siteTotalsFor(t, dbm.GetConnection(), site.ID).BounceCount, "only the other visitor's visit is a bounce")
	})

	t.Run("a visit that opens with a click is no bounce", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")

		process(t, dbm, customNamed(site.ID, "v1", "click:signup", start))
		process(t, dbm, pageViewAt(site.ID, "v1", "/", start.Add(time.Minute)))

		assert.Equal(t, siteTotals{PageViews: 1, Visitors: 1, Sessions: 1, BounceCount: 0}, siteTotalsFor(t, dbm.GetConnection(), site.ID))
	})

	t.Run("the rebuild counts bounces the same way", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		db := dbm.GetConnection()
		process(t, dbm, pageViewAt(site.ID, "a", "/", start))
		process(t, dbm, customNamed(site.ID, "a", "click:signup", start.Add(time.Minute)))
		process(t, dbm, pageViewAt(site.ID, "b", "/", start))
		process(t, dbm, customNamed(site.ID, "b", "scroll:depth", start.Add(time.Minute)))
		process(t, dbm, customNamed(site.ID, "c", "click:signup", start))
		process(t, dbm, pageViewAt(site.ID, "c", "/", start.Add(time.Minute)))
		want := siteTotalsFor(t, db, site.ID)
		zeroVisitCounts(t, db)

		require.NoError(t, events.RebuildVisitCounts(db, site.ID))

		assert.Equal(t, want, siteTotalsFor(t, db, site.ID))
		assert.Equal(t, 1, want.BounceCount)
	})
}
