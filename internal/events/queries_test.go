package events_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/events"
	"fusionaly/internal/testsupport"
)

func TestGetFilteredEvents(t *testing.T) {
	end := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	listOf := func(t *testing.T, n int) func(limit, offset int) events.EventsResult {
		t.Helper()
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "list.test")
		db := dbm.GetConnection()
		rows := make([]events.Event, n)
		for i := range rows {
			rows[i] = events.Event{WebsiteID: site.ID, UserSignature: fmt.Sprint("v", i), Hostname: "list.test",
				Pathname: "/", EventType: events.EventTypePageView, Timestamp: end.Add(-time.Duration(i+1) * time.Second)}
		}
		require.NoError(t, db.CreateInBatches(rows, 500).Error)
		return func(limit, offset int) events.EventsResult {
			result, err := events.GetFilteredEvents(db, events.EventFilters{
				WebsiteID: site.ID, FromDate: end.Add(-24 * time.Hour), ToDate: end, Limit: limit, Offset: offset,
			})
			require.NoError(t, err)
			return result
		}
	}

	t.Run("gives the exact total on the last page", func(t *testing.T) {
		list := listOf(t, 3)

		result := list(2, 2)

		assert.Len(t, result.Events, 1)
		assert.Equal(t, int64(3), result.Total)
		assert.False(t, result.TotalCapped)
	})

	t.Run("counts exactly when the matches fit in the count", func(t *testing.T) {
		list := listOf(t, 120)

		result := list(50, 0)

		assert.Len(t, result.Events, 50)
		assert.Equal(t, int64(120), result.Total)
		assert.False(t, result.TotalCapped)
	})

	t.Run("stops counting 1,000 events past the page, and says there are more", func(t *testing.T) {
		list := listOf(t, 1100)

		first, later := list(50, 0), list(50, 50)

		assert.Equal(t, int64(1000), first.Total)
		assert.True(t, first.TotalCapped)
		assert.Equal(t, int64(1050), later.Total)
		assert.True(t, later.TotalCapped)
	})

	t.Run("counts on a page past the end", func(t *testing.T) {
		list := listOf(t, 3)

		result := list(50, 100)

		assert.Empty(t, result.Events)
		assert.Equal(t, int64(3), result.Total)
		assert.False(t, result.TotalCapped)
	})
}

func TestGetDistinctEventNamesForWebsite(t *testing.T) {
	t.Run("names each custom event of the range once, in order", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		at := time.Now().UTC().Add(-time.Hour)
		arriveNamed(t, dbm, site.ID, "signup", at)
		arriveNamed(t, dbm, site.ID, "checkout", at.Add(time.Minute))
		arriveNamed(t, dbm, site.ID, "signup", at.Add(2*time.Minute))
		// A name seen only before the range: the processor keeps its count.
		require.NoError(t, dbm.GetConnection().Exec(`INSERT INTO event_stats (website_id, event_name, event_key, hour, visitors_count, page_views_count)
			VALUES (?, 'old', '', ?, 1, 1)`, site.ID, at.Add(-40*24*time.Hour).Truncate(30*time.Minute)).Error)

		names, err := events.GetDistinctEventNamesForWebsite(dbm.GetConnection(), site.ID, 30)

		require.NoError(t, err)
		require.Len(t, names, 2)
		assert.Equal(t, "checkout", names[0].EventName)
		assert.Equal(t, "signup", names[1].EventName)
		assert.Equal(t, "visits.test", names[0].Domain)
	})
}

// arriveNamed stores a custom event and runs the processor on it.
func arriveNamed(t *testing.T, dbm *testsupport.TestDBManager, websiteID uint, name string, at time.Time) {
	t.Helper()
	event := events.IngestedEvent{WebsiteID: websiteID, UserSignature: "v1", Hostname: "visits.test", Pathname: "/",
		EventType: events.EventTypeCustomEvent, CustomEventName: name,
		Timestamp: at, CreatedAt: at, UserAgent: "Mozilla/5.0 (Macintosh) Safari/605.1.15"}
	require.NoError(t, dbm.GetConnection().Create(&event).Error)
	_, err := events.ProcessUnprocessedEvents(dbm, testsupport.GetLogger(), 100)
	require.NoError(t, err)
}
