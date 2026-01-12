package analytics_test

import (
	"fusionaly/internal/analytics"
	"fusionaly/internal/timeframe"

	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/events"
	"fusionaly/internal/websites"
	"fusionaly/internal/testsupport"
)

// TestDBManager is a simple test implementation of the DBManager interface

// setupTimeFrame creates a standard time frame for tests
func setupTimeFrame(t *testing.T) *timeframe.TimeFrame {
	timeFrame, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
		FromTime:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
		ToTime:        time.Date(2024, 7, 2, 0, 0, 0, 0, time.UTC),
		TimeFrameSize: timeframe.DailyTimeFrame,
	}, time.Local)
	require.NoError(t, err)
	return timeFrame
}

// Testanalytics.GetVisitDurationInTimeFrame tests the average session duration calculation
func TestGetVisitDurationInTimeFrame(t *testing.T) {
	// Create a test DBManager
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	// Create test website
	testsupport.CreateTestWebsite(db, "example.com")
	websiteID, err := websites.GetWebsiteOrNotFound(db, "example.com")
	require.NoError(t, err)

	// Create events for two sessions
	// Session 1: 20 minutes
	testsupport.CreateEvent(t, dbManager, websiteID, "visitor1", "/page1", time.Date(2024, 7, 1, 10, 0, 0, 0, time.UTC))
	testsupport.CreateEvent(t, dbManager, websiteID, "visitor1", "/page2", time.Date(2024, 7, 1, 10, 10, 0, 0, time.UTC))
	testsupport.CreateEvent(t, dbManager, websiteID, "visitor1", "/page3", time.Date(2024, 7, 1, 10, 20, 0, 0, time.UTC))

	// Session 2: 20 minutes
	testsupport.CreateEvent(t, dbManager, websiteID, "visitor2", "/page1", time.Date(2024, 7, 1, 11, 0, 0, 0, time.UTC))
	testsupport.CreateEvent(t, dbManager, websiteID, "visitor2", "/page2", time.Date(2024, 7, 1, 11, 10, 0, 0, time.UTC))
	testsupport.CreateEvent(t, dbManager, websiteID, "visitor2", "/page3", time.Date(2024, 7, 1, 11, 20, 0, 0, time.UTC))

	// Set up time frame
	timeFrame := setupTimeFrame(t)
	queryParams := analytics.NewWebsiteScopedQueryParams(timeFrame, int(websiteID))

	// Set limit to ensure we get all results in tests
	queryParams.Limit = 10

	// Get the visit duration
	duration, err := analytics.GetVisitDurationInTimeFrame(db, queryParams)
	require.NoError(t, err)

	// Two sessions of 20 minutes each = average of 1200 seconds
	expectedDuration := float64(1200)
	assert.InDelta(t, expectedDuration, duration, 1.0,
		"Expected average duration of %.2f seconds, got %.2f", expectedDuration, duration)
}

