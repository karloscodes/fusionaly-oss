package feed_test

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"fusionaly/internal/feed"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// Create required tables
	err = db.Exec(`
		CREATE TABLE websites (
			id INTEGER PRIMARY KEY,
			domain TEXT NOT NULL,
			created_at DATETIME
		)
	`).Error
	require.NoError(t, err)

	err = db.Exec(`
		CREATE TABLE site_stats (
			id INTEGER PRIMARY KEY,
			website_id INTEGER NOT NULL,
			page_views INTEGER,
			visitors INTEGER,
			sessions INTEGER,
			bounce_count INTEGER,
			hour DATETIME,
			created_at DATETIME
		)
	`).Error
	require.NoError(t, err)

	err = db.Exec(`
		CREATE TABLE ref_stats (
			id INTEGER PRIMARY KEY,
			website_id INTEGER NOT NULL,
			hostname TEXT,
			pathname TEXT,
			visitors_count INTEGER,
			page_views_count INTEGER,
			hour DATETIME,
			created_at DATETIME
		)
	`).Error
	require.NoError(t, err)

	err = db.Exec(`
		CREATE TABLE event_stats (
			id INTEGER PRIMARY KEY,
			website_id INTEGER NOT NULL,
			event_name TEXT,
			event_key TEXT,
			visitors_count INTEGER,
			page_views_count INTEGER,
			hour DATETIME,
			created_at DATETIME
		)
	`).Error
	require.NoError(t, err)

	err = db.Exec(`
		CREATE TABLE settings (
			id INTEGER PRIMARY KEY,
			key TEXT NOT NULL UNIQUE,
			value TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)
	`).Error
	require.NoError(t, err)

	err = feed.AutoMigrate(db)
	require.NoError(t, err)

	return db
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestDetector_TrafficSpike(t *testing.T) {
	db := setupTestDB(t)

	// Create website
	db.Exec("INSERT INTO websites (id, domain) VALUES (1, 'test.com')")

	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1).Truncate(24 * time.Hour)

	// Seed 7 days of normal traffic (~100 visitors/day)
	for i := 8; i >= 2; i-- {
		day := now.AddDate(0, 0, -i).Truncate(24 * time.Hour)
		db.Exec(`
			INSERT INTO site_stats (website_id, visitors, hour)
			VALUES (1, 100, ?)
		`, day.Add(12*time.Hour))
	}

	// Yesterday: traffic spike (200 visitors - 100% increase)
	db.Exec(`
		INSERT INTO site_stats (website_id, visitors, hour)
		VALUES (1, 200, ?)
	`, yesterday.Add(12*time.Hour))

	detector := feed.NewDetector(db, testLogger())
	err := detector.DetectForWebsite(1)
	require.NoError(t, err)

	var items []feed.FeedItem
	db.Where("website_id = ? AND item_type = ?", 1, feed.ItemTypeTrafficSpike).Find(&items)

	assert.Len(t, items, 1)
	assert.Equal(t, "Busy day", items[0].Title)
	assert.Contains(t, items[0].Description, "200 visitors")
}

func TestDetector_TrafficDrop(t *testing.T) {
	db := setupTestDB(t)

	db.Exec("INSERT INTO websites (id, domain) VALUES (1, 'test.com')")

	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1).Truncate(24 * time.Hour)

	// Seed 7 days of normal traffic (~100 visitors/day)
	for i := 8; i >= 2; i-- {
		day := now.AddDate(0, 0, -i).Truncate(24 * time.Hour)
		db.Exec(`
			INSERT INTO site_stats (website_id, visitors, hour)
			VALUES (1, 100, ?)
		`, day.Add(12*time.Hour))
	}

	// Yesterday: traffic drop (50 visitors - 50% decrease)
	db.Exec(`
		INSERT INTO site_stats (website_id, visitors, hour)
		VALUES (1, 50, ?)
	`, yesterday.Add(12*time.Hour))

	detector := feed.NewDetector(db, testLogger())
	err := detector.DetectForWebsite(1)
	require.NoError(t, err)

	var items []feed.FeedItem
	db.Where("website_id = ? AND item_type = ?", 1, feed.ItemTypeTrafficDrop).Find(&items)

	assert.Len(t, items, 1)
	assert.Equal(t, "Slow day", items[0].Title)
	assert.Contains(t, items[0].Description, "50 visitors")
}

