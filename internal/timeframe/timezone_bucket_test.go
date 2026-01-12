package timeframe_test

import (
	"fusionaly/internal/timeframe"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTimezoneBucketTimestamps verifies that bucket timestamps represent
// the START of the period in UTC, not arbitrary times within the period.
// This prevents timezone display bugs where "2025-12-01T23:00:00Z" shows as "Dec 2nd"
func TestTimezoneBucketTimestamps_DailyBuckets(t *testing.T) {
	// SCENARIO: CET user (UTC+1) on Dec 1st, 2025 at 5:28 PM
	// Current time: 2025-12-01 17:28 CET = 2025-12-01 16:28 UTC

	cetLoc, err := time.LoadLocation("Europe/Madrid")
	require.NoError(t, err)

	// User's current time in CET
	currentTime := time.Date(2025, 12, 1, 17, 28, 0, 0, cetLoc)
	mockProvider := &MockTimeProvider{FixedTime: currentTime}
	parser := timeframe.NewTimeFrameParser(mockProvider)

	// User views "last 7 days" which should include today (Dec 1st)
	params := timeframe.TimeFrameParserParams{
		FromDate: "2025-11-25", // Nov 25
		ToDate:   "2025-12-01", // Dec 1 (today)
		Tz:       "Europe/Madrid",
	}

	tf, err := parser.ParseTimeFrame(params)
	require.NoError(t, err)
	assert.Equal(t, timeframe.TimeFrameBucketSizeDay, tf.BucketSize)

	datePoints := tf.GenerateDateTimePointsReference()

	// Verify we have the right number of days
	assert.GreaterOrEqual(t, len(datePoints), 7, "Should have at least 7 days")

	// CRITICAL TEST: Verify that Dec 1st bucket timestamp is at midnight UTC
	// NOT at 23:00 UTC which would display as Dec 2nd in CET
	lastPoint := datePoints[len(datePoints)-1]

	t.Logf("Last bucket SQLite format: %s", lastPoint.SQLiteBucketTimeFormat)
	t.Logf("Last bucket user-facing format: %s", lastPoint.UserFacingTimeFormat)

	// Should be Dec 1st
	assert.Equal(t, "2025-12-01", lastPoint.SQLiteBucketTimeFormat,
		"Last bucket SQL format should be 2025-12-01")

	// Parse the user-facing timestamp
	parsedTime, err := time.Parse(time.RFC3339, lastPoint.UserFacingTimeFormat)
	require.NoError(t, err)

	// CRITICAL: Should be midnight UTC, not 23:00 UTC
	assert.Equal(t, 0, parsedTime.Hour(),
		"User-facing timestamp should be at START of day (00:00 UTC), not end (23:00 UTC)")
	assert.Equal(t, 2025, parsedTime.Year())
	assert.Equal(t, time.December, parsedTime.Month())
	assert.Equal(t, 1, parsedTime.Day())

	// When frontend converts "2025-12-01T00:00:00Z" to CET:
	// 2025-12-01T00:00:00Z → 2025-12-01 01:00 CET (still Dec 1st) ✓
	// vs broken behavior:
	// 2025-12-01T23:00:00Z → 2025-12-02 00:00 CET (Dec 2nd!) ✗
	cetTime := parsedTime.In(cetLoc)
	assert.Equal(t, 1, cetTime.Day(), "When converted to CET, should still be Dec 1st")
}

func TestTimezoneBucketTimestamps_HourlyBuckets(t *testing.T) {
	// SCENARIO: User in JST (UTC+9) viewing hourly data
	jstLoc, err := time.LoadLocation("Asia/Tokyo")
	require.NoError(t, err)

	// Current time: 2025-12-01 14:00 JST = 2025-12-01 05:00 UTC
	currentTime := time.Date(2025, 12, 1, 14, 0, 0, 0, jstLoc)
	mockProvider := &MockTimeProvider{FixedTime: currentTime}
	parser := timeframe.NewTimeFrameParser(mockProvider)

	// View "today" (last 24 hours)
	params := timeframe.TimeFrameParserParams{
		FromDate: "2025-12-01",
		ToDate:   "2025-12-01",
		Tz:       "Asia/Tokyo",
	}

	tf, err := parser.ParseTimeFrame(params)
	require.NoError(t, err)

	// Should use hourly buckets for single day
	assert.Equal(t, timeframe.TimeFrameBucketSizeHour, tf.BucketSize)

	datePoints := tf.GenerateDateTimePointsReference()
	require.NotEmpty(t, datePoints)

	// Check first bucket
	firstPoint := datePoints[0]
	parsedTime, err := time.Parse(time.RFC3339, firstPoint.UserFacingTimeFormat)
	require.NoError(t, err)

	// Should be at start of hour (minute=0, second=0)
	assert.Equal(t, 0, parsedTime.Minute(), "Hourly bucket should be at start of hour")
	assert.Equal(t, 0, parsedTime.Second(), "Hourly bucket should have zero seconds")
}

func TestTimezoneBucketTimestamps_MonthlyBuckets(t *testing.T) {
	// SCENARIO: User viewing yearly data (monthly buckets)
	pstLoc, err := time.LoadLocation("America/Los_Angeles")
	require.NoError(t, err)

	currentTime := time.Date(2025, 12, 1, 10, 0, 0, 0, pstLoc)
	mockProvider := &MockTimeProvider{FixedTime: currentTime}
	parser := timeframe.NewTimeFrameParser(mockProvider)

	// View last 12 months
	params := timeframe.TimeFrameParserParams{
		FromDate: "2024-12-01",
		ToDate:   "2025-12-01",
		Tz:       "America/Los_Angeles",
	}

	tf, err := parser.ParseTimeFrame(params)
	require.NoError(t, err)
	assert.Equal(t, timeframe.TimeFrameBucketSizeMonth, tf.BucketSize)

	datePoints := tf.GenerateDateTimePointsReference()
	require.NotEmpty(t, datePoints)

	// Check first bucket (Dec 2024)
	firstPoint := datePoints[0]
	parsedTime, err := time.Parse(time.RFC3339, firstPoint.UserFacingTimeFormat)
	require.NoError(t, err)

	// Should be first day of month at midnight
	assert.Equal(t, 1, parsedTime.Day(), "Monthly bucket should be 1st of month")
	assert.Equal(t, 0, parsedTime.Hour(), "Monthly bucket should be at midnight")
	assert.Equal(t, time.December, parsedTime.Month())
	assert.Equal(t, 2024, parsedTime.Year())
}

// Removed TestTimezoneBucketTimestamps_AcrossMultipleTimezones - unrealistic test
// It expected all timezones to get identical buckets for same date range,
// but "Nov 29-Dec 1" in PST covers different UTC hours than in CET.
// The real bug (Dec 1 showing as Dec 2) is properly tested by:
// - TestTimezoneBucketTimestamps_DailyBuckets
// - TestTimezoneBucketTimestamps_HourlyBuckets
// - TestTimezoneBucketTimestamps_MonthlyBuckets
