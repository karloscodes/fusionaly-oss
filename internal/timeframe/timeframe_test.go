// Package timeframe_test contains tests for the timeframe package
package timeframe_test

import (
	"fusionaly/internal/timeframe"

	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockTimeProvider implements the TimeProvider interface for testing
type MockTimeProvider struct {
	FixedTime time.Time
}

func (m *MockTimeProvider) Now(loc *time.Location) time.Time {
	return m.FixedTime.In(loc)
}

func TestTimeFrameParserWithTimeProvider(t *testing.T) {
	// Fixed time for stable testing: March 15, 2024, 12:00 UTC
	fixedTime := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	mockProvider := &MockTimeProvider{FixedTime: fixedTime}

	parser := timeframe.NewTimeFrameParser(mockProvider)

	testCases := []struct {
		name           string
		params         timeframe.TimeFrameParserParams
		expectedFrom   time.Time
		expectedTo     time.Time
		expectedBucket timeframe.TimeFrameBucketSize
		expectedError  bool
	}{
		{
			name: "Today Range - UTC",
			params: timeframe.TimeFrameParserParams{
				FromDate: "2024-03-15",
				ToDate:   "2024-03-15",
				Tz:       "UTC",
			},
			expectedFrom:  time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
			expectedTo:    time.Date(2024, 3, 15, 12, 59, 59, 0, time.UTC), // Truncated to hour 12:00 + 1 hour - 1 second
			expectedError: false,
		},
		{
			name: "Yesterday Range - UTC",
			params: timeframe.TimeFrameParserParams{
				FromDate: "2024-03-14",
				ToDate:   "2024-03-14",
				Tz:       "UTC",
			},
			expectedFrom:  time.Date(2024, 3, 14, 0, 0, 0, 0, time.UTC),
			expectedTo:    time.Date(2024, 3, 14, 23, 59, 59, 999999999, time.UTC), // Past date, end of day
			expectedError: false,
		},
		{
			name: "Last 7 Days - UTC",
			params: timeframe.TimeFrameParserParams{
				FromDate: "2024-03-08",
				ToDate:   "2024-03-15",
				Tz:       "UTC",
			},
			expectedFrom:   time.Date(2024, 3, 8, 0, 0, 0, 0, time.UTC),
			expectedTo:     time.Date(2024, 3, 15, 23, 59, 59, 0, time.UTC), // Truncated to day + 1 day - 1 second
			expectedBucket: timeframe.TimeFrameBucketSizeDay,
			expectedError:  false,
		},
		{
			name: "Last 30 Days - UTC",
			params: timeframe.TimeFrameParserParams{
				FromDate: "2024-02-14",
				ToDate:   "2024-03-15",
				Tz:       "UTC",
			},
			expectedFrom:   time.Date(2024, 2, 14, 0, 0, 0, 0, time.UTC),
			expectedTo:     time.Date(2024, 3, 15, 23, 59, 59, 0, time.UTC), // Truncated to day + 1 day - 1 second
			expectedBucket: timeframe.TimeFrameBucketSizeDay,
			expectedError:  false,
		},
		{
			name: "This Month - UTC",
			params: timeframe.TimeFrameParserParams{
				FromDate: "2024-03-01",
				ToDate:   "2024-03-15",
				Tz:       "UTC",
			},
			expectedFrom:   time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
			expectedTo:     time.Date(2024, 3, 15, 23, 59, 59, 0, time.UTC), // Truncated to day + 1 day - 1 second
			expectedBucket: timeframe.TimeFrameBucketSizeDay,
			expectedError:  false,
		},
		{
			name: "Custom Date Range - Within Bounds",
			params: timeframe.TimeFrameParserParams{
				FromDate: "2024-03-01",
				ToDate:   "2024-03-10",
				Tz:       "UTC",
			},
			expectedFrom:  time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
			expectedTo:    time.Date(2024, 3, 10, 23, 59, 59, 999999999, time.UTC),
			expectedError: false,
		},
		{
			name: "Custom Date Range - To Date After Stable Now",
			params: timeframe.TimeFrameParserParams{
				FromDate: "2024-03-01",
				ToDate:   "2024-03-20", // After our fixed stable now
				Tz:       "UTC",
			},
			expectedFrom:  time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
			expectedTo:    time.Date(2024, 3, 15, 23, 59, 59, 0, time.UTC), // 19-day range = daily bucket, so day + 1 day - 1 second
			expectedError: false,
		},
		{
			name: "Custom Date Range - Invalid From Date",
			params: timeframe.TimeFrameParserParams{
				FromDate: "invalid-date",
				ToDate:   "2024-03-10",
				Tz:       "UTC",
			},
			expectedError: true,
		},
		{
			name: "Custom Date Range - Invalid To Date",
			params: timeframe.TimeFrameParserParams{
				FromDate: "2024-03-01",
				ToDate:   "invalid-date",
				Tz:       "UTC",
			},
			expectedError: true,
		},
		{
			name: "Custom Date Range - From After To",
			params: timeframe.TimeFrameParserParams{
				FromDate: "2024-03-10",
				ToDate:   "2024-03-01",
				Tz:       "UTC",
			},
			expectedError: true,
		},
		// {
		// 	name: "Today Range - Different Time Zone (America/New_York)",
		// 	params: timeframe.TimeFrameParserParams{
		// 		RangeParam: string(timeframe.TimeFrameRangeLabelToday),
		// 		Tz:         "America/New_York",
		// 	},
		// 	// America/New_York is UTC-4 in March 2024 (daylight saving time starts March 10, 2024)
		// 	// Fixed time: 2024-03-15 12:00:00 UTC → 2024-03-15 08:00:00 America/New_York (EDT)
		// 	// Start of day in America/New_York: 2024-03-15 00:00:00 America/New_York → 2024-03-15 04:00:00 UTC
		// 	// Current time in America/New_York: 2024-03-15 08:00:00 America/New_York → 2024-03-15 12:00:00 UTC
		// 	expectedFrom:  time.Date(2024, 3, 15, 4, 0, 0, 0, time.UTC),
		// 	expectedTo:    fixedTime,
		// 	expectedError: false,
		// },
		{
			name: "Invalid Time Zone",
			params: timeframe.TimeFrameParserParams{
				FromDate: "2024-03-15",
				ToDate:   "2024-03-15",
				Tz:       "Invalid/Timezone",
			},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			timeFrame, err := parser.ParseTimeFrame(tc.params)

			if tc.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, timeFrame)

			// Log the From and To times with their locations
			t.Logf("From: %v, To: %v", timeFrame.From, timeFrame.To)

			// Check bucket size if specified
			if tc.expectedBucket != "" {
				assert.Equal(t, tc.expectedBucket, timeFrame.BucketSize,
					"Bucket size should match expected for %s", tc.name)
			}

			// We need some tolerance for timezone conversions
			fromDiff := tc.expectedFrom.Sub(timeFrame.From).Abs()
			toDiff := tc.expectedTo.Sub(timeFrame.To).Abs()

			assert.True(t, fromDiff < time.Second,
				"Expected From time %v, got %v, diff: %v",
				tc.expectedFrom, timeFrame.From, fromDiff)
			assert.True(t, toDiff < time.Second,
				"Expected To time %v, got %v, diff: %v",
				tc.expectedTo, timeFrame.To, toDiff)
		})
	}
}