func TestDetector_NewReferrer(t *testing.T) {
	db := setupTestDB(t)

	db.Exec("INSERT INTO websites (id, domain) VALUES (1, 'test.com')")

	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1).Truncate(24 * time.Hour)

	// Add an existing referrer with history
	for i := 8; i >= 2; i-- {
		day := now.AddDate(0, 0, -i).Truncate(24 * time.Hour)
		db.Exec(`
			INSERT INTO ref_stats (website_id, hostname, pathname, visitors_count, hour)
			VALUES (1, 'google.com', '/', 20, ?)
		`, day.Add(12*time.Hour))
	}

	// Yesterday: new referrer appears with significant traffic
	db.Exec(`
		INSERT INTO ref_stats (website_id, hostname, pathname, visitors_count, hour)
		VALUES (1, 'news.ycombinator.com', '/', 25, ?)
	`, yesterday.Add(14*time.Hour))

	detector := feed.NewDetector(db, testLogger())
	err := detector.DetectForWebsite(1)
	require.NoError(t, err)

	var items []feed.FeedItem
	db.Where("website_id = ? AND item_type = ?", 1, feed.ItemTypeNewReferrer).Find(&items)

	assert.Len(t, items, 1)
	assert.Equal(t, "Hacker News", items[0].Title)
	assert.Contains(t, items[0].Description, "Hacker News")
	assert.Contains(t, items[0].Description, "25 visitors")
}

func TestDetector_GoalSpike(t *testing.T) {
	db := setupTestDB(t)

	db.Exec("INSERT INTO websites (id, domain) VALUES (1, 'test.com')")
	// Configure 'signup' as a goal for website 1
	db.Exec(`INSERT INTO settings (key, value) VALUES ('website_goals', '{"goals":{"1":["signup"]}}')`)

	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1).Truncate(24 * time.Hour)

	// Seed 7 days of normal goal conversions (~5/day)
	for i := 8; i >= 2; i-- {
		day := now.AddDate(0, 0, -i).Truncate(24 * time.Hour)
		db.Exec(`
			INSERT INTO event_stats (website_id, event_name, visitors_count, hour)
			VALUES (1, 'signup', 5, ?)
		`, day.Add(15*time.Hour))
	}

	// Yesterday: goal spike (25 conversions - 5x normal)
	db.Exec(`
		INSERT INTO event_stats (website_id, event_name, visitors_count, hour)
		VALUES (1, 'signup', 25, ?)
	`, yesterday.Add(15*time.Hour))

	detector := feed.NewDetector(db, testLogger())
	err := detector.DetectForWebsite(1)
	require.NoError(t, err)

	var items []feed.FeedItem
	db.Where("website_id = ? AND item_type = ?", 1, feed.ItemTypeGoalHit).Find(&items)

	assert.Len(t, items, 1)
	assert.Equal(t, "Conversions up", items[0].Title)
	assert.Contains(t, items[0].Description, "signup")
	assert.Contains(t, items[0].Description, "25")
}

func TestDetector_Milestone(t *testing.T) {
	db := setupTestDB(t)

	db.Exec("INSERT INTO websites (id, domain) VALUES (1, 'test.com')")

	now := time.Now().UTC()
	weekAgo := now.AddDate(0, 0, -7)

	// Historical visitors (before threshold)
	db.Exec(`
		INSERT INTO site_stats (website_id, visitors, hour)
		VALUES (1, 800, ?)
	`, weekAgo.Add(-24*time.Hour))

	// Recent visitors (push over 1000)
	db.Exec(`
		INSERT INTO site_stats (website_id, visitors, hour)
		VALUES (1, 300, ?)
	`, now.Add(-24*time.Hour))

	detector := feed.NewDetector(db, testLogger())
	err := detector.DetectForWebsite(1)
	require.NoError(t, err)

	var items []feed.FeedItem
	db.Where("website_id = ? AND item_type = ?", 1, feed.ItemTypeMilestone).Find(&items)

	assert.Len(t, items, 1)
	assert.Equal(t, "Milestone", items[0].Title)
	assert.Contains(t, items[0].Description, "1K")
}

