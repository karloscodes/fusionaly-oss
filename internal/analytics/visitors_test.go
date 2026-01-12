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

func TestAggregatedUniqueVisitors(t *testing.T) {
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
						Visitors:  2,
						Hour:      time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
					},
					{
						WebsiteID: 1,
						Visitors:  2,
						Hour:      time.Date(2024, 7, 2, 12, 0, 0, 0, time.UTC),
					},
					{
						WebsiteID: 1,
						Visitors:  1,
						Hour:      time.Date(2024, 7, 3, 12, 0, 0, 0, time.UTC),
					},
				}
				db.CreateInBatches(siteStats, len(siteStats))
			},
			timeFrameSize: timeframe.DailyTimeFrame,
			fromTime:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
			toTime:        time.Date(2024, 7, 3, 23, 59, 59, 0, time.UTC),
			expectedCounts: map[string]int{
				"2024-07-01T00:00:00Z": 2,
				"2024-07-02T00:00:00Z": 2,
				"2024-07-03T00:00:00Z": 1,
			},
		},
		{
			name: "Weekly Format",
			setup: func() {
				dbManager, _ := testsupport.SetupTestDBManager(t)
				db := dbManager.GetConnection()
				// Using week 27 and 28 of 2024
				week27Start := testsupport.GetFirstDayOfISOWeek(2024, 27)
				week28Start := testsupport.GetFirstDayOfISOWeek(2024, 28)

				// Create site_stats entries for weekly format
				siteStats := []analytics.SiteStat{
					{
						WebsiteID: 1,
						Visitors:  3,
						Hour:      week27Start.Add(24 * time.Hour), // Tuesday of week 27
					},
					{
						WebsiteID: 1,
						Visitors:  2,
						Hour:      week28Start.Add(48 * time.Hour), // Wednesday of week 28
					},
				}
				db.CreateInBatches(siteStats, len(siteStats))
			},
			timeFrameSize: timeframe.WeeklyTimeFrame,
			fromTime:      testsupport.GetFirstDayOfISOWeek(2024, 27),
			toTime:        testsupport.GetFirstDayOfISOWeek(2024, 28).Add(7*24*time.Hour - time.Second),
			expectedCounts: map[string]int{
				testsupport.GetFirstDayOfISOWeek(2024, 27).Format(time.RFC3339): 3,
				testsupport.GetFirstDayOfISOWeek(2024, 28).Format(time.RFC3339): 2,
			},
		},
		{
			name: "Hourly Format",
			setup: func() {
				dbManager, _ := testsupport.SetupTestDBManager(t)
				db := dbManager.GetConnection()
				// Create site_stats entries for hourly format
				siteStats := []analytics.SiteStat{
					{
						WebsiteID: 1,
						Visitors:  2,
						Hour:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
					},
					{
						WebsiteID: 1,
						Visitors:  3,
						Hour:      time.Date(2024, 7, 1, 1, 0, 0, 0, time.UTC),
					},
					{
						WebsiteID: 1,
						Visitors:  0,
						Hour:      time.Date(2024, 7, 1, 2, 0, 0, 0, time.UTC),
					},
				}
				db.CreateInBatches(siteStats, len(siteStats))
			},
			timeFrameSize: timeframe.HourlyTimeFrame,
			fromTime:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
			toTime:        time.Date(2024, 7, 1, 3, 0, 0, 0, time.UTC),
			expectedCounts: map[string]int{
				"2024-07-01T00:00:00Z": 2,
				"2024-07-01T01:00:00Z": 3,
				"2024-07-01T02:00:00Z": 0,
				"2024-07-01T03:00:00Z": 0, // Extra point due to TimeWindowBuffer including endpoint
			},
		},
		{
			name: "Monthly Format",
			setup: func() {
				dbManager, _ := testsupport.SetupTestDBManager(t)
				db := dbManager.GetConnection()
				// Create site_stats entries for monthly format
				siteStats := []analytics.SiteStat{
					{
						WebsiteID: 1,
						Visitors:  3,
						Hour:      time.Date(2024, 7, 15, 12, 0, 0, 0, time.UTC),
					},
					{
						WebsiteID: 1,
						Visitors:  2,
						Hour:      time.Date(2024, 8, 15, 12, 0, 0, 0, time.UTC),
					},
				}
				db.CreateInBatches(siteStats, len(siteStats))
			},
			timeFrameSize: timeframe.MonthlyTimeFrame,
			fromTime:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
			toTime:        time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC),
			expectedCounts: map[string]int{
				"2024-07-01T00:00:00Z": 3,
				"2024-08-01T00:00:00Z": 2,
				"2024-09-01T00:00:00Z": 0, // Extra point due to TimeWindowBuffer including endpoint
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
			result, err := analytics.AggregatedVisitorsInTimeFrame(db, queryParams)
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

func TestVisitorsEdgeCases(t *testing.T) {
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
			name: "Single-day timeframe",
			setup: func() *timeframe.TimeFrame {
				dbManager, _ := testsupport.SetupTestDBManager(t)
				db := dbManager.GetConnection()

				// Create site_stats for a single day
				siteStats := []analytics.SiteStat{
					{
						WebsiteID: 1,
						Visitors:  2,
						Hour:      time.Date(2024, 7, 1, 10, 0, 0, 0, time.UTC),
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
				assert.Equal(t, 2, result[0].Count, "Should count both visitors")
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
						Visitors:  3,
						Hour:      time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC),
					},
					{
						WebsiteID: 2,
						Visitors:  2,
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
				// Now we're using a website-scoped query, so we only get visitors for website 1
				assert.Equal(t, 3, result[0].Count, "Should only count visitors for website ID 1")
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
			result, err := analytics.AggregatedVisitorsInTimeFrame(db, queryParams)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedLength, len(result), "Unexpected number of data points")
			tc.checkFunc(t, result)
		})
	}
}
