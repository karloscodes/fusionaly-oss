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

// sessionKeysByVisitor lists the site's events and maps each visitor to the
// session keys of their events, oldest first.
func sessionKeysByVisitor(t *testing.T, db *gorm.DB, websiteID uint, from, to time.Time) map[string][]string {
	t.Helper()
	result, err := events.GetFilteredEvents(db, events.EventFilters{WebsiteID: websiteID, FromDate: from, ToDate: to, Limit: 50})
	require.NoError(t, err)
	keys := events.SessionKeys(result.Events)
	byVisitor := map[string][]string{}
	for i := len(result.Events) - 1; i >= 0; i-- {
		signature := result.Events[i].UserSignature
		byVisitor[signature] = append(byVisitor[signature], keys[i])
	}
	return byVisitor
}

func TestSessionKeys(t *testing.T) {
	start := time.Date(2026, 3, 10, 10, 20, 0, 0, time.UTC)
	from, to := start.Add(-time.Hour), start.Add(3*time.Hour)

	t.Run("one visit across a 30-minute boundary is one session", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		arrive(t, dbm, site.ID, "v1", "/", start, events.EventTypePageView)
		arrive(t, dbm, site.ID, "v1", "/pricing", start.Add(20*time.Minute), events.EventTypePageView)

		keys := sessionKeysByVisitor(t, dbm.GetConnection(), site.ID, from, to)

		require.Len(t, keys["v1"], 2)
		assert.Equal(t, keys["v1"][0], keys["v1"][1])
	})

	t.Run("two visitors in the same 30-minute bucket are two sessions", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		arrive(t, dbm, site.ID, "v1", "/", start, events.EventTypePageView)
		arrive(t, dbm, site.ID, "v2", "/", start.Add(time.Minute), events.EventTypePageView)

		keys := sessionKeysByVisitor(t, dbm.GetConnection(), site.ID, from, to)

		require.Len(t, keys["v1"], 1)
		require.Len(t, keys["v2"], 1)
		assert.NotEqual(t, keys["v1"][0], keys["v2"][0])
	})

	t.Run("a visitor who returns after the timeout starts a new session", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		arrive(t, dbm, site.ID, "v1", "/", start, events.EventTypePageView)
		arrive(t, dbm, site.ID, "v1", "/", start.Add(time.Hour), events.EventTypePageView)

		keys := sessionKeysByVisitor(t, dbm.GetConnection(), site.ID, from, to)

		require.Len(t, keys["v1"], 2)
		assert.NotEqual(t, keys["v1"][0], keys["v1"][1])
	})

	t.Run("events without a session start fall back to the timeout gap", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		db := dbm.GetConnection()
		arrive(t, dbm, site.ID, "v1", "/", start, events.EventTypePageView)
		arrive(t, dbm, site.ID, "v1", "/pricing", start.Add(20*time.Minute), events.EventTypePageView)
		arrive(t, dbm, site.ID, "v1", "/", start.Add(90*time.Minute), events.EventTypePageView)
		arrive(t, dbm, site.ID, "v2", "/", start.Add(5*time.Minute), events.EventTypePageView)
		require.NoError(t, db.Exec("UPDATE events SET session_start = NULL").Error)

		keys := sessionKeysByVisitor(t, db, site.ID, from, to)

		require.Len(t, keys["v1"], 3)
		assert.Equal(t, keys["v1"][0], keys["v1"][1], "a 20-minute gap keeps the visit")
		assert.NotEqual(t, keys["v1"][1], keys["v1"][2], "a 70-minute gap starts a new visit")
		assert.NotEqual(t, keys["v1"][0], keys["v2"][0])
	})

	t.Run("page hides are not listed", func(t *testing.T) {
		dbm, _, site := testsupport.SetupTestDBManagerWithWebsite(t, "visits.test")
		arrive(t, dbm, site.ID, "v1", "/", start, events.EventTypePageView)
		arrive(t, dbm, site.ID, "v1", "/", start.Add(3*time.Minute), events.EventTypePageHide)

		keys := sessionKeysByVisitor(t, dbm.GetConnection(), site.ID, from, to)

		assert.Len(t, keys["v1"], 1)
	})
}