func TestTimeFrameMethods(t *testing.T) {
	testCases := []struct {
		name           string
		timeFrame      *timeframe.TimeFrame
		expectedPoints int
		checkFunc      func(*testing.T, *timeframe.TimeFrame)
	}{
		{
			name: "Hourly Time Frame",
			timeFrame: func() *timeframe.TimeFrame {
				tf, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
					FromTime:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
					ToTime:        time.Date(2024, 7, 1, 3, 0, 0, 0, time.UTC),
					TimeFrameSize: timeframe.HourlyTimeFrame,
				}, time.UTC)
				assert.NoError(t, err)
				return tf
			}(),
			expectedPoints: 4, // 00:00, 01:00, 02:00, 03:00 (changed from 3 to 4 to include endpoint)
			checkFunc: func(t *testing.T, tf *timeframe.TimeFrame) {
				// Test GetSQLiteGroupByExpression
				expr, err := tf.GetSQLiteGroupByExpression()
				assert.NoError(t, err)
				assert.Equal(t, "strftime('%Y-%m-%d %H', hour)", expr)

				// Test GenerateDateTimePointsReference
				points := tf.GenerateDateTimePointsReference()
				assert.Len(t, points, 4) // Updated from 3 to 4
				assert.Equal(t, "2024-07-01 00", points[0].SQLiteBucketTimeFormat)
				assert.Equal(t, "2024-07-01T00:00:00Z", points[0].UserFacingTimeFormat)
				assert.Equal(t, "2024-07-01 01", points[1].SQLiteBucketTimeFormat)
				assert.Equal(t, "2024-07-01T01:00:00Z", points[1].UserFacingTimeFormat)
				assert.Equal(t, "2024-07-01 02", points[2].SQLiteBucketTimeFormat)
				assert.Equal(t, "2024-07-01T02:00:00Z", points[2].UserFacingTimeFormat)
				assert.Equal(t, "2024-07-01 03", points[3].SQLiteBucketTimeFormat)
				assert.Equal(t, "2024-07-01T03:00:00Z", points[3].UserFacingTimeFormat)

				// Test BuildTimeSeriesPoints with sample data
				rawData := []timeframe.DateStat{
					{Date: "2024-07-01 00", Count: 2},
					{Date: "2024-07-01 01", Count: 3},
				}
				result := tf.BuildTimeSeriesPoints(rawData)
				assert.Len(t, result, 4) // Updated from 3 to 4
				assert.Equal(t, timeframe.DateStat{Date: "2024-07-01T00:00:00Z", Count: 2}, result[0])
				assert.Equal(t, timeframe.DateStat{Date: "2024-07-01T01:00:00Z", Count: 3}, result[1])
				assert.Equal(t, timeframe.DateStat{Date: "2024-07-01T02:00:00Z", Count: 0}, result[2])
				assert.Equal(t, timeframe.DateStat{Date: "2024-07-01T03:00:00Z", Count: 0}, result[3])
			},
		},
		{
			name: "Daily Time Frame",
			timeFrame: func() *timeframe.TimeFrame {
				tf, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
					FromTime:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
					ToTime:        time.Date(2024, 7, 3, 23, 59, 59, 0, time.UTC),
					TimeFrameSize: timeframe.DailyTimeFrame,
				}, time.UTC)
				assert.NoError(t, err)
				return tf
			}(),
			expectedPoints: 3, // 2024-07-01, 2024-07-02, 2024-07-03
			checkFunc: func(t *testing.T, tf *timeframe.TimeFrame) {
				// Test GetSQLiteGroupByExpression
				expr, err := tf.GetSQLiteGroupByExpression()
				assert.NoError(t, err)
				assert.Equal(t, "strftime('%Y-%m-%d', hour)", expr)

				// Test GenerateDateTimePointsReference
				points := tf.GenerateDateTimePointsReference()
				assert.Len(t, points, 3)
				assert.Equal(t, "2024-07-01", points[0].SQLiteBucketTimeFormat)
				assert.Equal(t, "2024-07-01T00:00:00Z", points[0].UserFacingTimeFormat)
				assert.Equal(t, "2024-07-02", points[1].SQLiteBucketTimeFormat)
				assert.Equal(t, "2024-07-02T00:00:00Z", points[1].UserFacingTimeFormat)
				assert.Equal(t, "2024-07-03", points[2].SQLiteBucketTimeFormat)
				assert.Equal(t, "2024-07-03T00:00:00Z", points[2].UserFacingTimeFormat)
			},
		},
		{
			name: "Weekly Time Frame",
			timeFrame: func() *timeframe.TimeFrame {
				tf, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
					FromTime:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
					ToTime:        time.Date(2024, 7, 14, 23, 59, 59, 0, time.UTC),
					TimeFrameSize: timeframe.WeeklyTimeFrame,
				}, time.UTC)
				assert.NoError(t, err)
				return tf
			}(),
			expectedPoints: 2, // 2024-07-01, 2024-07-08
			checkFunc: func(t *testing.T, tf *timeframe.TimeFrame) {
				// Test GetSQLiteGroupByExpression
				expr, err := tf.GetSQLiteGroupByExpression()
				assert.NoError(t, err)
				assert.Equal(t, "date(hour, 'start of day', '-' || ((strftime('%w', hour) + 6) % 7) || ' days')", expr)

				// Test GenerateDateTimePointsReference
				points := tf.GenerateDateTimePointsReference()
				assert.Len(t, points, 2)
				assert.Equal(t, "2024-07-01", points[0].SQLiteBucketTimeFormat)
				assert.Equal(t, "2024-07-01T00:00:00Z", points[0].UserFacingTimeFormat)
				assert.Equal(t, "2024-07-08", points[1].SQLiteBucketTimeFormat)
				assert.Equal(t, "2024-07-08T00:00:00Z", points[1].UserFacingTimeFormat)
			},
		},
		{
			name: "Monthly Time Frame",
			timeFrame: func() *timeframe.TimeFrame {
				tf, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
					FromTime:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
					ToTime:        time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC),
					TimeFrameSize: timeframe.MonthlyTimeFrame,
				}, time.UTC)
				assert.NoError(t, err)
				return tf
			}(),
			expectedPoints: 3, // 2024-07-01, 2024-08-01, 2024-09-01 (changed from 2 to 3)
			checkFunc: func(t *testing.T, tf *timeframe.TimeFrame) {
				// Test GetSQLiteGroupByExpression
				expr, err := tf.GetSQLiteGroupByExpression()
				assert.NoError(t, err)
				assert.Equal(t, "strftime('%Y-%m', hour)", expr)

				// Test GenerateDateTimePointsReference
				points := tf.GenerateDateTimePointsReference()
				assert.Len(t, points, 3) // Updated from 2 to 3
				assert.Equal(t, "2024-07", points[0].SQLiteBucketTimeFormat)
				assert.Equal(t, "2024-07-01T00:00:00Z", points[0].UserFacingTimeFormat)
				assert.Equal(t, "2024-08", points[1].SQLiteBucketTimeFormat)
				assert.Equal(t, "2024-08-01T00:00:00Z", points[1].UserFacingTimeFormat)
				assert.Equal(t, "2024-09", points[2].SQLiteBucketTimeFormat)
				assert.Equal(t, "2024-09-01T00:00:00Z", points[2].UserFacingTimeFormat)
			},
		},
		{
			name: "Yearly Time Frame",
			timeFrame: func() *timeframe.TimeFrame {
				tf, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
					FromTime:      time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
					ToTime:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
					TimeFrameSize: timeframe.YearlyTimeFrame,
				}, time.UTC)
				assert.NoError(t, err)
				return tf
			}(),
			expectedPoints: 3, // 2023, 2024, 2025 (changed from 2 to 3)
			checkFunc: func(t *testing.T, tf *timeframe.TimeFrame) {
				// Test GetSQLiteGroupByExpression
				expr, err := tf.GetSQLiteGroupByExpression()
				assert.NoError(t, err)
				assert.Equal(t, "strftime('%Y', hour)", expr)

				// Test GenerateDateTimePointsReference
				points := tf.GenerateDateTimePointsReference()
				assert.Len(t, points, 3) // Updated from 2 to 3
				assert.Equal(t, "2023", points[0].SQLiteBucketTimeFormat)
				assert.Equal(t, "2023-01-01T00:00:00Z", points[0].UserFacingTimeFormat)
				assert.Equal(t, "2024", points[1].SQLiteBucketTimeFormat)
				assert.Equal(t, "2024-01-01T00:00:00Z", points[1].UserFacingTimeFormat)
				assert.Equal(t, "2025", points[2].SQLiteBucketTimeFormat)
				assert.Equal(t, "2025-01-01T00:00:00Z", points[2].UserFacingTimeFormat)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test GenerateDateTimePointsReference length
			points := tc.timeFrame.GenerateDateTimePointsReference()
			assert.Equal(t, tc.expectedPoints, len(points), "Unexpected number of date points")

			// Run additional checks
			tc.checkFunc(t, tc.timeFrame)
		})
	}
}