func TestDetector_NoDuplicates(t *testing.T) {
	db := setupTestDB(t)

	db.Exec("INSERT INTO websites (id, domain) VALUES (1, 'test.com')")

	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1).Truncate(24 * time.Hour)

	// Seed traffic spike data
	for i := 8; i >= 2; i-- {
		day := now.AddDate(0, 0, -i).Truncate(24 * time.Hour)
		db.Exec(`INSERT INTO site_stats (website_id, visitors, hour) VALUES (1, 100, ?)`, day.Add(12*time.Hour))
	}
	db.Exec(`INSERT INTO site_stats (website_id, visitors, hour) VALUES (1, 200, ?)`, yesterday.Add(12*time.Hour))

	detector := feed.NewDetector(db, testLogger())

	// Run detection twice
	err := detector.DetectForWebsite(1)
	require.NoError(t, err)

	err = detector.DetectForWebsite(1)
	require.NoError(t, err)

	// Should have no duplicate traffic spikes (only one per period)
	var spikeCount int64
	db.Model(&feed.FeedItem{}).Where("website_id = ? AND item_type = ?", 1, feed.ItemTypeTrafficSpike).Count(&spikeCount)
	assert.Equal(t, int64(1), spikeCount)
}

func TestDetector_LowTrafficSpikeDetection(t *testing.T) {
	db := setupTestDB(t)

	db.Exec("INSERT INTO websites (id, domain) VALUES (1, 'test.com')")

	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1).Truncate(24 * time.Hour)

	// Low traffic site: 3 visitors/day baseline
	for i := 8; i >= 2; i-- {
		day := now.AddDate(0, 0, -i).Truncate(24 * time.Hour)
		db.Exec(`INSERT INTO site_stats (website_id, visitors, hour) VALUES (1, 3, ?)`, day.Add(12*time.Hour))
	}
	// Yesterday: 5 visitors — cold start mean=3, stddev=0.75, z=(5-3)/0.75=2.67 → spike
	db.Exec(`INSERT INTO site_stats (website_id, visitors, hour) VALUES (1, 5, ?)`, yesterday.Add(12*time.Hour))

	detector := feed.NewDetector(db, testLogger())
	err := detector.DetectForWebsite(1)
	require.NoError(t, err)

	// Small sites now get spikes when z-score warrants it
	var spikeCount int64
	db.Model(&feed.FeedItem{}).Where("website_id = ? AND item_type = ?", 1, feed.ItemTypeTrafficSpike).Count(&spikeCount)
	assert.Equal(t, int64(1), spikeCount)
}

func TestDetector_ZeroTrafficNoSpike(t *testing.T) {
	db := setupTestDB(t)

	db.Exec("INSERT INTO websites (id, domain) VALUES (1, 'test.com')")

	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1).Truncate(24 * time.Hour)

	// No traffic at all yesterday
	_ = yesterday

	detector := feed.NewDetector(db, testLogger())
	err := detector.DetectForWebsite(1)
	require.NoError(t, err)

	var spikeCount int64
	db.Model(&feed.FeedItem{}).Where("website_id = ? AND item_type = ?", 1, feed.ItemTypeTrafficSpike).Count(&spikeCount)
	assert.Equal(t, int64(0), spikeCount)
}

func TestDetector_TrendingContent(t *testing.T) {
	db := setupTestDB(t)

	// Create page_stats table
	err := db.Exec(`
		CREATE TABLE page_stats (
			id INTEGER PRIMARY KEY,
			website_id INTEGER NOT NULL,
			hostname TEXT,
			pathname TEXT,
			page_views_count INTEGER,
			visitors_count INTEGER,
			entrances INTEGER,
			exits INTEGER,
			hour DATETIME,
			created_at DATETIME
		)
	`).Error
	require.NoError(t, err)

	db.Exec("INSERT INTO websites (id, domain) VALUES (1, 'test.com')")

	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1).Truncate(24 * time.Hour)

	// Seed 7 days of normal traffic for a page (~10 visitors/day)
	for i := 8; i >= 2; i-- {
		day := now.AddDate(0, 0, -i).Truncate(24 * time.Hour)
		db.Exec(`
			INSERT INTO page_stats (website_id, hostname, pathname, visitors_count, hour)
			VALUES (1, 'test.com', '/blog/my-post', 10, ?)
		`, day.Add(12*time.Hour))
	}

	// Yesterday: the post went viral (50 visitors - 5x normal)
	db.Exec(`
		INSERT INTO page_stats (website_id, hostname, pathname, visitors_count, hour)
		VALUES (1, 'test.com', '/blog/my-post', 50, ?)
	`, yesterday.Add(12*time.Hour))

	detector := feed.NewDetector(db, testLogger())
	err = detector.DetectForWebsite(1)
	require.NoError(t, err)

	var items []feed.FeedItem
	db.Where("website_id = ? AND item_type = ?", 1, feed.ItemTypeTrendingContent).Find(&items)

	assert.Len(t, items, 1)
	assert.Equal(t, "Popular page", items[0].Title)
	assert.Contains(t, items[0].Description, "/blog/my-post")
	assert.Contains(t, items[0].Description, "50 visitors")
}

