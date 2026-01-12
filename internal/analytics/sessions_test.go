package analytics_test

import (
	"fusionaly/internal/analytics"
	"fusionaly/internal/timeframe"

	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/testsupport"
)

func TestAggregatedSessions(t *testing.T) {
	tests := []struct {
		name           string
		setup          func() // Function to set up site_stats data
		timeFrameSize  timeframe.TimeFrameSize
		fromTime       time.Time
		toTime         time.Time
		expectedCounts map[string]int // Map date string to expected count
	}{
		{
			name: "Daily Format",
			setup: func() {
				dbManager, _ := testsupport.SetupTestDBManager(t)
				db := dbManager.GetConnection()
				// Create site_stats entries for testing daily format
				siteStats := []analytics.SiteStat{
					{
						WebsiteID: 1,
						Sessions:  2,
						Hour:      time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
					},
					{
						WebsiteID: 1,
						Sessions:  2,
						Hour:      time.Date(2024, 7, 1, 15, 0, 0, 0, time.UTC),
					},
					{
						WebsiteID: 1,
						Sessions:  1,
						Hour:      time.Date(2024, 7, 2, 10, 0, 0, 0, time.UTC),
					},
					{
						WebsiteID: 1,
						Sessions:  2,
						Hour:      time.Date(2024, 7, 2, 14, 0, 0, 0, time.UTC),
					},
				}
				db.CreateInBatches(siteStats, len(siteStats))
			},
			timeFrameSize: timeframe.DailyTimeFrame,
			fromTime:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
			toTime:        time.Date(2024, 7, 2, 23, 59, 59, 0, time.UTC),
			expectedCounts: map[string]int{
				"2024-07-01T00:00:00Z": 4, // 2 + 2
				"2024-07-02T00:00:00Z": 3, // 1 + 2
			},
		},
		{
			name: "Hourly Format",
			setup: func() {
				dbManager, _ := testsupport.SetupTestDBManager(t)
				db := dbManager.GetConnection()
				// Create site_stats entries for testing hourly format
				siteStats := []analytics.SiteStat{
					{
						WebsiteID: 1,
						Sessions:  2,
						Hour:      time.Date(2024, 7, 1, 10, 0, 0, 0, time.UTC),
					},
					{
						WebsiteID: 1,
						Sessions:  1,
						Hour:      time.Date(2024, 7, 1, 11, 0, 0, 0, time.UTC),
					},
					{
						WebsiteID: 1,
						Sessions:  1,
						Hour:      time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
					},
				}
				db.CreateInBatches(siteStats, len(siteStats))
			},
			timeFrameSize: timeframe.HourlyTimeFrame,
			fromTime:      time.Date(2024, 7, 1, 10, 0, 0, 0, time.UTC),
			toTime:        time.Date(2024, 7, 1, 12, 59, 59, 0, time.UTC),
			expectedCounts: map[string]int{
				"2024-07-01T10:00:00Z": 2,
				"2024-07-01T11:00:00Z": 1,
				"2024-07-01T12:00:00Z": 1,
			},
		},
		{
			name: "Monthly Format",
			setup: func() {
				dbManager, _ := testsupport.SetupTestDBManager(t)
				db := dbManager.GetConnection()
				// Create site_stats entries for testing monthly format
				siteStats := []analytics.SiteStat{
					{
						WebsiteID: 1,
						Sessions:  1,
						Hour:      time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC),
					},
					{
						WebsiteID: 1,
						Sessions:  1,
						Hour:      time.Date(2024, 6, 30, 23, 0, 0, 0, time.UTC),
					},
					{
						WebsiteID: 1,
						Sessions:  2,
						Hour:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
					},
					{
						WebsiteID: 1,
						Sessions:  1,
						Hour:      time.Date(2024, 7, 31, 23, 0, 0, 0, time.UTC),
					},
					{
						WebsiteID: 1,
						Sessions:  2,
						Hour:      time.Date(2024, 8, 15, 12, 0, 0, 0, time.UTC),
					},
				}
				db.CreateInBatches(siteStats, len(siteStats))
			},
			timeFrameSize: timeframe.MonthlyTimeFrame,
			fromTime:      time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
			toTime:        time.Date(2024, 8, 31, 23, 59, 59, 0, time.UTC),
			expectedCounts: map[string]int{
				"2024-06-01T00:00:00Z": 2, // 1 + 1
				"2024-07-01T00:00:00Z": 3, // 2 + 1
				"2024-08-01T00:00:00Z": 2, // 2
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dbManager, _ := testsupport.SetupTestDBManager(t)
			db := dbManager.GetConnection()
			testsupport.CleanAllAggregates(db)

			// Set up test data
			tc.setup()

			timeFrame, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
				FromTime:      tc.fromTime,
				ToTime:        tc.toTime,
				TimeFrameSize: tc.timeFrameSize,
			}, time.UTC)
			require.NoError(t, err)

			// Create query params with website ID 1
			queryParams := analytics.NewWebsiteScopedQueryParams(timeFrame, 1)
			result, err := analytics.AggregatedSessionsInTimeFrame(db, queryParams)
			require.NoError(t, err)

			// Check that all expected dates and counts match
			resultMap := make(map[string]int)
			for _, item := range result {
				resultMap[item.Date] = item.Count
			}

			// Verify expected counts
			for expectedDate, expectedCount := range tc.expectedCounts {
				actualCount, exists := resultMap[expectedDate]
				assert.True(t, exists, "Expected date %v not found in results", expectedDate)
				assert.Equal(t, expectedCount, actualCount, "Count mismatch for date %v", expectedDate)
			}

			// Verify no extra dates are present
			assert.Equal(t, len(tc.expectedCounts), len(resultMap),
				"Number of dates in result doesn't match expected")
		})
	}
}