func TestBuildTimeSeriesPoints(t *testing.T) {
	testCases := []struct {
		name         string
		timeFrame    *timeframe.TimeFrame
		inputData    []timeframe.DateStat
		expectedData []timeframe.DateStat
	}{
		{
			name: "Hourly data with standard format",
			timeFrame: func() *timeframe.TimeFrame {
				tf, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
					FromTime:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
					ToTime:        time.Date(2024, 7, 1, 2, 0, 0, 0, time.UTC),
					TimeFrameSize: timeframe.HourlyTimeFrame,
				}, time.UTC)
				assert.NoError(t, err)
				return tf
			}(),
			inputData: []timeframe.DateStat{
				{Date: "2024-07-01 00", Count: 5},
				{Date: "2024-07-01 01", Count: 10},
			},
			expectedData: []timeframe.DateStat{
				{Date: "2024-07-01T00:00:00Z", Count: 5},
				{Date: "2024-07-01T01:00:00Z", Count: 10},
				{Date: "2024-07-01T02:00:00Z", Count: 0},
			},
		},
		{
			name: "Hourly data with seconds format",
			timeFrame: func() *timeframe.TimeFrame {
				tf, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
					FromTime:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
					ToTime:        time.Date(2024, 7, 1, 2, 0, 0, 0, time.UTC),
					TimeFrameSize: timeframe.HourlyTimeFrame,
				}, time.UTC)
				assert.NoError(t, err)
				return tf
			}(),
			inputData: []timeframe.DateStat{
				{Date: "2024-07-01 00:00:00", Count: 5},
				{Date: "2024-07-01 01:00:00", Count: 10},
			},
			expectedData: []timeframe.DateStat{
				{Date: "2024-07-01T00:00:00Z", Count: 5},
				{Date: "2024-07-01T01:00:00Z", Count: 10},
				{Date: "2024-07-01T02:00:00Z", Count: 0},
			},
		},
		{
			name: "Daily data",
			timeFrame: func() *timeframe.TimeFrame {
				tf, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
					FromTime:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
					ToTime:        time.Date(2024, 7, 3, 0, 0, 0, 0, time.UTC),
					TimeFrameSize: timeframe.DailyTimeFrame,
				}, time.UTC)
				assert.NoError(t, err)
				return tf
			}(),
			inputData: []timeframe.DateStat{
				{Date: "2024-07-01", Count: 50},
				{Date: "2024-07-02", Count: 75},
			},
			expectedData: []timeframe.DateStat{
				{Date: "2024-07-01T00:00:00Z", Count: 50},
				{Date: "2024-07-02T00:00:00Z", Count: 75},
				{Date: "2024-07-03T00:00:00Z", Count: 0},
			},
		},
		{
			name: "Monthly data",
			timeFrame: func() *timeframe.TimeFrame {
				tf, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
					FromTime:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					ToTime:        time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
					TimeFrameSize: timeframe.MonthlyTimeFrame,
				}, time.UTC)
				assert.NoError(t, err)
				return tf
			}(),
			inputData: []timeframe.DateStat{
				{Date: "2024-01", Count: 500},
				{Date: "2024-02", Count: 750},
			},
			expectedData: []timeframe.DateStat{
				{Date: "2024-01-01T00:00:00Z", Count: 500},
				{Date: "2024-02-01T00:00:00Z", Count: 750},
				{Date: "2024-03-01T00:00:00Z", Count: 0},
			},
		},
		{
			name: "Yearly data",
			timeFrame: func() *timeframe.TimeFrame {
				tf, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
					FromTime:      time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					ToTime:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					TimeFrameSize: timeframe.YearlyTimeFrame,
				}, time.UTC)
				assert.NoError(t, err)
				return tf
			}(),
			inputData: []timeframe.DateStat{
				{Date: "2022", Count: 5000},
				{Date: "2023", Count: 7500},
			},
			expectedData: []timeframe.DateStat{
				{Date: "2022-01-01T00:00:00Z", Count: 5000},
				{Date: "2023-01-01T00:00:00Z", Count: 7500},
				{Date: "2024-01-01T00:00:00Z", Count: 0},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.timeFrame.BuildTimeSeriesPoints(tc.inputData)
			assert.Equal(t, tc.expectedData, result)
		})
	}
}