func TestDetector_NewPopularPage(t *testing.T) {
	db := setupTestDB(t)

	// Create page_stats table
	err := db.Exec(`
		CREATE TABLE page_stats (
			id INTEGER PRIMARY KEY,
			website_id INTEGER NOT NULL,
			hostname TEXT,
			pathname TEXT,
			page_views_count INTEGER,
			visitors_count INTEGER,
			entrances INTEGER,
			exits INTEGER,
			hour DATETIME,
			created_at DATETIME
		)
	`).Error
	require.NoError(t, err)

	db.Exec("INSERT INTO websites (id, domain) VALUES (1, 'test.com')")

	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1).Truncate(24 * time.Hour)

	// A brand new page with no history gets significant traffic
	db.Exec(`
		INSERT INTO page_stats (website_id, hostname, pathname, visitors_count, hour)
		VALUES (1, 'test.com', '/new-feature', 30, ?)
	`, yesterday.Add(12*time.Hour))

	detector := feed.NewDetector(db, testLogger())
	err = detector.DetectForWebsite(1)
	require.NoError(t, err)

	var items []feed.FeedItem
	db.Where("website_id = ? AND item_type = ?", 1, feed.ItemTypeTrendingContent).Find(&items)

	assert.Len(t, items, 1)
	assert.Equal(t, "New page", items[0].Title)
	assert.Contains(t, items[0].Description, "/new-feature")
	assert.Contains(t, items[0].Description, "30 visitors")
}

func TestDetector_MonthlySummary(t *testing.T) {
	db := setupTestDB(t)

	// Create page_stats table
	err := db.Exec(`
		CREATE TABLE page_stats (
			id INTEGER PRIMARY KEY,
			website_id INTEGER NOT NULL,
			hostname TEXT,
			pathname TEXT,
			page_views_count INTEGER,
			visitors_count INTEGER,
			entrances INTEGER,
			exits INTEGER,
			hour DATETIME,
			created_at DATETIME
		)
	`).Error
	require.NoError(t, err)

	db.Exec("INSERT INTO websites (id, domain) VALUES (1, 'test.com')")

	now := time.Now().UTC()
	firstOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	firstOfLastMonth := firstOfThisMonth.AddDate(0, -1, 0)
	firstOfTwoMonthsAgo := firstOfThisMonth.AddDate(0, -2, 0)

	// Seed last month with 500 visitors
	for i := 0; i < 10; i++ {
		day := firstOfLastMonth.AddDate(0, 0, i)
		db.Exec(`INSERT INTO site_stats (website_id, visitors, hour) VALUES (1, 50, ?)`, day.Add(12*time.Hour))
	}

	// Seed two months ago with 400 visitors
	for i := 0; i < 8; i++ {
		day := firstOfTwoMonthsAgo.AddDate(0, 0, i)
		db.Exec(`INSERT INTO site_stats (website_id, visitors, hour) VALUES (1, 50, ?)`, day.Add(12*time.Hour))
	}

	// Seed page stats for last month
	db.Exec(`INSERT INTO page_stats (website_id, pathname, visitors_count, hour) VALUES (1, '/blog', 200, ?)`, firstOfLastMonth.Add(12*time.Hour))
	db.Exec(`INSERT INTO page_stats (website_id, pathname, visitors_count, hour) VALUES (1, '/about', 100, ?)`, firstOfLastMonth.Add(12*time.Hour))

	// Seed ref stats for last month
	db.Exec(`INSERT INTO ref_stats (website_id, hostname, visitors_count, hour) VALUES (1, 'google.com', 150, ?)`, firstOfLastMonth.Add(12*time.Hour))

	detector := feed.NewDetector(db, testLogger())
	err = detector.DetectForWebsite(1)
	require.NoError(t, err)

	var items []feed.FeedItem
	db.Where("website_id = ? AND item_type = ?", 1, feed.ItemTypeMonthlySummary).Find(&items)

	assert.Len(t, items, 1)
	assert.Contains(t, items[0].Description, "500")
	assert.Contains(t, items[0].Description, "up")
	assert.Contains(t, items[0].Description, "/blog") // top page highlight

	// Verify metadata has top pages and sources
	metadata := items[0].MetadataMap()
	assert.NotNil(t, metadata["topPages"])
	assert.NotNil(t, metadata["topSources"])
}