// TestMetricsFromAggregationTables tests metrics that use aggregation tables
func TestMetricsFromAggregationTables(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()

	// Create website entry (needed for referrer self-filtering)
	testsupport.CreateTestWebsite(db, "example.com")

	// Create time frame for all tests
	timeFrame := setupTimeFrame(t)
	websiteID := 1
	queryParams := analytics.NewWebsiteScopedQueryParams(timeFrame, websiteID)

	// Ensure limit is set to allow for all test data (we only have 2 items per category in the test)
	queryParams.Limit = 10

	// Create test data for site_stats table
	siteStat := analytics.SiteStat{
		WebsiteID:   1,
		PageViews:   100,
		Visitors:    50,
		Sessions:    60,
		BounceCount: 30,
		Hour:        time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	require.NoError(t, db.Create(&siteStat).Error)

	// Create test data for page_stats table
	pageStats := []analytics.PageStat{
		{
			WebsiteID:      1,
			Hostname:       "example.com",
			Pathname:       "/home",
			PageViewsCount: 60,
			VisitorsCount:  40,
			Entrances:      40,
			Exits:          20,
			Hour:           time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			WebsiteID:      1,
			Hostname:       "example.com",
			Pathname:       "/about",
			PageViewsCount: 40,
			VisitorsCount:  30,
			Entrances:      20,
			Exits:          40,
			Hour:           time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}
	require.NoError(t, db.CreateInBatches(pageStats, len(pageStats)).Error)

	// Create test data for ref_stats table
	refStats := []analytics.RefStat{
		{
			WebsiteID:      1,
			Hostname:       "google.com",
			Pathname:       "/search",
			VisitorsCount:  30,
			PageViewsCount: 50,
			Hour:           time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			WebsiteID:      1,
			Hostname:       "twitter.com",
			Pathname:       "",
			VisitorsCount:  20,
			PageViewsCount: 30,
			Hour:           time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}
	require.NoError(t, db.CreateInBatches(refStats, len(refStats)).Error)

	// Create test data for browser_stats table
	browserStats := []analytics.BrowserStat{
		{
			WebsiteID:      1,
			Browser:        "chrome",
			VisitorsCount:  35,
			PageViewsCount: 60,
			Hour:           time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			WebsiteID:      1,
			Browser:        "firefox",
			VisitorsCount:  15,
			PageViewsCount: 40,
			Hour:           time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}
	require.NoError(t, db.CreateInBatches(browserStats, len(browserStats)).Error)

	// Create test data for os_stats table
	osStats := []analytics.OSStat{
		{
			WebsiteID:       1,
			OperatingSystem: "windows",
			VisitorsCount:   30,
			PageViewsCount:  50,
			Hour:            time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
		{
			WebsiteID:       1,
			OperatingSystem: "macos",
			VisitorsCount:   20,
			PageViewsCount:  50,
			Hour:            time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
	}
	require.NoError(t, db.CreateInBatches(osStats, len(osStats)).Error)

	// Create test data for device_stats table
	deviceStats := []analytics.DeviceStat{
		{
			WebsiteID:      1,
			DeviceType:     "desktop",
			VisitorsCount:  35,
			PageViewsCount: 70,
			Hour:           time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			WebsiteID:      1,
			DeviceType:     "mobile",
			VisitorsCount:  15,
			PageViewsCount: 30,
			Hour:           time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}
	require.NoError(t, db.CreateInBatches(deviceStats, len(deviceStats)).Error)

	// Create test data for country_stats table
	countryStats := []analytics.CountryStat{
		{
			WebsiteID:      1,
			Country:        "us",
			VisitorsCount:  30,
			PageViewsCount: 50,
			Hour:           time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			WebsiteID:      1,
			Country:        "uk",
			VisitorsCount:  20,
			PageViewsCount: 50,
			Hour:           time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}
	require.NoError(t, db.CreateInBatches(countryStats, len(countryStats)).Error)

	// Create test data for utm_stats table
	utmStats := []analytics.UTMStat{
		{
			WebsiteID:      1,
			UTMSource:      "google",
			UTMMedium:      "cpc",
			UTMCampaign:    "spring_sale",
			UTMTerm:        "analytics",
			UTMContent:     "banner1",
			VisitorsCount:  30,
			PageViewsCount: 50,
			Hour:           time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			WebsiteID:      1,
			UTMSource:      "facebook",
			UTMMedium:      "social",
			UTMCampaign:    "product_launch",
			UTMTerm:        "",
			UTMContent:     "",
			VisitorsCount:  20,
			PageViewsCount: 40,
			Hour:           time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}
	require.NoError(t, db.CreateInBatches(utmStats, len(utmStats)).Error)

	// Create test data for event_stats table
	eventStats := []analytics.EventStat{
		{
			WebsiteID:      1,
			EventName:      "button_click",
			EventKey:       "signup_button",
			VisitorsCount:  25,
			PageViewsCount: 30,
			Hour:           time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			WebsiteID:      1,
			EventName:      "form_submit",
			EventKey:       "contact_form",
			VisitorsCount:  15,
			PageViewsCount: 15,
			Hour:           time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}
	require.NoError(t, db.CreateInBatches(eventStats, len(eventStats)).Error)

	// Run tests for each metric function
	t.Run("BounceRate", func(t *testing.T) {
		bounceRate, err := analytics.GetBounceRateInTimeFrame(db, queryParams)
		require.NoError(t, err)
		assert.InDelta(t, 0.5, bounceRate, 0.01) // 30 bounces / 60 sessions = 0.5
	})

	t.Run("TopURLs", func(t *testing.T) {
		results, err := analytics.GetTopURLsInTimeFrame(db, queryParams)
		require.NoError(t, err)
		require.Len(t, results, 2)
		assert.Equal(t, "example.com/home", results[0].Name)
		assert.Equal(t, int64(40), results[0].Count)
		assert.Equal(t, "example.com/about", results[1].Name)
		assert.Equal(t, int64(30), results[1].Count)
	})

	t.Run("TopBrowsers", func(t *testing.T) {
		results, err := analytics.GetTopBrowsersInTimeFrame(db, queryParams)
		require.NoError(t, err)
		require.Len(t, results, 2)
		assert.Equal(t, "chrome", results[0].Name)
		assert.Equal(t, int64(35), results[0].Count)
		assert.Equal(t, "firefox", results[1].Name)
		assert.Equal(t, int64(15), results[1].Count)
	})

	t.Run("TopOS", func(t *testing.T) {
		results, err := analytics.GetTopOsInTimeFrame(db, queryParams)
		require.NoError(t, err)
		require.Len(t, results, 2)
		assert.Equal(t, "Windows", results[0].Name)
		assert.Equal(t, int64(30), results[0].Count)
		assert.Equal(t, "MacOS", results[1].Name)
		assert.Equal(t, int64(20), results[1].Count)
	})

	t.Run("TopDeviceTypes", func(t *testing.T) {
		results, err := analytics.GetTopDeviceTypesInTimeFrame(db, queryParams)
		require.NoError(t, err)
		require.Len(t, results, 2)
		assert.Equal(t, "desktop", results[0].Name)
		assert.Equal(t, int64(35), results[0].Count)
		assert.Equal(t, "mobile", results[1].Name)
		assert.Equal(t, int64(15), results[1].Count)
	})

	t.Run("TopCountries", func(t *testing.T) {
		results, err := analytics.GetTopCountriesInTimeFrame(db, queryParams)
		require.NoError(t, err)
		require.Len(t, results, 2)
		assert.Equal(t, "us", results[0].Name)
		assert.Equal(t, int64(30), results[0].Count)
		assert.Equal(t, "uk", results[1].Name)
		assert.Equal(t, int64(20), results[1].Count)
	})

	t.Run("TopReferrers", func(t *testing.T) {
		topReferrers, err := analytics.GetTopReferrersInTimeFrame(db, queryParams)
		require.NoError(t, err)
		require.Len(t, topReferrers, 2)
		assert.Equal(t, "Google", topReferrers[0].Name) // google.com (normalized to Google) has 30 visitors
		assert.Equal(t, int64(30), topReferrers[0].Count)
		assert.Equal(t, "Twitter", topReferrers[1].Name) // twitter.com (normalized to Twitter) has 20 visitors
		assert.Equal(t, int64(20), topReferrers[1].Count)
	})

	t.Run("TopCustomEvents", func(t *testing.T) {
		results, err := analytics.GetTopCustomEventsInTimeFrame(db, queryParams)
		require.NoError(t, err)
		require.Len(t, results, 2)
		assert.Equal(t, "signup_button", results[0].Name)
		assert.Equal(t, int64(25), results[0].Count)
		assert.Equal(t, "contact_form", results[1].Name)
		assert.Equal(t, int64(15), results[1].Count)
	})

	t.Run("TopEntryPages", func(t *testing.T) {
		results, err := analytics.GetTopEntryPagesInTimeFrame(db, queryParams)
		require.NoError(t, err)
		require.Len(t, results, 2)
		assert.Equal(t, "example.com/home", results[0].Name)
		assert.Equal(t, int64(40), results[0].Count)
		assert.Equal(t, "example.com/about", results[1].Name)
		assert.Equal(t, int64(20), results[1].Count)
	})

	t.Run("TopExitPages", func(t *testing.T) {
		results, err := analytics.GetTopExitPagesInTimeFrame(db, queryParams)
		require.NoError(t, err)
		require.Len(t, results, 2)
		assert.Equal(t, "example.com/about", results[0].Name)
		assert.Equal(t, int64(40), results[0].Count)
		assert.Equal(t, "example.com/home", results[1].Name)
		assert.Equal(t, int64(20), results[1].Count)
	})

	t.Run("TopUTMSources", func(t *testing.T) {
		results, err := analytics.GetTopUTMSourcesInTimeFrame(db, queryParams)
		require.NoError(t, err)
		require.Len(t, results, 2)
		assert.Equal(t, "google", results[0].Name)
		assert.Equal(t, int64(30), results[0].Count)
		assert.Equal(t, "facebook", results[1].Name)
		assert.Equal(t, int64(20), results[1].Count)
	})

	t.Run("TopUTMMediums", func(t *testing.T) {
		results, err := analytics.GetTopUTMMediumsInTimeFrame(db, queryParams)
		require.NoError(t, err)
		require.Len(t, results, 2)
		assert.Equal(t, "cpc", results[0].Name)
		assert.Equal(t, int64(30), results[0].Count)
		assert.Equal(t, "social", results[1].Name)
		assert.Equal(t, int64(20), results[1].Count)
	})

	t.Run("TopUTMCampaigns", func(t *testing.T) {
		results, err := analytics.GetTopUTMCampaignsInTimeFrame(db, queryParams)
		require.NoError(t, err)
		require.Len(t, results, 2)
		assert.Equal(t, "spring_sale", results[0].Name)
		assert.Equal(t, int64(30), results[0].Count)
		assert.Equal(t, "product_launch", results[1].Name)
		assert.Equal(t, int64(20), results[1].Count)
	})
}

func TestTotalCounts(t *testing.T) {
	// Create a test DBManager
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	testsupport.CleanAllAggregates(db)

	// Setup data via direct aggregation inserts for controlled testing
	// Create site_stats entries for page views, visitors, sessions
	siteStats := []analytics.SiteStat{
		{
			WebsiteID: 1,
			PageViews: 30,
			Visitors:  25,
			Sessions:  20,
			Hour:      time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
		},
		{
			WebsiteID: 1,
			PageViews: 40,
			Visitors:  35,
			Sessions:  30,
			Hour:      time.Date(2024, 7, 1, 13, 0, 0, 0, time.UTC),
		},
	}
	db.CreateInBatches(siteStats, len(siteStats))

	// Create page_stats entries for entry and exit pages
	pageStats := []analytics.PageStat{
		{
			WebsiteID: 1,
			Hostname:  "example.com",
			Pathname:  "/home",
			Entrances: 25,
			Exits:     15,
			Hour:      time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
		},
		{
			WebsiteID: 1,
			Hostname:  "example.com",
			Pathname:  "/about",
			Entrances: 15,
			Exits:     25,
			Hour:      time.Date(2024, 7, 1, 13, 0, 0, 0, time.UTC),
		},
	}
	db.CreateInBatches(pageStats, len(pageStats))

	// Create event_stats entries for custom events
	eventStats := []analytics.EventStat{
		{
			WebsiteID:     1,
			EventKey:      "add_to_cart",
			VisitorsCount: 20,
			Hour:          time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
		},
		{
			WebsiteID:     1,
			EventKey:      "checkout",
			VisitorsCount: 10,
			Hour:          time.Date(2024, 7, 1, 13, 0, 0, 0, time.UTC),
		},
	}
	db.CreateInBatches(eventStats, len(eventStats))

	// Set up time frame
	timeFrame, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
		FromTime:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
		ToTime:        time.Date(2024, 7, 1, 23, 59, 59, 0, time.UTC),
		TimeFrameSize: timeframe.DailyTimeFrame,
	}, time.UTC)
	require.NoError(t, err)

	queryParams := analytics.NewWebsiteScopedQueryParams(timeFrame, 1)

	// Test total page views
	t.Run("TotalPageViews", func(t *testing.T) {
		totalViews, err := analytics.GetTotalPageViewsInTimeFrame(db, queryParams)
		require.NoError(t, err)
		assert.Equal(t, int64(70), totalViews, "Expected 70 total page views (30+40)")
	})

	// Test total visitors
	t.Run("TotalVisitors", func(t *testing.T) {
		totalVisitors, err := analytics.GetTotalVisitorsInTimeFrame(db, queryParams)
		require.NoError(t, err)
		assert.Equal(t, int64(60), totalVisitors, "Expected 60 total visitors (25+35)")
	})

	// Test total sessions
	t.Run("TotalSessions", func(t *testing.T) {
		totalSessions, err := analytics.GetTotalSessionsInTimeFrame(db, queryParams)
		require.NoError(t, err)
		assert.Equal(t, int64(50), totalSessions, "Expected 50 total sessions (20+30)")
	})

	// Test total entries
	t.Run("TotalEntryCount", func(t *testing.T) {
		totalEntries, err := analytics.GetTotalEntryCountInTimeFrame(db, queryParams)
		require.NoError(t, err)
		assert.Equal(t, int64(40), totalEntries, "Expected 40 total entries (25+15)")
	})

	// Test total exits
	t.Run("TotalExitCount", func(t *testing.T) {
		totalExits, err := analytics.GetTotalExitCountInTimeFrame(db, queryParams)
		require.NoError(t, err)
		assert.Equal(t, int64(40), totalExits, "Expected 40 total exits (15+25)")
	})

	// Test total custom events
	t.Run("TotalCustomEvents", func(t *testing.T) {
		totalEvents, err := analytics.GetTotalCustomEventsInTimeFrame(db, queryParams)
		require.NoError(t, err)
		assert.Equal(t, int64(30), totalEvents, "Expected 30 total custom events (20+10)")
	})
}

// TestAggregatedGoalConversionsInTimeFrame tests the goal conversion aggregation function
func TestAggregatedGoalConversionsInTimeFrame(t *testing.T) {
	// Create a test DBManager
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()

	// Create test website
	testsupport.CreateTestWebsite(db, "example.com")
	websiteID, err := websites.GetWebsiteOrNotFound(db, "example.com")
	require.NoError(t, err)

	// Set up time frame for testing (2 days to test time series)
	timeFrame, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
		FromTime:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
		ToTime:        time.Date(2024, 7, 2, 23, 59, 59, 0, time.UTC),
		TimeFrameSize: timeframe.DailyTimeFrame,
	}, time.Local)
	require.NoError(t, err)

	queryParams := analytics.NewWebsiteScopedQueryParams(timeFrame, int(websiteID))

	// Clear any existing event stats to avoid unique constraint issues
	require.NoError(t, db.Exec("DELETE FROM event_stats").Error)

	// Create test event stats for goal conversions
	eventStats := []analytics.EventStat{
		{
			WebsiteID:      websiteID,
			EventName:      "newsletter_signup",
			EventKey:       "newsletter_signup",
			VisitorsCount:  5,
			PageViewsCount: 7,
			Hour:           time.Date(2024, 7, 1, 10, 0, 0, 0, time.UTC),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			WebsiteID:      websiteID,
			EventName:      "purchase_completed",
			EventKey:       "purchase_completed",
			VisitorsCount:  3,
			PageViewsCount: 3,
			Hour:           time.Date(2024, 7, 1, 15, 0, 0, 0, time.UTC),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			WebsiteID:      websiteID,
			EventName:      "newsletter_signup",
			EventKey:       "newsletter_signup",
			VisitorsCount:  8,
			PageViewsCount: 12,
			Hour:           time.Date(2024, 7, 2, 11, 0, 0, 0, time.UTC),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			WebsiteID:      websiteID,
			EventName:      "demo_requested",
			EventKey:       "demo_requested",
			VisitorsCount:  2,
			PageViewsCount: 2,
			Hour:           time.Date(2024, 7, 2, 14, 0, 0, 0, time.UTC),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			WebsiteID:      websiteID,
			EventName:      "non_goal_event",
			EventKey:       "non_goal_event",
			VisitorsCount:  10,
			PageViewsCount: 15,
			Hour:           time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}

	// Create event stats in the database
	for _, stat := range eventStats {
		require.NoError(t, db.Create(&stat).Error)
	}

	t.Run("WithValidGoals", func(t *testing.T) {
		// Test with valid goal names
		conversionGoals := []string{"newsletter_signup", "purchase_completed", "demo_requested"}

		result, err := analytics.AggregatedGoalConversionsInTimeFrame(db, queryParams, conversionGoals)
		require.NoError(t, err)

		// Should return time series data for the time frame
		assert.Len(t, result, 2, "Should return 2 days of data")

		// Check first day (July 1, 2024) - should have newsletter_signup (5) + purchase_completed (3) = 8
		assert.Equal(t, "2024-07-01T00:00:00Z", result[0].Date)
		assert.Equal(t, 8, result[0].Count, "First day should have 8 goal conversions")

		// Check second day (July 2, 2024) - should have newsletter_signup (8) + demo_requested (2) = 10
		assert.Equal(t, "2024-07-02T00:00:00Z", result[1].Date)
		assert.Equal(t, 10, result[1].Count, "Second day should have 10 goal conversions")
	})

	t.Run("WithEmptyGoals", func(t *testing.T) {
		// Test with empty goal list
		conversionGoals := []string{}

		result, err := analytics.AggregatedGoalConversionsInTimeFrame(db, queryParams, conversionGoals)
		require.NoError(t, err)

		// Should return empty time series (zeros)
		assert.Len(t, result, 2, "Should return 2 days of data")
		assert.Equal(t, 0, result[0].Count, "First day should have 0 goal conversions")
		assert.Equal(t, 0, result[1].Count, "Second day should have 0 goal conversions")
	})

	t.Run("WithNonExistentGoals", func(t *testing.T) {
		// Test with goal names that don't exist
		conversionGoals := []string{"nonexistent_goal", "another_fake_goal"}

		result, err := analytics.AggregatedGoalConversionsInTimeFrame(db, queryParams, conversionGoals)
		require.NoError(t, err)

		// Should return empty time series (zeros)
		assert.Len(t, result, 2, "Should return 2 days of data")
		assert.Equal(t, 0, result[0].Count, "First day should have 0 goal conversions")
		assert.Equal(t, 0, result[1].Count, "Second day should have 0 goal conversions")
	})

	t.Run("WithPartialMatchingGoals", func(t *testing.T) {
		// Test with mix of existing and non-existing goals
		conversionGoals := []string{"newsletter_signup", "nonexistent_goal", "purchase_completed"}

		result, err := analytics.AggregatedGoalConversionsInTimeFrame(db, queryParams, conversionGoals)
		require.NoError(t, err)

		// Should return data only for existing goals
		assert.Len(t, result, 2, "Should return 2 days of data")
		assert.Equal(t, 8, result[0].Count, "First day should have 8 goal conversions (newsletter + purchase)")
		assert.Equal(t, 8, result[1].Count, "Second day should have 8 goal conversions (newsletter only)")
	})

	t.Run("WithSingleGoal", func(t *testing.T) {
		// Test with single goal
		conversionGoals := []string{"newsletter_signup"}

		result, err := analytics.AggregatedGoalConversionsInTimeFrame(db, queryParams, conversionGoals)
		require.NoError(t, err)

		// Should return data only for newsletter signup
		assert.Len(t, result, 2, "Should return 2 days of data")
		assert.Equal(t, 5, result[0].Count, "First day should have 5 newsletter signups")
		assert.Equal(t, 8, result[1].Count, "Second day should have 8 newsletter signups")
	})

	t.Run("WithDifferentWebsite", func(t *testing.T) {
		// Create another website to test isolation
		// Create the website directly in the database with a unique domain
		uniqueDomain := fmt.Sprintf("test-%d.com", time.Now().UnixNano())
		otherWebsite := websites.Website{
			Domain:    uniqueDomain,
			CreatedAt: time.Now(),
		}
		require.NoError(t, db.Create(&otherWebsite).Error)

		// Test with different website ID
		otherQueryParams := analytics.NewWebsiteScopedQueryParams(timeFrame, int(otherWebsite.ID))
		conversionGoals := []string{"newsletter_signup", "purchase_completed"}

		result, err := analytics.AggregatedGoalConversionsInTimeFrame(db, otherQueryParams, conversionGoals)
		require.NoError(t, err)

		// Should return zeros for different website
		assert.Len(t, result, 2, "Should return 2 days of data")
		assert.Equal(t, 0, result[0].Count, "First day should have 0 goal conversions for other website")
		assert.Equal(t, 0, result[1].Count, "Second day should have 0 goal conversions for other website")
	})
}

// TestAggregatedRevenueInTimeFrame tests the revenue aggregation function
func TestAggregatedRevenueInTimeFrame(t *testing.T) {
	// Create a test DBManager
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()

	// Create test website
	testsupport.CreateTestWebsite(db, "example.com")
	websiteID, err := websites.GetWebsiteOrNotFound(db, "example.com")
	require.NoError(t, err)

	// Set up time frame for testing (2 days to test time series)
	timeFrame, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
		FromTime:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
		ToTime:        time.Date(2024, 7, 2, 23, 59, 59, 0, time.UTC),
		TimeFrameSize: timeframe.DailyTimeFrame,
	}, time.Local)
	require.NoError(t, err)

	queryParams := analytics.NewWebsiteScopedQueryParams(timeFrame, int(websiteID))

	// Clear any existing events to avoid conflicts
	require.NoError(t, db.Exec("DELETE FROM events").Error)

	// Create test events for revenue:purchased events with revenue metadata
	testEvents := []events.Event{
		{
			WebsiteID:       websiteID,
			UserSignature:   "user1",
			Hostname:        "example.com",
			Pathname:        "/pricing",
			EventType:       events.EventTypeCustomEvent,
			CustomEventName: "revenue:purchased",
			CustomEventMeta: `{"price": 2999, "currency": "USD"}`, // $29.99
			Timestamp:       time.Date(2024, 7, 1, 10, 0, 0, 0, time.UTC),
			CreatedAt:       time.Now(),
		},
		{
			WebsiteID:       websiteID,
			UserSignature:   "user2",
			Hostname:        "example.com",
			Pathname:        "/pricing",
			EventType:       events.EventTypeCustomEvent,
			CustomEventName: "revenue:purchased",
			CustomEventMeta: `{"price": 4999, "currency": "USD"}`, // $49.99
			Timestamp:       time.Date(2024, 7, 1, 14, 0, 0, 0, time.UTC),
			CreatedAt:       time.Now(),
		},
		{
			WebsiteID:       websiteID,
			UserSignature:   "user3",
			Hostname:        "example.com",
			Pathname:        "/pricing",
			EventType:       events.EventTypeCustomEvent,
			CustomEventName: "revenue:purchased",
			CustomEventMeta: `{"price": 1999, "currency": "USD"}`, // $19.99
			Timestamp:       time.Date(2024, 7, 2, 9, 0, 0, 0, time.UTC),
			CreatedAt:       time.Now(),
		},
		{
			WebsiteID:       websiteID,
			UserSignature:   "user4",
			Hostname:        "example.com",
			Pathname:        "/home",
			EventType:       events.EventTypeCustomEvent,
			CustomEventName: "other_event",
			CustomEventMeta: `{"price": 999}`, // Should be ignored (not revenue:purchased)
			Timestamp:       time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
			CreatedAt:       time.Now(),
		},
	}

	// Create events
	for _, event := range testEvents {
		require.NoError(t, db.Create(&event).Error)
	}

	// Test 1: Valid revenue aggregation
	result, err := analytics.AggregatedRevenueInTimeFrame(db, queryParams)
	require.NoError(t, err)
	require.Len(t, result, 2) // Should have 2 days

	// Day 1: $29.99 + $49.99 = $79.98 = 7998 cents
	assert.Equal(t, "2024-07-01T00:00:00Z", result[0].Date)
	assert.Equal(t, int(7998), result[0].Count)

	// Day 2: $19.99 = 1999 cents
	assert.Equal(t, "2024-07-02T00:00:00Z", result[1].Date)
	assert.Equal(t, int(1999), result[1].Count)

	// Test 2: No revenue events should return zeros
	// Clear all revenue events
	require.NoError(t, db.Exec("DELETE FROM events WHERE custom_event_name = 'revenue:purchased'").Error)

	result, err = analytics.AggregatedRevenueInTimeFrame(db, queryParams)
	require.NoError(t, err)
	require.Len(t, result, 2) // Should still have 2 days but with zero revenue

	assert.Equal(t, "2024-07-01T00:00:00Z", result[0].Date)
	assert.Equal(t, int(0), result[0].Count)
	assert.Equal(t, "2024-07-02T00:00:00Z", result[1].Date)
	assert.Equal(t, int(0), result[1].Count)

	// Test 3: Invalid JSON metadata should be ignored
	invalidEvent := events.Event{
		WebsiteID:       websiteID,
		UserSignature:   "user_invalid",
		Hostname:        "example.com",
		Pathname:        "/pricing",
		EventType:       events.EventTypeCustomEvent,
		CustomEventName: "revenue:purchased",
		CustomEventMeta: `{"invalid": "json"`, // Invalid JSON
		Timestamp:       time.Date(2024, 7, 1, 15, 0, 0, 0, time.UTC),
		CreatedAt:       time.Now(),
	}
	require.NoError(t, db.Create(&invalidEvent).Error)

	result, err = analytics.AggregatedRevenueInTimeFrame(db, queryParams)
	require.NoError(t, err)
	require.Len(t, result, 2)

	// Should still be zero since invalid metadata is ignored
	assert.Equal(t, int(0), result[0].Count)
	assert.Equal(t, int(0), result[1].Count)

	// Test 4: Website isolation - revenue from different website should not be included
	testsupport.CreateTestWebsite(db, "other.com")
	otherWebsiteID, err := websites.GetWebsiteOrNotFound(db, "other.com")
	require.NoError(t, err)

	otherWebsiteEvent := events.Event{
		WebsiteID:       otherWebsiteID,
		UserSignature:   "user_other",
		Hostname:        "other.com",
		Pathname:        "/pricing",
		EventType:       events.EventTypeCustomEvent,
		CustomEventName: "revenue:purchased",
		CustomEventMeta: `{"price": 5999, "currency": "USD"}`, // $59.99
		Timestamp:       time.Date(2024, 7, 1, 16, 0, 0, 0, time.UTC),
		CreatedAt:       time.Now(),
	}
	require.NoError(t, db.Create(&otherWebsiteEvent).Error)

	result, err = analytics.AggregatedRevenueInTimeFrame(db, queryParams)
	require.NoError(t, err)
	require.Len(t, result, 2)

	// Should still be zero since other website's revenue is excluded
	assert.Equal(t, int(0), result[0].Count)
	assert.Equal(t, int(0), result[1].Count)
}

func TestGetEventRevenueTotals(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()

	testsupport.CreateTestWebsite(db, "example.com")
	websiteID, err := websites.GetWebsiteOrNotFound(db, "example.com")
	require.NoError(t, err)

	require.NoError(t, db.Exec("DELETE FROM events").Error)

	timeFrame := setupTimeFrame(t)
	queryParams := analytics.NewWebsiteScopedQueryParams(timeFrame, int(websiteID))

	baseTime := time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC)

	testEvents := []events.Event{
		{
			WebsiteID:       websiteID,
			UserSignature:   "user-1",
			Hostname:        "example.com",
			Pathname:        "/checkout",
			EventType:       events.EventTypeCustomEvent,
			CustomEventName: "revenue:purchased",
			CustomEventMeta: `{"price": 2500, "quantity": 2, "currency": "USD"}`, // $50.00
			Timestamp:       baseTime,
			CreatedAt:       time.Now(),
		},
		{
			WebsiteID:       websiteID,
			UserSignature:   "user-2",
			Hostname:        "example.com",
			Pathname:        "/checkout",
			EventType:       events.EventTypeCustomEvent,
			CustomEventName: "revenue:purchased",
			CustomEventMeta: `{"price": 2500, "currency": "USD"}`, // $25.00
			Timestamp:       baseTime.Add(1 * time.Hour),
			CreatedAt:       time.Now(),
		},
		{
			WebsiteID:       websiteID,
			UserSignature:   "user-3",
			Hostname:        "example.com",
			Pathname:        "/upsell",
			EventType:       events.EventTypeCustomEvent,
			CustomEventName: "upsell:purchased",
			CustomEventMeta: `{"price": 1500, "currency": "USD"}`, // $15.00
			Timestamp:       baseTime.Add(2 * time.Hour),
			CreatedAt:       time.Now(),
		},
		{
			WebsiteID:       websiteID,
			UserSignature:   "user-4",
			Hostname:        "example.com",
			Pathname:        "/checkout",
			EventType:       events.EventTypeCustomEvent,
			CustomEventName: "revenue:purchased",
			CustomEventMeta: `{"quantity": 3}`, // Missing price, should be ignored
			Timestamp:       baseTime.Add(3 * time.Hour),
			CreatedAt:       time.Now(),
		},
		{
			WebsiteID:       websiteID,
			UserSignature:   "user-5",
			Hostname:        "example.com",
			Pathname:        "/checkout",
			EventType:       events.EventTypeCustomEvent,
			CustomEventName: "revenue:purchased",
			CustomEventMeta: `{"price": 2900}`,
			Timestamp:       baseTime.AddDate(0, 0, 2), // Outside timeframe
			CreatedAt:       time.Now(),
		},
	}

	for _, event := range testEvents {
		require.NoError(t, db.Create(&event).Error)
	}

	// Event for another website should be excluded
	otherWebsite := websites.Website{Domain: "othersite.com", CreatedAt: time.Now()}
	require.NoError(t, db.Create(&otherWebsite).Error)
	require.NoError(t, db.Create(&events.Event{
		WebsiteID:       otherWebsite.ID,
		UserSignature:   "other-user",
		Hostname:        "othersite.com",
		Pathname:        "/checkout",
		EventType:       events.EventTypeCustomEvent,
		CustomEventName: "revenue:purchased",
		CustomEventMeta: `{"price": 9999, "currency": "USD"}`,
		Timestamp:       baseTime,
		CreatedAt:       time.Now(),
	}).Error)

	totals, err := analytics.GetEventRevenueTotals(db, queryParams)
	require.NoError(t, err)

	require.Len(t, totals, 2)
	assert.InDelta(t, 75.0, totals["revenue:purchased"], 0.01)
	assert.InDelta(t, 15.0, totals["upsell:purchased"], 0.01)
	assert.NotContains(t, totals, "revenue:ignored")
}

func TestGetEventRevenueTotalsEmpty(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()

	testsupport.CreateTestWebsite(db, "empty.com")
	websiteID, err := websites.GetWebsiteOrNotFound(db, "empty.com")
	require.NoError(t, err)

	require.NoError(t, db.Exec("DELETE FROM events").Error)

	timeFrame := setupTimeFrame(t)
	queryParams := analytics.NewWebsiteScopedQueryParams(timeFrame, int(websiteID))

	totals, err := analytics.GetEventRevenueTotals(db, queryParams)
	require.NoError(t, err)
	assert.Empty(t, totals)
}