func TestGenerateDateTimePointsReference_Timezone(t *testing.T) {
	// Test new UTC-only approach - backend returns UTC dates, frontend handles timezone conversion
	madridTz, err := time.LoadLocation("Europe/Madrid")
	assert.NoError(t, err)

	// Create a time frame for July 6, 2025 in Madrid timezone
	// User input: from=2025-07-06 (meaning July 6 midnight in Madrid)
	// Backend should parse this as Madrid midnight, convert to UTC for storage
	fromTime := time.Date(2025, 7, 6, 0, 0, 0, 0, madridTz) // July 6 00:00 Madrid
	toTime := time.Date(2025, 7, 8, 0, 0, 0, 0, madridTz)   // July 8 00:00 Madrid

	timeFrame := &timeframe.TimeFrame{
		From:       fromTime.UTC(), // TimeFrame stores UTC internally (July 5 22:00 UTC)
		To:         toTime.UTC(),   // TimeFrame stores UTC internally (July 7 22:00 UTC)
		BucketSize: timeframe.TimeFrameBucketSizeDay,
		Label:      timeframe.TimeFrameRangeLabelCustom,
		Tz:         madridTz, // Timezone for reference (not used in new approach)
	}

	points := timeFrame.GenerateDateTimePointsReference()

	// Should have 2 points: July 6, 7
	// ToTime is July 8 00:00 Madrid = July 7 22:00 UTC, so July 8 is not included
	assert.Len(t, points, 2)

	// First point should be July 6 at 00:00 UTC (representing the date July 6)
	// User requested July 6 in Madrid, so we display it as July 6
	firstPoint := points[0]
	assert.Equal(t, "2025-07-06", firstPoint.SQLiteBucketTimeFormat)

	// Parse the RFC3339 date to verify it's in UTC format
	parsedTime, err := time.Parse(time.RFC3339, firstPoint.UserFacingTimeFormat)
	assert.NoError(t, err)

	// The time should be July 6 00:00 UTC (so it displays as July 6 in all timezones)
	expectedTime := time.Date(2025, 7, 6, 0, 0, 0, 0, time.UTC)
	assert.True(t, parsedTime.Equal(expectedTime),
		"Expected %s, got %s", expectedTime.Format(time.RFC3339), parsedTime.Format(time.RFC3339))

	// Verify the timezone is UTC (should have Z suffix)
	assert.Contains(t, firstPoint.UserFacingTimeFormat, "Z",
		"Date should be in UTC timezone (Z), got: %s", firstPoint.UserFacingTimeFormat)
}

