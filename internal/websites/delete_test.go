package websites_test

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"fusionaly/internal/analytics"
	"fusionaly/internal/annotations"
	"fusionaly/internal/events"
	"fusionaly/internal/settings"
	"fusionaly/internal/testsupport"
	"fusionaly/internal/websites"
)

// storeSiteData gives a website a raw event, a queue row, two summary rows,
// an annotation, a goal, and subdomain tracking.
func storeSiteData(t *testing.T, db *gorm.DB, site websites.Website) {
	t.Helper()
	at := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(&events.Event{WebsiteID: site.ID, UserSignature: "v1", Hostname: site.Domain, Pathname: "/", EventType: events.EventTypePageView, Timestamp: at}).Error)
	require.NoError(t, db.Create(&events.IngestedEvent{WebsiteID: site.ID, Hostname: site.Domain, Pathname: "/", Timestamp: at, Processed: 1}).Error)
	require.NoError(t, db.Create(&analytics.SiteStat{WebsiteID: site.ID, Hour: at, PageViews: 1}).Error)
	require.NoError(t, db.Create(&analytics.PageStat{WebsiteID: site.ID, Hostname: site.Domain, Pathname: "/", Hour: at, PageViewsCount: 1}).Error)
	require.NoError(t, db.Create(&annotations.Annotation{WebsiteID: site.ID, Title: "Launch", AnnotationDate: at}).Error)
	require.NoError(t, settings.SaveWebsiteGoals(db, site.ID, []string{"signup"}))
	require.NoError(t, settings.UpdateSubdomainTrackingSettings(db, site.Domain, true))
}

// rowsOf counts a website's rows in each table that has a website_id.
func rowsOf(t *testing.T, db *gorm.DB, websiteID uint) map[string]int64 {
	t.Helper()
	var tables []string
	require.NoError(t, db.Raw(`SELECT m.name FROM sqlite_master m, pragma_table_info(m.name) p
		WHERE m.type = 'table' AND p.name = 'website_id'`).Scan(&tables).Error)
	counts := map[string]int64{}
	for _, table := range tables {
		var n int64
		require.NoError(t, db.Table(table).Where("website_id = ?", websiteID).Count(&n).Error)
		if n > 0 {
			counts[table] = n
		}
	}
	return counts
}

func goalsByWebsite(t *testing.T, db *gorm.DB) map[string][]string {
	t.Helper()
	value, err := settings.GetSetting(db, "website_goals")
	require.NoError(t, err)
	var goals settings.WebsiteGoals
	require.NoError(t, json.Unmarshal([]byte(value), &goals))
	return goals.Goals
}

func TestDeleteWebsite(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	testsupport.CleanAllTables(db)
	gone := testsupport.CreateTestWebsite(db, "gone.example.com")
	kept := testsupport.CreateTestWebsite(db, "kept.example.com")
	storeSiteData(t, db, gone)
	storeSiteData(t, db, kept)

	require.NoError(t, websites.DeleteWebsite(db, gone.ID))
	deleted, err := websites.PurgeDeletedWebsites(dbManager)

	require.NoError(t, err)

	t.Run("removes every row of the website", func(t *testing.T) {
		assert.Empty(t, rowsOf(t, db, gone.ID))
		assert.Equal(t, int64(5), deleted)
	})

	t.Run("removes its goals and its subdomain setting", func(t *testing.T) {
		subdomains, err := settings.GetSubdomainTrackingSettings(db)
		require.NoError(t, err)

		assert.NotContains(t, goalsByWebsite(t, db), strconv.FormatUint(uint64(gone.ID), 10))
		assert.NotContains(t, subdomains, gone.Domain)
	})

	t.Run("keeps the data of the other website", func(t *testing.T) {
		goals, err := settings.GetWebsiteGoals(db, kept.ID)
		require.NoError(t, err)

		assert.Len(t, rowsOf(t, db, kept.ID), 5)
		assert.Equal(t, []string{"signup"}, goals)
		assert.True(t, settings.IsSubdomainTrackingEnabled(db, kept.Domain))
	})

	t.Run("a second run finds nothing to delete", func(t *testing.T) {
		deleted, err := websites.PurgeDeletedWebsites(dbManager)

		require.NoError(t, err)
		assert.Zero(t, deleted)
	})
}