func TestDetector_MonthlySummary_AnyTrafficGetsSummary(t *testing.T) {
	db := setupTestDB(t)

	db.Exec("INSERT INTO websites (id, domain) VALUES (1, 'test.com')")

	now := time.Now().UTC()
	firstOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	firstOfLastMonth := firstOfThisMonth.AddDate(0, -1, 0)

	// Even small traffic gets a monthly summary
	db.Exec(`INSERT INTO site_stats (website_id, visitors, hour) VALUES (1, 5, ?)`, firstOfLastMonth.Add(12*time.Hour))

	detector := feed.NewDetector(db, testLogger())
	err := detector.DetectForWebsite(1)
	require.NoError(t, err)

	var count int64
	db.Model(&feed.FeedItem{}).Where("website_id = ? AND item_type = ?", 1, feed.ItemTypeMonthlySummary).Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestDetector_MonthlySummary_SkipsZeroTraffic(t *testing.T) {
	db := setupTestDB(t)

	db.Exec("INSERT INTO websites (id, domain) VALUES (1, 'test.com')")

	detector := feed.NewDetector(db, testLogger())
	err := detector.DetectForWebsite(1)
	require.NoError(t, err)

	var count int64
	db.Model(&feed.FeedItem{}).Where("website_id = ? AND item_type = ?", 1, feed.ItemTypeMonthlySummary).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestDetector_DroppingPages(t *testing.T) {
	db := setupTestDB(t)

	// Create page_stats table
	err := db.Exec(`
		CREATE TABLE page_stats (
			id INTEGER PRIMARY KEY,
			website_id INTEGER NOT NULL,
			hostname TEXT,
			pathname TEXT,
			page_views_count INTEGER,
			visitors_count INTEGER,
			entrances INTEGER,
			exits INTEGER,
			hour DATETIME,
			created_at DATETIME
		)
	`).Error
	require.NoError(t, err)

	db.Exec("INSERT INTO websites (id, domain) VALUES (1, 'test.com')")

	now := time.Now().UTC()
	firstOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	firstOfLastMonth := firstOfThisMonth.AddDate(0, -1, 0)
	firstOfTwoMonthsAgo := firstOfThisMonth.AddDate(0, -2, 0)

	// Two months ago: page had 100 visitors
	db.Exec(`INSERT INTO page_stats (website_id, pathname, visitors_count, hour) VALUES (1, '/dropping-page', 100, ?)`, firstOfTwoMonthsAgo.Add(12*time.Hour))

	// Last month: page dropped to 50 visitors (50% drop)
	db.Exec(`INSERT INTO page_stats (website_id, pathname, visitors_count, hour) VALUES (1, '/dropping-page', 50, ?)`, firstOfLastMonth.Add(12*time.Hour))

	detector := feed.NewDetector(db, testLogger())
	err = detector.DetectForWebsite(1)
	require.NoError(t, err)

	var items []feed.FeedItem
	db.Where("website_id = ? AND item_type = ?", 1, feed.ItemTypeDroppingPages).Find(&items)

	assert.Len(t, items, 1)
	assert.Equal(t, "Traffic shifts", items[0].Title)
	assert.Contains(t, items[0].Description, "/dropping-page")
	assert.Contains(t, items[0].Description, "50%")

	metadata := items[0].MetadataMap()
	pages := metadata["pages"].([]any)
	assert.Len(t, pages, 1)
}

func TestDetector_DroppingPages_IgnoresTinyPages(t *testing.T) {
	db := setupTestDB(t)

	// Create page_stats table
	err := db.Exec(`
		CREATE TABLE page_stats (
			id INTEGER PRIMARY KEY,
			website_id INTEGER NOT NULL,
			hostname TEXT,
			pathname TEXT,
			page_views_count INTEGER,
			visitors_count INTEGER,
			hour DATETIME
		)
	`).Error
	require.NoError(t, err)

	db.Exec("INSERT INTO websites (id, domain) VALUES (1, 'test.com')")

	now := time.Now().UTC()
	firstOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	firstOfLastMonth := firstOfThisMonth.AddDate(0, -1, 0)
	firstOfTwoMonthsAgo := firstOfThisMonth.AddDate(0, -2, 0)

	// Two months ago: page had only 3 visitors (below 5 threshold)
	db.Exec(`INSERT INTO page_stats (website_id, pathname, visitors_count, hour) VALUES (1, '/tiny-page', 3, ?)`, firstOfTwoMonthsAgo.Add(12*time.Hour))

	// Last month: dropped to 1 visitor
	db.Exec(`INSERT INTO page_stats (website_id, pathname, visitors_count, hour) VALUES (1, '/tiny-page', 1, ?)`, firstOfLastMonth.Add(12*time.Hour))

	detector := feed.NewDetector(db, testLogger())
	err = detector.DetectForWebsite(1)
	require.NoError(t, err)

	var count int64
	db.Model(&feed.FeedItem{}).Where("website_id = ? AND item_type = ?", 1, feed.ItemTypeDroppingPages).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestDetector_BestSources(t *testing.T) {
	db := setupTestDB(t)

	db.Exec("INSERT INTO websites (id, domain) VALUES (1, 'test.com')")

	now := time.Now().UTC()
	firstOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	firstOfLastMonth := firstOfThisMonth.AddDate(0, -1, 0)

	// High engagement source: 50 visitors, 150 page views (3 pages/visit)
	db.Exec(`INSERT INTO ref_stats (website_id, hostname, visitors_count, page_views_count, hour) VALUES (1, 'news.ycombinator.com', 50, 150, ?)`, firstOfLastMonth.Add(12*time.Hour))

	// Low engagement source: 100 visitors, 110 page views (1.1 pages/visit)
	db.Exec(`INSERT INTO ref_stats (website_id, hostname, visitors_count, page_views_count, hour) VALUES (1, 'twitter.com', 100, 110, ?)`, firstOfLastMonth.Add(12*time.Hour))

	detector := feed.NewDetector(db, testLogger())
	err := detector.DetectForWebsite(1)
	require.NoError(t, err)

	var items []feed.FeedItem
	db.Where("website_id = ? AND item_type = ?", 1, feed.ItemTypeBestSources).Find(&items)

	assert.Len(t, items, 1)
	assert.Equal(t, "Engaged readers", items[0].Title)
	assert.Contains(t, items[0].Description, "pages")

	metadata := items[0].MetadataMap()
	sources := metadata["sources"].([]any)
	assert.GreaterOrEqual(t, len(sources), 1)

	// First source should be the one with highest pages per visit
	firstSource := sources[0].(map[string]any)
	assert.Equal(t, "news.ycombinator.com", firstSource["hostname"])
	assert.Equal(t, float64(3.0), firstSource["pagesPerVisit"])
}

func TestDetector_BestSources_SkipsLowEngagement(t *testing.T) {
	db := setupTestDB(t)

	db.Exec("INSERT INTO websites (id, domain) VALUES (1, 'test.com')")

	now := time.Now().UTC()
	firstOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	firstOfLastMonth := firstOfThisMonth.AddDate(0, -1, 0)

	// Source with visitors but low engagement (1.5 pages/visit, below 3.0 threshold)
	db.Exec(`INSERT INTO ref_stats (website_id, hostname, visitors_count, page_views_count, hour) VALUES (1, 'low-engagement.com', 10, 15, ?)`, firstOfLastMonth.Add(12*time.Hour))

	detector := feed.NewDetector(db, testLogger())
	err := detector.DetectForWebsite(1)
	require.NoError(t, err)

	var count int64
	db.Model(&feed.FeedItem{}).Where("website_id = ? AND item_type = ?", 1, feed.ItemTypeBestSources).Count(&count)
	assert.Equal(t, int64(0), count)
}