func TestGenerateDateTimePointsReference_UTC(t *testing.T) {
	// Test UTC timezone handling
	utcTz := time.UTC

	fromTime := time.Date(2025, 7, 6, 0, 0, 0, 0, utcTz)
	toTime := time.Date(2025, 7, 8, 0, 0, 0, 0, utcTz)

	timeFrame := &timeframe.TimeFrame{
		From:       fromTime,
		To:         toTime,
		BucketSize: timeframe.TimeFrameBucketSizeDay,
		Label:      timeframe.TimeFrameRangeLabelCustom,
		Tz:         utcTz,
	}

	points := timeFrame.GenerateDateTimePointsReference()

	// Should have 3 points: July 6, 7, 8
	assert.Len(t, points, 3)

	// First point should be July 6 at midnight UTC
	firstPoint := points[0]
	assert.Equal(t, "2025-07-06", firstPoint.SQLiteBucketTimeFormat)

	// Parse the RFC3339 date to verify it's in UTC
	parsedTime, err := time.Parse(time.RFC3339, firstPoint.UserFacingTimeFormat)
	assert.NoError(t, err)

	// The time should be midnight in UTC
	expectedTime := time.Date(2025, 7, 6, 0, 0, 0, 0, utcTz)
	assert.True(t, parsedTime.Equal(expectedTime),
		"Expected %s, got %s", expectedTime.Format(time.RFC3339), parsedTime.Format(time.RFC3339))

	// Verify the timezone is UTC (Z suffix)
	assert.Contains(t, firstPoint.UserFacingTimeFormat, "Z",
		"Date should be in UTC timezone (Z), got: %s", firstPoint.UserFacingTimeFormat)
}

