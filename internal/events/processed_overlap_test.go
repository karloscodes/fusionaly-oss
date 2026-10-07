package events_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/events"
	"fusionaly/internal/testsupport"
)

// During a deploy the old and the new container both run a processor. Both
// can read the same rows before either marks them processed.
func TestProcessBatchByTwoProcessors(t *testing.T) {
	t.Run("counts each event once", func(t *testing.T) {
		dbManager, logger, website := testsupport.SetupTestDBManagerWithWebsite(t, "overlap.test")
		db := dbManager.GetConnection()
		now := time.Now().UTC()
		rows := []events.IngestedEvent{
			{WebsiteID: website.ID, UserSignature: "sig-a", Hostname: "overlap.test", Pathname: "/a",
				EventType: events.EventTypePageView, Timestamp: now, CreatedAt: now, UserAgent: "Mozilla/5.0 (Macintosh) Safari/605.1.15"},
			{WebsiteID: website.ID, UserSignature: "sig-b", Hostname: "overlap.test", Pathname: "/b",
				EventType: events.EventTypePageView, Timestamp: now, CreatedAt: now, UserAgent: "Mozilla/5.0 (Macintosh) Safari/605.1.15"},
		}
		require.NoError(t, db.Create(&rows).Error)
		var batch []events.IngestedEvent // both processors read this
		require.NoError(t, db.Where("processed = 0").Order("id").Find(&batch).Error)

		first := &events.EventProcessingResult{}
		second := &events.EventProcessingResult{}
		require.NoError(t, events.ProcessBatchInWrite(dbManager, logger, batch, first))
		require.NoError(t, events.ProcessBatchInWrite(dbManager, logger, batch, second))

		assert.Len(t, first.ProcessedEvents, 2)
		assert.Empty(t, second.ProcessedEvents, "the second processor finds nothing left to do")
		var stored int64
		db.Model(&events.Event{}).Where("website_id = ?", website.ID).Count(&stored)
		assert.Equal(t, int64(2), stored, "no copies")
		var views int64
		db.Table("site_stats").Where("website_id = ?", website.ID).Select("COALESCE(SUM(page_views), 0)").Scan(&views)
		assert.Equal(t, int64(2), views, "page views counted once")
	})
}