func TestWeeklySessionsSpecialCase(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	testsupport.CleanAllAggregates(db)

	// Using week 27 of 2024 as base week
	week27Start := testsupport.GetFirstDayOfISOWeek(2024, 27)
	week28Start := testsupport.GetFirstDayOfISOWeek(2024, 28)
	week29Start := testsupport.GetFirstDayOfISOWeek(2024, 29)

	// Print week details for debugging
	t.Logf("Week 27 starts: %v", week27Start)
	t.Logf("Week 28 starts: %v", week28Start)
	t.Logf("Week 29 starts: %v", week29Start)

	// Create site_stats entries for each week
	siteStats := []analytics.SiteStat{
		{
			WebsiteID: 1,
			Sessions:  2,
			Hour:      week27Start.Add(24 * time.Hour), // Tuesday of week 27
		},
		{
			WebsiteID: 1,
			Sessions:  2,
			Hour:      week28Start.Add(48 * time.Hour), // Wednesday of week 28
		},
		{
			WebsiteID: 1,
			Sessions:  1,
			Hour:      week29Start.Add(24 * time.Hour), // Tuesday of week 29
		},
	}
	db.CreateInBatches(siteStats, len(siteStats))

	fromTime := week27Start
	toTime := week29Start.Add(7 * 24 * time.Hour) // One week after week 29 start

	t.Logf("Testing timeframe from %v to %v", fromTime, toTime)

	timeFrame, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
		FromTime:      fromTime,
		ToTime:        toTime,
		TimeFrameSize: timeframe.WeeklyTimeFrame,
	}, time.UTC)
	assert.NoError(t, err)

	// Create query params with website ID 1
	queryParams := analytics.NewWebsiteScopedQueryParams(timeFrame, 1)
	result, err := analytics.AggregatedSessionsInTimeFrame(db, queryParams)
	assert.NoError(t, err)

	// Log the actual results
	t.Logf("Actual results: %+v", result)

	// Helper function to find a specific week's data
	findWeekData := func(weekStartDate time.Time, results []timeframe.DateStat) (int, bool) {
		dateStr := weekStartDate.Format("2006-01-02")
		for _, item := range results {
			// Parse the RFC3339 date
			itemDate, err := time.Parse(time.RFC3339, item.Date)
			if err != nil {
				t.Fatalf("Failed to parse date %s: %v", item.Date, err)
			}
			if itemDate.Format("2006-01-02") == dateStr {
				return item.Count, true
			}
		}
		return 0, false
	}

	// Expected counts for each week
	expectedWeeklyCounts := map[time.Time]int{
		week27Start: 2,
		week28Start: 2,
		week29Start: 1,
	}

	// Verify that each expected week has the right session count
	for weekStart, expectedCount := range expectedWeeklyCounts {
		actualCount, found := findWeekData(weekStart, result)
		assert.True(t, found, "Expected data for week starting %v was not found", weekStart)
		assert.Equal(t, expectedCount, actualCount, "Wrong session count for week starting %v", weekStart)
	}

	// Ensure result has the right number of weeks (either 3 or 4 depending on implementation)
	assert.GreaterOrEqual(t, len(result), 3, "Result should have at least 3 weeks")
	assert.LessOrEqual(t, len(result), 4, "Result should have at most 4 weeks")
}