func TestGenerateDateTimePointsReference_HourlyTimezone(t *testing.T) {
	// Test hourly bucketing with new UTC-only approach
	madridTz, err := time.LoadLocation("Europe/Madrid")
	assert.NoError(t, err)

	// Create a time frame for July 6, 2025 from 10 AM to 2 PM Madrid time
	// User input: from=2025-07-06 10:00 to=2025-07-06 14:00 (Madrid time)
	fromTime := time.Date(2025, 7, 6, 10, 0, 0, 0, madridTz) // 10 AM Madrid = 8 AM UTC
	toTime := time.Date(2025, 7, 6, 14, 0, 0, 0, madridTz)   // 2 PM Madrid = 12 PM UTC

	timeFrame := &timeframe.TimeFrame{
		From:       fromTime.UTC(), // 8 AM UTC
		To:         toTime.UTC(),   // 12 PM UTC
		BucketSize: timeframe.TimeFrameBucketSizeHour,
		Label:      timeframe.TimeFrameRangeLabelCustom,
		Tz:         madridTz, // Timezone for reference (not used in new approach)
	}

	points := timeFrame.GenerateDateTimePointsReference()

	// Should have 5 points: 8 AM, 9 AM, 10 AM, 11 AM, 12 PM (UTC)
	assert.Len(t, points, 5)

	// First point should be 8 AM UTC (representing 10 AM Madrid)
	firstPoint := points[0]
	assert.Equal(t, "2025-07-06 08", firstPoint.SQLiteBucketTimeFormat)

	// Parse the RFC3339 date to verify it's 8 AM UTC
	parsedTime, err := time.Parse(time.RFC3339, firstPoint.UserFacingTimeFormat)
	assert.NoError(t, err)

	// The time should be 8 AM UTC (which equals 10 AM Madrid)
	expectedTime := time.Date(2025, 7, 6, 8, 0, 0, 0, time.UTC)
	assert.True(t, parsedTime.Equal(expectedTime),
		"Expected %s, got %s", expectedTime.Format(time.RFC3339), parsedTime.Format(time.RFC3339))

	// Verify the timezone is UTC (should have Z suffix)
	assert.Contains(t, firstPoint.UserFacingTimeFormat, "Z",
		"Date should be in UTC timezone (Z), got: %s", firstPoint.UserFacingTimeFormat)
}

