package events_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/events"
	"fusionaly/internal/testsupport"
)

func TestProcessUnprocessedEventsLimits(t *testing.T) {
	t.Run("sets aside an event that cannot be processed and keeps the rest", func(t *testing.T) {
		dbManager, logger, website := testsupport.SetupTestDBManagerWithWebsite(t, "limits.test")
		db := dbManager.GetConnection()
		now := time.Now().UTC()
		event := func(path string) events.IngestedEvent {
			return events.IngestedEvent{WebsiteID: website.ID, UserSignature: "sig", Hostname: "limits.test", Pathname: path,
				EventType: events.EventTypePageView, Timestamp: now, CreatedAt: now, UserAgent: "Mozilla/5.0 (Macintosh) Safari/605.1.15"}
		}
		batch := []events.IngestedEvent{event("/a"), event("/poison"), event("/b")}
		require.NoError(t, db.Create(&batch).Error)
		// A real database failure for exactly one event.
		require.NoError(t, db.Exec(`CREATE TRIGGER poison BEFORE INSERT ON events WHEN NEW.pathname = '/poison'
			BEGIN SELECT RAISE(ABORT, 'poison event'); END`).Error)
		t.Cleanup(func() { db.Exec("DROP TRIGGER IF EXISTS poison") })

		result, err := events.ProcessUnprocessedEvents(dbManager, logger, 10)

		require.NoError(t, err)
		assert.Equal(t, 3, result.Fetched)
		assert.Len(t, result.ProcessedEvents, 2, "the good events still get processed")
		status := func(path string) int {
			var e events.IngestedEvent
			db.Where("pathname = ?", path).First(&e)
			return e.Processed
		}
		assert.Equal(t, 1, status("/a"))
		assert.Equal(t, 1, status("/b"))
		assert.Equal(t, 2, status("/poison"), "the failing event is set aside, not retried forever")
	})
}
