// Package timeframe_test contains tests for the timeframe package
package timeframe_test

import (
	"fusionaly/internal/timeframe"

	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTimeProvider implements the TimeProvider interface for testing
type TestTimeProvider struct {
	CurrentTime time.Time
}

// Now returns the fixed test time, allowing stable tests with predictable times
func (t *TestTimeProvider) Now(loc *time.Location) time.Time {
	return t.CurrentTime.In(loc)
}

// Helper function to load location and handle errors
func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic("Failed to load time zone location: " + name)
	}
	return loc
}

// TestTimeFrameParserDayBoundaryFix tests the fix for the issue where
// TimeWindowBuffer could push the end time into the next day, causing
// future dates to appear in daily bucket views.
func TestTimeFrameParserDayBoundaryFix(t *testing.T) {
	testCases := []struct {
		name        string
		fixedTime   time.Time
		timezone    string
		fromDate    string
		toDate      string
		expectedDay string // Expected day for the "to" date in YYYY-MM-DD format
		description string
	}{
		{
			name:        "Late night UTC - should not spill into next day",
			fixedTime:   time.Date(2025, 10, 6, 23, 58, 0, 0, time.UTC), // 23:58 UTC
			timezone:    "UTC",
			fromDate:    "2025-09-06", // 30 days ago
			toDate:      "2025-10-06", // today
			expectedDay: "2025-10-06", // Should stay within Oct 6
			description: "When it's late at night (23:58) UTC, adding 5min buffer should not cross day boundary",
		},
		{
			name:        "Late night in positive timezone - should not spill into next day",
			fixedTime:   time.Date(2025, 10, 6, 21, 58, 0, 0, mustLoadLocation("Europe/Berlin")), // 21:58 Berlin time (23:58 UTC)
			timezone:    "Europe/Berlin",
			fromDate:    "2025-09-06",
			toDate:      "2025-10-06",
			expectedDay: "2025-10-06",
			description: "When it's late at night Berlin time, should not cross day boundary even after timezone conversion",
		},
		{
			name:        "Safe time - buffer should work normally",
			fixedTime:   time.Date(2025, 10, 6, 12, 0, 0, 0, time.UTC), // 12:00 UTC
			timezone:    "UTC",
			fromDate:    "2025-09-06",
			toDate:      "2025-10-06",
			expectedDay: "2025-10-06",
			description: "During normal hours, buffer should work as expected without crossing boundaries",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a time provider with our fixed time
			timeProvider := &TestTimeProvider{CurrentTime: tc.fixedTime}
			parser := timeframe.NewTimeFrameParser(timeProvider)

			// Parse the timeframe
			tf, err := parser.ParseTimeFrame(timeframe.TimeFrameParserParams{
				FromDate: tc.fromDate,
				ToDate:   tc.toDate,
				Tz:       tc.timezone,
			})

			require.NoError(t, err, "Failed to parse timeframe for test case: %s", tc.description)
			require.NotNil(t, tf, "Timeframe should not be nil")

			// Convert the To time back to the user's timezone to check the day
			loc := mustLoadLocation(tc.timezone)
			toTimeInUserTZ := tf.To.In(loc)

			actualDay := toTimeInUserTZ.Format("2006-01-02")

			t.Logf("Test: %s", tc.description)
			t.Logf("Fixed time: %s", tc.fixedTime.Format(time.RFC3339))
			t.Logf("TimeFrame.From (UTC): %s", tf.From.Format(time.RFC3339))
			t.Logf("TimeFrame.To (UTC): %s", tf.To.Format(time.RFC3339))
			t.Logf("TimeFrame.To (user tz): %s", toTimeInUserTZ.Format(time.RFC3339))
			t.Logf("Bucket size: %s", tf.BucketSize)
			t.Logf("Expected day: %s, Actual day: %s", tc.expectedDay, actualDay)

			// Check if this range would result in daily buckets (which is where the issue manifests)
			daysDifference := int(tf.To.Sub(tf.From).Hours() / 24)
			t.Logf("Days difference: %d", daysDifference)

			// For the 30-day range issue you're experiencing, we should have daily buckets
			// and the end date should not spill into the next day when viewed in user timezone
			if daysDifference >= 30 {
				// This should use daily buckets, and end time shouldn't cross date boundary
				assert.True(t, toTimeInUserTZ.Format("2006-01-02") <= tc.expectedDay,
					"For daily bucket ranges, TimeFrame end should not extend beyond %s in user timezone. Got: %s (from UTC: %s)",
					tc.expectedDay, actualDay, tf.To.Format(time.RFC3339))
			} else {
				// For shorter ranges (hourly buckets), we're more lenient
				t.Logf("Short range detected, using more lenient day boundary check")
				// Still shouldn't go too far beyond
				assert.True(t, toTimeInUserTZ.Format("2006-01-02") <= tc.expectedDay,
					"Even for hourly buckets, shouldn't extend way beyond expected day. Got: %s", actualDay)
			}
		})
	}
}

// TestTimeFrameParserOriginalBehavior ensures that our fix doesn't break
// the intended TimeWindowBuffer behavior for hourly buckets
func TestTimeFrameParserOriginalBehavior(t *testing.T) {
	// Test with a time range that should use hourly buckets (< 7 days)
	fixedTime := time.Date(2025, 10, 6, 14, 30, 0, 0, time.UTC)
	timeProvider := &TestTimeProvider{CurrentTime: fixedTime}
	parser := timeframe.NewTimeFrameParser(timeProvider)

	// Parse a short timeframe that should use hourly buckets
	tf, err := parser.ParseTimeFrame(timeframe.TimeFrameParserParams{
		FromDate: "2025-10-06", // same day
		ToDate:   "2025-10-06", // same day
		Tz:       "UTC",
	})

	require.NoError(t, err)
	require.NotNil(t, tf)

	// For hourly buckets and same-day ranges, the TimeWindowBuffer should still work
	// The end time should be current time + buffer, but within the day
	expectedMinTime := fixedTime
	expectedMaxTime := time.Date(2025, 10, 6, 23, 59, 59, 999999999, time.UTC) // end of day

	assert.True(t, tf.To.After(expectedMinTime) || tf.To.Equal(expectedMinTime),
		"TimeFrame end should be at least the current time")
	assert.True(t, tf.To.Before(expectedMaxTime) || tf.To.Equal(expectedMaxTime),
		"TimeFrame end should not exceed end of day")

	t.Logf("Fixed time: %s", fixedTime.Format(time.RFC3339))
	t.Logf("TimeFrame.To: %s", tf.To.Format(time.RFC3339))
	t.Logf("Bucket size: %s", tf.BucketSize)
}