func TestTruncateToBucketInTimezone(t *testing.T) {
	madridTz, err := time.LoadLocation("Europe/Madrid")
	assert.NoError(t, err)

	// Test day truncation
	testTime := time.Date(2025, 7, 6, 15, 30, 45, 0, madridTz)
	truncated := timeframe.TruncateToBucketInTimezone(testTime, timeframe.TimeFrameBucketSizeDay, madridTz)

	expected := time.Date(2025, 7, 6, 0, 0, 0, 0, madridTz)
	assert.True(t, truncated.Equal(expected),
		"Expected %s, got %s", expected.Format(time.RFC3339), truncated.Format(time.RFC3339))

	// Test hour truncation
	truncated = timeframe.TruncateToBucketInTimezone(testTime, timeframe.TimeFrameBucketSizeHour, madridTz)
	expected = time.Date(2025, 7, 6, 15, 0, 0, 0, madridTz)
	assert.True(t, truncated.Equal(expected),
		"Expected %s, got %s", expected.Format(time.RFC3339), truncated.Format(time.RFC3339))

	// Test month truncation
	truncated = timeframe.TruncateToBucketInTimezone(testTime, timeframe.TimeFrameBucketSizeMonth, madridTz)
	expected = time.Date(2025, 7, 1, 0, 0, 0, 0, madridTz)
	assert.True(t, truncated.Equal(expected),
		"Expected %s, got %s", expected.Format(time.RFC3339), truncated.Format(time.RFC3339))
}

func TestGenerateDateTimePointsReference_FallbackToUTC(t *testing.T) {
	// Test fallback to UTC when no timezone is set
	fromTime := time.Date(2025, 7, 6, 0, 0, 0, 0, time.UTC)
	toTime := time.Date(2025, 7, 8, 0, 0, 0, 0, time.UTC)

	timeFrame := &timeframe.TimeFrame{
		From:       fromTime,
		To:         toTime,
		BucketSize: timeframe.TimeFrameBucketSizeDay,
		Label:      timeframe.TimeFrameRangeLabelCustom,
		Tz:         nil, // No timezone set
	}

	points := timeFrame.GenerateDateTimePointsReference()

	// Should have 3 points: July 6, 7, 8
	assert.Len(t, points, 3)

	// First point should be July 6 at midnight UTC
	firstPoint := points[0]
	assert.Equal(t, "2025-07-06", firstPoint.SQLiteBucketTimeFormat)

	// Verify the timezone is UTC (Z suffix)
	assert.Contains(t, firstPoint.UserFacingTimeFormat, "Z",
		"Date should fallback to UTC timezone (Z), got: %s", firstPoint.UserFacingTimeFormat)
}

