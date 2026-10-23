package events_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/analytics"
	"fusionaly/internal/config"
	"fusionaly/internal/events"
	"fusionaly/internal/testsupport"
)

func TestDeleteOldEvents(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	site := testsupport.CreateTestWebsite(db, "example.com")
	now := time.Now().UTC()
	keptSince := events.KeptSince(now)
	// KeptSince is one day after the cut, so this is the last hour before it.
	old := keptSince.Add(-25 * time.Hour)
	event := func(name string, eventType events.EventType, meta string, at time.Time) events.Event {
		return events.Event{WebsiteID: site.ID, UserSignature: "v1", Hostname: "example.com", Pathname: "/" + name,
			EventType: eventType, CustomEventName: name, CustomEventMeta: meta, Timestamp: at}
	}
	require.NoError(t, db.Create(&[]events.Event{
		event("old-view", events.EventTypePageView, "", old),
		event("old-click", events.EventTypeCustomEvent, `{"plan":"pro"}`, old),
		event("old-purchase", events.EventTypeCustomEvent, `{"price":1000}`, old),
		event("first-kept-view", events.EventTypePageView, "", keptSince),
	}).Error)
	require.NoError(t, db.Create(&events.Event{WebsiteID: site.ID, UserSignature: "v2", Hostname: "example.com", Pathname: "/null-meta",
		EventType: events.EventTypePageView, Timestamp: old}).Error)
	require.NoError(t, db.Exec("UPDATE events SET custom_event_meta = NULL WHERE pathname = '/null-meta'").Error)
	remaining := func() []string {
		var paths []string
		require.NoError(t, db.Model(&events.Event{}).Order("pathname").Pluck("pathname", &paths).Error)
		return paths
	}

	t.Run("keeps every event when the retention is 0", func(t *testing.T) {
		cfg := config.GetConfig()
		days := cfg.EventsRetentionDays
		cfg.EventsRetentionDays = 0
		t.Cleanup(func() { cfg.EventsRetentionDays = days })

		deleted, err := events.DeleteOldEvents(dbManager, now)

		require.NoError(t, err)
		assert.Zero(t, deleted)
		assert.Len(t, remaining(), 5)
	})

	t.Run("deletes the events of older days, except the events with a price", func(t *testing.T) {
		deleted, err := events.DeleteOldEvents(dbManager, now)

		require.NoError(t, err)
		assert.Equal(t, int64(3), deleted)
		assert.Equal(t, []string{"/first-kept-view", "/old-purchase"}, remaining())
	})
}

func TestRecountsKeepTheSummariesOfDeletedDays(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	site := testsupport.CreateTestWebsite(db, "example.com")
	keptSince := events.KeptSince(time.Now())
	oldBucket := keptSince.Add(-14 * time.Hour)
	// The summaries of a day whose page views are deleted. Its purchase is
	// kept, so it is the website's first event.
	oldSite := analytics.SiteStat{WebsiteID: site.ID, Hour: oldBucket, PageViews: 3, Visitors: 2, LastVisitors: 2,
		HourVisitors: 2, Sessions: 2, BounceCount: 1, VisitSeconds: 300}
	oldPage := analytics.PageStat{WebsiteID: site.ID, Hostname: "example.com", Pathname: "/", Hour: oldBucket,
		PageViewsCount: 3, VisitorsCount: 2, Entrances: 2, Exits: 2}
	require.NoError(t, db.Create(&oldSite).Error)
	require.NoError(t, db.Create(&oldPage).Error)
	require.NoError(t, db.Create(&[]events.Event{
		{WebsiteID: site.ID, UserSignature: "v1", Hostname: "example.com", Pathname: "/", EventType: events.EventTypeCustomEvent,
			CustomEventName: "revenue:purchased", CustomEventMeta: `{"price":1000}`, Timestamp: oldBucket.Add(5 * time.Minute)},
		{WebsiteID: site.ID, UserSignature: "v2", Hostname: "example.com", Pathname: "/", EventType: events.EventTypePageView,
			Timestamp: keptSince.Add(10 * time.Hour)},
	}).Error)

	require.NoError(t, events.RebuildVisitCounts(db, site.ID))
	require.NoError(t, events.RebuildVisitTime(dbManager, site.ID))
	require.NoError(t, events.RebuildVisitorBuckets(dbManager, site.ID))

	var site2 analytics.SiteStat
	var page2 analytics.PageStat
	require.NoError(t, db.First(&site2, oldSite.ID).Error)
	require.NoError(t, db.First(&page2, oldPage.ID).Error)
	assert.Equal(t, [7]float64{3, 2, 2, 2, 2, 1, 300},
		[7]float64{float64(site2.PageViews), float64(site2.Visitors), float64(site2.LastVisitors), float64(site2.HourVisitors),
			float64(site2.Sessions), float64(site2.BounceCount), site2.VisitSeconds})
	assert.Equal(t, [3]int{2, 2, 2}, [3]int{page2.VisitorsCount, page2.Entrances, page2.Exits})
}
