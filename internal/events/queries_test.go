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