// TestDailyBucketIncludesCurrentDay ensures that when generating daily buckets,
// the bucket for the current day is included even if current time is before end of day.
// This fixes the bug where "last 30 days" was missing today's data.
func TestDailyBucketIncludesCurrentDay(t *testing.T) {
	// Simulate the bug scenario:
	// Current time: Nov 29, 14:02 CET (13:02 UTC)
	// User selects "last 30 days" which should include today (Nov 29)

	cetLoc, err := time.LoadLocation("Europe/Madrid")
	assert.NoError(t, err)

	currentTime := time.Date(2025, 11, 29, 14, 2, 0, 0, cetLoc)
	mockProvider := &MockTimeProvider{FixedTime: currentTime}

	parser := timeframe.NewTimeFrameParser(mockProvider)

	// Frontend sends: from="2025-10-30", to="2025-11-29" (today)
	params := timeframe.TimeFrameParserParams{
		FromDate: "2025-10-30",
		ToDate:   "2025-11-29",
		Tz:       "Europe/Madrid",
	}

	tf, err := parser.ParseTimeFrame(params)
	assert.NoError(t, err)
	assert.NotNil(t, tf)

	// Should use daily buckets for 30-day range
	assert.Equal(t, timeframe.TimeFrameBucketSizeDay, tf.BucketSize)

	// Generate time series points
	datePoints := tf.GenerateDateTimePointsReference()

	// Check that we have points for the full 30-day range
	assert.GreaterOrEqual(t, len(datePoints), 30, "Should have at least 30 daily points")

	// The last point should be for Nov 29 (today)
	lastPoint := datePoints[len(datePoints)-1]

	// Parse the last point's date
	lastDate, err := time.Parse(time.RFC3339, lastPoint.UserFacingTimeFormat)
	assert.NoError(t, err)

	// The last bucket should represent Nov 29 in UTC
	// For CET timezone, Nov 29 starts at Nov 28, 23:00 UTC
	// The last bucket should represent Nov 29 (the current day)
	// For CET timezone (UTC+1), Nov 29 00:00 CET = Nov 28 23:00 UTC, but bucket is Nov 29
	expectedLastBucketStart := time.Date(2025, 11, 29, 23, 0, 0, 0, time.UTC)
	assert.Equal(t, expectedLastBucketStart.Month(), lastDate.Month(), "Last bucket should be for November")
	assert.Equal(t, expectedLastBucketStart.Day(), lastDate.Day(), "Last bucket should start on Nov 28 23:00 UTC (Nov 29 CET)")

	assert.Equal(t, expectedLastBucketStart.Day(), lastDate.Day(), "Last bucket should be Nov 29")

	t.Logf("First bucket: %s (%s)", datePoints[0].UserFacingTimeFormat, datePoints[0].SQLiteBucketTimeFormat)
	t.Logf("Last bucket: %s (%s)", lastPoint.UserFacingTimeFormat, lastPoint.SQLiteBucketTimeFormat)
}

// TestDailyBucketAtEndOfDay tests the edge case when current time is near end of day
func TestDailyBucketAtEndOfDay(t *testing.T) {
	utcLoc := time.UTC

	// Current time: Nov 29, 23:30 UTC (30 minutes before end of day)
	currentTime := time.Date(2025, 11, 29, 23, 30, 0, 0, utcLoc)
	mockProvider := &MockTimeProvider{FixedTime: currentTime}

	parser := timeframe.NewTimeFrameParser(mockProvider)

	// Select today in UTC
	params := timeframe.TimeFrameParserParams{
		FromDate: "2025-11-29",
		ToDate:   "2025-11-29",
		Tz:       "UTC",
	}

	tf, err := parser.ParseTimeFrame(params)
	assert.NoError(t, err)

	// Generate time series points
	datePoints := tf.GenerateDateTimePointsReference()

	// Should have at least one point for today
	assert.GreaterOrEqual(t, len(datePoints), 1, "Should have at least one bucket for today")

	// The bucket should be for Nov 29
	firstPoint := datePoints[0]
	firstDate, err := time.Parse(time.RFC3339, firstPoint.UserFacingTimeFormat)
	assert.NoError(t, err)

	assert.Equal(t, 2025, firstDate.Year())
	assert.Equal(t, time.November, firstDate.Month())
	assert.Equal(t, 29, firstDate.Day())

	t.Logf("Bucket for today: %s (%s)", firstPoint.UserFacingTimeFormat, firstPoint.SQLiteBucketTimeFormat)
}