func TestSessionsEdgeCases(t *testing.T) {
	testCases := []struct {
		name           string
		setup          func() *timeframe.TimeFrame
		expectedLength int
		checkFunc      func(*testing.T, []timeframe.DateStat)
	}{
		{
			name: "Empty dataset",
			setup: func() *timeframe.TimeFrame {
				testsupport.SetupTestDBManager(t)
				timeFrame, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
					FromTime:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
					ToTime:        time.Date(2024, 7, 3, 23, 59, 59, 0, time.UTC),
					TimeFrameSize: timeframe.DailyTimeFrame,
				}, time.UTC)
				require.NoError(t, err)
				return timeFrame
			},
			expectedLength: 3, // Should still have 3 data points for the 3 days
			checkFunc: func(t *testing.T, result []timeframe.DateStat) {
				for _, point := range result {
					assert.Equal(t, 0, point.Count, "Count should be zero for empty dataset")
				}
			},
		},
		{
			name: "Single website with multiple sessions",
			setup: func() *timeframe.TimeFrame {
				dbManager, _ := testsupport.SetupTestDBManager(t)
				db := dbManager.GetConnection()

				// Create site_stats for a single website
				siteStats := []analytics.SiteStat{
					{
						WebsiteID: 1,
						Sessions:  3,
						Hour:      time.Date(2024, 7, 1, 10, 0, 0, 0, time.UTC),
					},
					{
						WebsiteID: 1,
						Sessions:  2,
						Hour:      time.Date(2024, 7, 1, 14, 0, 0, 0, time.UTC),
					},
				}
				db.CreateInBatches(siteStats, len(siteStats))

				timeFrame, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
					FromTime:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
					ToTime:        time.Date(2024, 7, 1, 23, 59, 59, 0, time.UTC),
					TimeFrameSize: timeframe.DailyTimeFrame,
				}, time.UTC)
				require.NoError(t, err)
				return timeFrame
			},
			expectedLength: 1, // Just one day
			checkFunc: func(t *testing.T, result []timeframe.DateStat) {
				assert.Equal(t, 5, result[0].Count, "Should count all sessions (3+2)")
			},
		},
		{
			name: "Multiple websites",
			setup: func() *timeframe.TimeFrame {
				dbManager, _ := testsupport.SetupTestDBManager(t)
				db := dbManager.GetConnection()
				// Clean the site_stats table specifically
				testsupport.CleanTables(db, []string{"site_stats"})

				fromTime := time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)
				toTime := time.Date(2024, 7, 1, 23, 59, 59, 0, time.UTC)

				// Insert site_stats for two different websites
				siteStats := []analytics.SiteStat{
					{
						WebsiteID: 1,
						Sessions:  3,
						Hour:      time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
					},
					{
						WebsiteID: 2,
						Sessions:  4,
						Hour:      time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
					},
				}

				// Insert records individually
				for i, stat := range siteStats {
					result := db.Create(&stat)
					if result.Error != nil {
						panic(fmt.Sprintf("Failed to insert test data %d: %v", i, result.Error))
					}
				}

				timeFrame, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
					FromTime:      fromTime,
					ToTime:        toTime,
					TimeFrameSize: timeframe.DailyTimeFrame,
				}, time.UTC)
				require.NoError(t, err)
				return timeFrame
			},
			expectedLength: 1, // Just one day
			checkFunc: func(t *testing.T, result []timeframe.DateStat) {
				// Now we're using a website-scoped query, so we only get sessions for website 1
				assert.Equal(t, 3, result[0].Count, "Should only count sessions for website ID 1")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dbManager, _ := testsupport.SetupTestDBManager(t)
			db := dbManager.GetConnection()
			testsupport.CleanAllAggregates(db)

			timeFrame := tc.setup()
			// Create query params with website ID 1
			queryParams := analytics.NewWebsiteScopedQueryParams(timeFrame, 1)
			result, err := analytics.AggregatedSessionsInTimeFrame(db, queryParams)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedLength, len(result), "Unexpected number of data points")
			tc.checkFunc(t, result)
		})
	}
}
