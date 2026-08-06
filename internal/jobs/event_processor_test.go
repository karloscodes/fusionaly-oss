package jobs_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/events"
	"fusionaly/internal/jobs"
	"fusionaly/internal/testsupport"
)

func TestEventProcessorJob(t *testing.T) {
	t.Run("processes queued events even when GeoLite is not configured", func(t *testing.T) {
		dbManager, logger, website := testsupport.SetupTestDBManagerWithWebsite(t, "example.com")
		db := dbManager.GetConnection()

		now := time.Now().UTC()
		ingested := events.IngestedEvent{
			WebsiteID:        website.ID,
			UserSignature:    "test-signature",
			Hostname:         "example.com",
			Pathname:         "/",
			ReferrerHostname: events.DirectOrUnknownReferrer,
			EventType:        events.EventTypePageView,
			Timestamp:        now,
			UserAgent:        "Mozilla/5.0 (Test Agent)",
			RawURL:           "https://example.com/",
			Processed:        0,
			CreatedAt:        now,
		}
		require.NoError(t, db.Create(&ingested).Error)

		job := jobs.NewEventProcessorJob(dbManager, logger)

		err := job.Run()

		require.NoError(t, err)
		var processedCount int64
		require.NoError(t, db.Model(&events.IngestedEvent{}).Where("processed = 1").Count(&processedCount).Error)
		assert.Equal(t, int64(1), processedCount, "queued event should have been processed")

		var eventCount int64
		require.NoError(t, db.Model(&events.Event{}).Count(&eventCount).Error)
		assert.Equal(t, int64(1), eventCount, "processed event should have been written to the Event table")
	})
}
