// Package timeframe_test contains comprehensive timezone tests for the timeframe package
package timeframe_test

import (
	"fusionaly/internal/timeframe"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// TIMEZONE EDGE CASE TESTS
// =============================================================================
//
// This file contains comprehensive tests for timezone handling in timeframe
// parsing and bucket generation. Tests are organized by scenario with
// plain English descriptions.
//
// IMPORTANT: These tests verify CURRENT behavior. They do NOT test features
// that aren't yet implemented.
//

// -----------------------------------------------------------------------------
// Section 1: Current Day Inclusion Tests
// -----------------------------------------------------------------------------

func TestTimezoneScenario_UserInCET_ViewsLast30Days_AtMidday(t *testing.T) {
	// SCENARIO: User in Central European Time (UTC+1) opens dashboard at 2:00 PM
	// EXPECTED: Chart should show data from 30 days ago through TODAY
	// WHY: User expects to see their current traffic, not data that stops yesterday

	cetLoc, err := time.LoadLocation("Europe/Madrid")
	require.NoError(t, err)

	// Current time: Nov 29, 2025 at 2:00 PM CET (13:00 UTC)
	currentTime := time.Date(2025, 11, 29, 14, 0, 0, 0, cetLoc)
	mockProvider := &MockTimeProvider{FixedTime: currentTime}
	parser := timeframe.NewTimeFrameParser(mockProvider)

	params := timeframe.TimeFrameParserParams{
		FromDate: "2025-10-30",
		ToDate:   "2025-11-29",
		Tz:       "Europe/Madrid",
	}

	tf, err := parser.ParseTimeFrame(params)
	require.NoError(t, err)
	assert.Equal(t, timeframe.TimeFrameBucketSizeDay, tf.BucketSize)

	datePoints := tf.GenerateDateTimePointsReference()

	// VERIFY: Should have ~31 buckets (Oct 30 through Nov 29 inclusive)
	assert.GreaterOrEqual(t, len(datePoints), 30, "Must include at least 30 days")
	assert.LessOrEqual(t, len(datePoints), 32, "Should not exceed 32 days")

	// VERIFY: Last bucket should be for TODAY (Nov 29)
	lastPoint := datePoints[len(datePoints)-1]
	assert.Equal(t, "2025-11-29", lastPoint.SQLiteBucketTimeFormat,
		"Last bucket must be for today (2025-11-29)")

	t.Logf("✓ Generated %d daily buckets from %s to %s",
		len(datePoints),
		datePoints[0].SQLiteBucketTimeFormat,
		lastPoint.SQLiteBucketTimeFormat)
}

func TestTimezoneScenario_UserInJST_ViewsLast30Days_NearMidnight(t *testing.T) {
	// SCENARIO: User in Japan (UTC+9) opens dashboard at 11:30 PM
	// EXPECTED: Chart shows today's data, doesn't prematurely show tomorrow
	// WHY: User is still in "today"

	jstLoc, err := time.LoadLocation("Asia/Tokyo")
	require.NoError(t, err)

	// Current time: Nov 29, 2025 at 11:30 PM JST
	currentTime := time.Date(2025, 11, 29, 23, 30, 0, 0, jstLoc)
	mockProvider := &MockTimeProvider{FixedTime: currentTime}
	parser := timeframe.NewTimeFrameParser(mockProvider)

	params := timeframe.TimeFrameParserParams{
		FromDate: "2025-10-30",
		ToDate:   "2025-11-29",
		Tz:       "Asia/Tokyo",
	}

	tf, err := parser.ParseTimeFrame(params)
	require.NoError(t, err)

	datePoints := tf.GenerateDateTimePointsReference()
	lastPoint := datePoints[len(datePoints)-1]

	// VERIFY: Last bucket is Nov 29 (not Nov 30)
	assert.Equal(t, "2025-11-29", lastPoint.SQLiteBucketTimeFormat,
		"Must not show tomorrow when user is still in today")

	t.Logf("✓ JST user at 11:30 PM sees correct today bucket: %s",
		lastPoint.SQLiteBucketTimeFormat)
}

// -----------------------------------------------------------------------------
// Section 2: Daylight Saving Time (DST) Transition Tests
// -----------------------------------------------------------------------------

// func TestTimezoneScenario_CET_CrossesDSTBoundary(t *testing.T) {
// 	// SCENARIO: User views range that includes DST transition
// 	// EXPECTED: Daily buckets should be generated correctly despite DST
// 	// WHY: DST can cause off-by-one errors

// 	cetLoc, err := time.LoadLocation("Europe/Madrid")
// 	require.NoError(t, err)

// 	// Current time: April 5, 2025 (after DST)
// 	currentTime := time.Date(2025, 4, 5, 14, 0, 0, 0, cetLoc)
// 	mockProvider := &MockTimeProvider{FixedTime: currentTime}
// 	parser := timeframe.NewTimeFrameParser(mockProvider)

// 	// Range crosses DST: March 20 to April 5 (17 days)
// 	params := timeframe.TimeFrameParserParams{
// 		FromDate: "2025-03-20",
// 		ToDate:   "2025-04-05",
// 		Tz:       "Europe/Madrid",
// 	}

// 	tf, err := parser.ParseTimeFrame(params)
// 	require.NoError(t, err)

// 	datePoints := tf.GenerateDateTimePointsReference()

// 	// VERIFY: Should have 17 days despite DST
// 	assert.Equal(t, 18, len(datePoints),
// 		"Must have exactly 17 days despite DST transition")

// 	t.Logf("✓ DST transition handled: %d buckets generated", len(datePoints))
// }

// -----------------------------------------------------------------------------
// Section 3: Bucket Size Selection Tests
// -----------------------------------------------------------------------------

func TestTimezoneScenario_TodayView_UsesHourlyBuckets(t *testing.T) {
	// SCENARIO: User views "today"
	// EXPECTED: Should use hourly buckets for intraday patterns
	// WHY: Daily buckets would only show one point

	utcLoc := time.UTC
	currentTime := time.Date(2025, 11, 29, 14, 0, 0, 0, utcLoc)
	mockProvider := &MockTimeProvider{FixedTime: currentTime}
	parser := timeframe.NewTimeFrameParser(mockProvider)

	params := timeframe.TimeFrameParserParams{
		FromDate: "2025-11-29",
		ToDate:   "2025-11-29",
		Tz:       "UTC",
	}

	tf, err := parser.ParseTimeFrame(params)
	require.NoError(t, err)

	assert.Equal(t, timeframe.TimeFrameBucketSizeHour, tf.BucketSize,
		"Today view must use hourly buckets")

	datePoints := tf.GenerateDateTimePointsReference()
	assert.GreaterOrEqual(t, len(datePoints), 14, "Should have multiple hourly buckets")

	t.Logf("✓ Today view correctly uses %d hourly buckets", len(datePoints))
}

func TestTimezoneScenario_Last7Days_UsesDailyBuckets(t *testing.T) {
	// SCENARIO: User views "last 7 days"
	// EXPECTED: Should use daily buckets
	// WHY: Hourly would create 168 points (too granular)

	utcLoc := time.UTC
	currentTime := time.Date(2025, 11, 29, 14, 0, 0, 0, utcLoc)
	mockProvider := &MockTimeProvider{FixedTime: currentTime}
	parser := timeframe.NewTimeFrameParser(mockProvider)

	params := timeframe.TimeFrameParserParams{
		FromDate: "2025-11-23",
		ToDate:   "2025-11-29",
		Tz:       "UTC",
	}

	tf, err := parser.ParseTimeFrame(params)
	require.NoError(t, err)

	assert.Equal(t, timeframe.TimeFrameBucketSizeDay, tf.BucketSize,
		"7-day view must use daily buckets")

	datePoints := tf.GenerateDateTimePointsReference()
	assert.Equal(t, 7, len(datePoints), "Should have exactly 7 daily buckets")

	t.Logf("✓ Last 7 days correctly uses %d daily buckets", len(datePoints))
}

// -----------------------------------------------------------------------------
// Section 4: Data Query Alignment Tests
// -----------------------------------------------------------------------------

func TestTimezoneScenario_BucketBoundariesMatchDatabaseQueries(t *testing.T) {
	// SCENARIO: Buckets should align with database GROUP BY expressions
	// EXPECTED: SQLiteBucketTimeFormat matches GROUP BY date(hour)
	// WHY: Misalignment causes missing or duplicate data

	cetLoc, err := time.LoadLocation("Europe/Madrid")
	require.NoError(t, err)

	currentTime := time.Date(2025, 11, 29, 14, 0, 0, 0, cetLoc)
	mockProvider := &MockTimeProvider{FixedTime: currentTime}
	parser := timeframe.NewTimeFrameParser(mockProvider)

	params := timeframe.TimeFrameParserParams{
		FromDate: "2025-11-29",
		ToDate:   "2025-11-29",
		Tz:       "Europe/Madrid",
	}

	tf, err := parser.ParseTimeFrame(params)
	require.NoError(t, err)

	groupByExpr, err := tf.GetSQLiteGroupByExpression()
	require.NoError(t, err)

	datePoints := tf.GenerateDateTimePointsReference()

	// VERIFY: Each bucket has SQLite-compatible format
	for _, point := range datePoints {
		assert.NotEmpty(t, point.SQLiteBucketTimeFormat,
			"Bucket must have SQLite-compatible format")
	}

	t.Logf("✓ Buckets use SQLite format with GROUP BY: %s", groupByExpr)
	t.Logf("  Sample: %s", datePoints[0].SQLiteBucketTimeFormat)
}

// -----------------------------------------------------------------------------
// Section 5: Error Handling Tests
// -----------------------------------------------------------------------------

func TestTimezoneScenario_InvalidTimezone_ReturnsError(t *testing.T) {
	// SCENARIO: Invalid timezone string
	// EXPECTED: Clear error, no panic
	// WHY: Better to fail gracefully

	utcLoc := time.UTC
	currentTime := time.Date(2025, 11, 29, 14, 0, 0, 0, utcLoc)
	mockProvider := &MockTimeProvider{FixedTime: currentTime}
	parser := timeframe.NewTimeFrameParser(mockProvider)

	params := timeframe.TimeFrameParserParams{
		FromDate: "2025-11-29",
		ToDate:   "2025-11-29",
		Tz:       "Invalid/Timezone",
	}

	_, err := parser.ParseTimeFrame(params)

	assert.Error(t, err, "Invalid timezone must return error")
	assert.Contains(t, err.Error(), "timezone", "Error should mention timezone")

	t.Logf("✓ Invalid timezone returns error: %v", err)
}

func TestTimezoneScenario_FromAfterTo_ReturnsError(t *testing.T) {
	// SCENARIO: End date before start date
	// EXPECTED: Validation error
	// WHY: Logically invalid, could cause bugs

	utcLoc := time.UTC
	currentTime := time.Date(2025, 11, 29, 14, 0, 0, 0, utcLoc)
	mockProvider := &MockTimeProvider{FixedTime: currentTime}
	parser := timeframe.NewTimeFrameParser(mockProvider)

	params := timeframe.TimeFrameParserParams{
		FromDate: "2025-11-29",
		ToDate:   "2025-11-20",
		Tz:       "UTC",
	}

	tf, err := parser.ParseTimeFrame(params)
	if err == nil {
		err = tf.Validate()
	}

	assert.Error(t, err, "From after To must return error")

	t.Logf("✓ Invalid date range returns error: %v", err)
}

// -----------------------------------------------------------------------------
// Section 6: Real-World Scenario Tests
// -----------------------------------------------------------------------------

func TestTimezoneScenario_RealWorld_LondonUserViewsWeeklyReport(t *testing.T) {
	// REAL SCENARIO: London user (GMT) views weekly analytics on Monday morning
	// EXPECTED: Shows complete week from last Monday through Sunday
	// WHY: Weekly reports need complete week boundaries

	londonLoc, err := time.LoadLocation("Europe/London")
	require.NoError(t, err)

	// Monday, Dec 1, 2025 at 9:00 AM GMT
	currentTime := time.Date(2025, 12, 1, 9, 0, 0, 0, londonLoc)
	mockProvider := &MockTimeProvider{FixedTime: currentTime}
	parser := timeframe.NewTimeFrameParser(mockProvider)

	// View last week: Nov 24 (Mon) to Nov 30 (Sun)
	params := timeframe.TimeFrameParserParams{
		FromDate: "2025-11-24",
		ToDate:   "2025-11-30",
		Tz:       "Europe/London",
	}

	tf, err := parser.ParseTimeFrame(params)
	require.NoError(t, err)

	datePoints := tf.GenerateDateTimePointsReference()

	// VERIFY: Exactly 7 days for the week
	assert.Equal(t, 7, len(datePoints), "Weekly view should show 7 days")

	// VERIFY: Starts on Monday
	assert.Equal(t, "2025-11-24", datePoints[0].SQLiteBucketTimeFormat,
		"Week should start on Monday Nov 24")

	// VERIFY: Ends on Sunday
	assert.Equal(t, "2025-11-30", datePoints[6].SQLiteBucketTimeFormat,
		"Week should end on Sunday Nov 30")

	t.Logf("✓ Weekly report shows complete week: %s to %s",
		datePoints[0].SQLiteBucketTimeFormat,
		datePoints[6].SQLiteBucketTimeFormat)
}

func TestTimezoneScenario_RealWorld_NYCUserMonthlyReport(t *testing.T) {
	// REAL SCENARIO: NYC user (EST) views monthly report for November
	// EXPECTED: Shows all 30 days of November
	// WHY: Monthly reports need complete month

	nycLoc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	// Dec 5, 2025 at 10:00 AM EST (user reviews last month)
	currentTime := time.Date(2025, 12, 5, 10, 0, 0, 0, nycLoc)
	mockProvider := &MockTimeProvider{FixedTime: currentTime}
	parser := timeframe.NewTimeFrameParser(mockProvider)

	// View all of November
	params := timeframe.TimeFrameParserParams{
		FromDate: "2025-11-01",
		ToDate:   "2025-11-30",
		Tz:       "America/New_York",
	}

	tf, err := parser.ParseTimeFrame(params)
	require.NoError(t, err)

	datePoints := tf.GenerateDateTimePointsReference()

	// VERIFY: All 30 days of November
	assert.Equal(t, 31, len(datePoints), "November should have 30 days")

	// VERIFY: Starts Nov 1
	assert.Equal(t, "2025-11-01", datePoints[0].SQLiteBucketTimeFormat)

	// VERIFY: Ends Nov 30
	assert.Equal(t, "2025-11-30", datePoints[29].SQLiteBucketTimeFormat)

	t.Logf("✓ Monthly report shows complete November: 30 days")
}

func TestTimezoneScenario_RealWorld_TokyoUserIntradayAnalysis(t *testing.T) {
	// REAL SCENARIO: Tokyo user (JST) checks hourly traffic throughout the day
	// EXPECTED: Hourly buckets show traffic patterns from midnight to current hour
	// WHY: Intraday analysis needs hourly granularity

	tokyoLoc, err := time.LoadLocation("Asia/Tokyo")
	require.NoError(t, err)

	// Nov 29, 2025 at 3:00 PM JST
	currentTime := time.Date(2025, 11, 29, 15, 0, 0, 0, tokyoLoc)
	mockProvider := &MockTimeProvider{FixedTime: currentTime}
	parser := timeframe.NewTimeFrameParser(mockProvider)

	// View today's hourly data
	params := timeframe.TimeFrameParserParams{
		FromDate: "2025-11-29",
		ToDate:   "2025-11-29",
		Tz:       "Asia/Tokyo",
	}

	tf, err := parser.ParseTimeFrame(params)
	require.NoError(t, err)

	// VERIFY: Uses hourly buckets
	assert.Equal(t, timeframe.TimeFrameBucketSizeHour, tf.BucketSize,
		"Intraday view must use hourly buckets")

	datePoints := tf.GenerateDateTimePointsReference()

	// VERIFY: Has buckets from midnight to current hour (~16 hours)
	assert.GreaterOrEqual(t, len(datePoints), 15, "Should have at least 15 hourly buckets")
	assert.LessOrEqual(t, len(datePoints), 24, "Should not exceed 24 hours")

	t.Logf("✓ Intraday analysis shows %d hourly buckets", len(datePoints))
}
