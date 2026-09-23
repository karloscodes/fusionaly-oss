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

		items, err := feed.RecentForWebsite(db, 1, now.Add(-7*24*time.Hour), 5)

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

		items, err := feed.RecentForWebsite(db, 1, now.Add(-7*24*time.Hour), 5)

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

		items, err := feed.RecentForWebsite(db, 1, now.Add(-7*24*time.Hour), 5)

		require.NoError(t, err)
		assert.Len(t, items, 5)
	})
}
