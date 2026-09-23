package feed_test

import (
	"testing"
	"time"

	"fusionaly/internal/feed"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecentForWebsite(t *testing.T) {
	now := time.Now().UTC()

	t.Run("returns the newest items for one website, newest first", func(t *testing.T) {
		db := setupTestDB(t)
		for i, title := range []string{"Busy day", "Hacker News", "Milestone"} {
			require.NoError(t, db.Create(&feed.FeedItem{
				WebsiteID: 1, ItemType: feed.ItemTypeTrafficSpike, Title: title, Description: title,
				DetectedAt: now.Add(-time.Duration(i) * time.Hour), PeriodStart: now, PeriodEnd: now,
			}).Error)
		}
		require.NoError(t, db.Create(&feed.FeedItem{
			WebsiteID: 2, ItemType: feed.ItemTypeTrafficSpike, Title: "Other site", Description: "x",
			DetectedAt: now, PeriodStart: now, PeriodEnd: now,
		}).Error)

		items, err := feed.RecentForWebsite(db, 1, now.Add(-7*24*time.Hour), now, 5)

		require.NoError(t, err)
		titles := []string{}
		for _, it := range items {
			titles = append(titles, it.Title)
		}
		assert.Equal(t, []string{"Busy day", "Hacker News", "Milestone"}, titles)
	})

	t.Run("leaves out items older than the cutoff", func(t *testing.T) {
		db := setupTestDB(t)
		require.NoError(t, db.Create(&feed.FeedItem{
			WebsiteID: 1, ItemType: feed.ItemTypeMilestone, Title: "Old", Description: "x",
			DetectedAt: now.Add(-10 * 24 * time.Hour), PeriodStart: now, PeriodEnd: now,
		}).Error)

		items, err := feed.RecentForWebsite(db, 1, now.Add(-7*24*time.Hour), now, 5)

		require.NoError(t, err)
		assert.Empty(t, items)
	})

	t.Run("leaves out items after the end of the period", func(t *testing.T) {
		db := setupTestDB(t)
		require.NoError(t, db.Create(&feed.FeedItem{
			WebsiteID: 1, ItemType: feed.ItemTypeMilestone, Title: "Later", Description: "x",
			DetectedAt: now.Add(-1 * time.Hour), PeriodStart: now, PeriodEnd: now,
		}).Error)

		items, err := feed.RecentForWebsite(db, 1, now.Add(-7*24*time.Hour), now.Add(-2*time.Hour), 5)

		require.NoError(t, err)
		assert.Empty(t, items)
	})

	t.Run("compares times across time zones", func(t *testing.T) {
		db := setupTestDB(t)
		// Detected at 01:00 UTC; the period ends at 23:00 the day before in UTC,
		// written in a +02:00 zone (01:00 local), so the item falls outside it.
		detected := time.Date(2026, 5, 24, 1, 0, 0, 0, time.UTC)
		plus2 := time.FixedZone("plus2", 2*60*60)
		require.NoError(t, db.Create(&feed.FeedItem{
			WebsiteID: 1, ItemType: feed.ItemTypeMilestone, Title: "Early", Description: "x",
			DetectedAt: detected, PeriodStart: detected, PeriodEnd: detected,
		}).Error)
		// Same instant, stored with a +02:00 offset (how the detector writes it).
		require.NoError(t, db.Create(&feed.FeedItem{
			WebsiteID: 1, ItemType: feed.ItemTypeMilestone, Title: "Early, local", Description: "x",
			DetectedAt: detected.In(plus2), PeriodStart: detected, PeriodEnd: detected,
		}).Error)

		items, err := feed.RecentForWebsite(db, 1, time.Date(2026, 5, 23, 0, 0, 0, 0, plus2), time.Date(2026, 5, 24, 1, 0, 0, 0, plus2), 5)

		require.NoError(t, err)
		assert.Empty(t, items)
	})

	t.Run("stops at the limit", func(t *testing.T) {
		db := setupTestDB(t)
		for i := 0; i < 8; i++ {
			require.NoError(t, db.Create(&feed.FeedItem{
				WebsiteID: 1, ItemType: feed.ItemTypeNewReferrer, Title: "Ref", Description: "x",
				DetectedAt: now.Add(-time.Duration(i) * time.Minute), PeriodStart: now, PeriodEnd: now,
			}).Error)
		}

		items, err := feed.RecentForWebsite(db, 1, now.Add(-7*24*time.Hour), now, 5)

		require.NoError(t, err)
		assert.Len(t, items, 5)
	})
}
