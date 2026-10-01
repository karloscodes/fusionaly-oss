package events_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/events"
	"fusionaly/internal/testsupport"
)

func TestStoredRawURLKeepsOnlySourceParameters(t *testing.T) {
	sentAt := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

	t.Run("drops private query parameters and keeps source parameters for the stats", func(t *testing.T) {
		dbManager, logger := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)
		website := testsupport.CreateTestWebsite(db, "example.com")
		input := &events.CollectEventInput{
			IPAddress:  "203.0.113.1",
			UserAgent:  "Mozilla/5.0 (test)",
			EventType:  events.EventTypePageView,
			Timestamp:  sentAt,
			ReceivedAt: sentAt,
			RawUrl:     "https://example.com/welcome?secret=hunter2&email=ann%40example.com&utm_source=newsletter&utm_medium=email&utm_campaign=spring&ref=producthunt",
		}

		require.NoError(t, events.CollectEvent(dbManager, logger, input))
		var ingested events.IngestedEvent
		require.NoError(t, db.Where("website_id = ?", website.ID).First(&ingested).Error)
		_, err := events.ProcessUnprocessedEvents(dbManager, logger, 10)
		require.NoError(t, err)

		assert.NotContains(t, ingested.RawURL, "secret")
		assert.NotContains(t, ingested.RawURL, "hunter2")
		assert.NotContains(t, ingested.RawURL, "email=")
		assert.Contains(t, ingested.RawURL, "https://example.com/welcome?")
		var utmCount int64
		require.NoError(t, db.Table("utm_stats").
			Where("website_id = ? AND utm_source = ? AND utm_medium = ? AND utm_campaign = ?", website.ID, "newsletter", "email", "spring").
			Count(&utmCount).Error)
		assert.Equal(t, int64(1), utmCount)
		var refCount int64
		require.NoError(t, db.Table("query_param_stats").
			Where("website_id = ? AND param_name = ? AND param_value = ?", website.ID, "ref", "producthunt").
			Count(&refCount).Error)
		assert.Equal(t, int64(1), refCount)
	})

	t.Run("stores a URL without query parameters as it is", func(t *testing.T) {
		dbManager, logger := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)
		website := testsupport.CreateTestWebsite(db, "example.com")
		input := &events.CollectEventInput{
			IPAddress:  "203.0.113.1",
			UserAgent:  "Mozilla/5.0 (test)",
			EventType:  events.EventTypePageView,
			Timestamp:  sentAt,
			ReceivedAt: sentAt,
			RawUrl:     "https://example.com/pricing",
		}

		require.NoError(t, events.CollectEvent(dbManager, logger, input))

		var ingested events.IngestedEvent
		require.NoError(t, db.Where("website_id = ?", website.ID).First(&ingested).Error)
		assert.Equal(t, "https://example.com/pricing", ingested.RawURL)
	})
}
