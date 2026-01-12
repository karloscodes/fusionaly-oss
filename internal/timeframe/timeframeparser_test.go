package timeframe_test

import (
	"fusionaly/internal/timeframe"

	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeFrameParserComprehensive(t *testing.T) {
	// Create a fixed time for testing - 2024-07-15 14:30:00 UTC (Monday)
	fixedTime := time.Date(2024, 7, 15, 14, 30, 0, 0, time.UTC)

	// Create a time provider that returns our fixed time
	timeProvider := &TestTimeProvider{CurrentTime: fixedTime}

	// Create a parser with our time provider
	parser := timeframe.NewTimeFrameParser(timeProvider)

	// Note on TimeWindowBuffer:
	// The TimeFrameParser uses a unified 5-minute buffer (TimeWindowBuffer) for all ongoing
	// time ranges to ensure we include the most recent data points. This approach simplifies
	// the previous dual mechanism (DataStabilizationDelay and buffer time) into a single concept.
	// For test expectations, this means the end time for ongoing periods will be fixedTime + TimeWindowBuffer.

	// Table-driven tests for all range labels
	testCases := []struct {
		name           string
		params         timeframe.TimeFrameParserParams
		expectedFrom   time.Time
		expectedTo     time.Time
		expectedBucket timeframe.TimeFrameBucketSize
		expectError    bool
	}{
		// Today Range Tests
		{
			name: "Today Range - UTC",
			params: timeframe.TimeFrameParserParams{
				FromDate: "2024-07-15",
				ToDate:   "2024-07-15",
				Tz:       "UTC",
			},
			expectedFrom:   time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC),
			expectedTo:     time.Date(2024, 7, 15, 14, 59, 59, 0, time.UTC), // Truncated to hour 14:00 + 1 hour - 1 second
			expectedBucket: timeframe.TimeFrameBucketSizeHour,
		},
		{
			name: "Today Range - America/New_York",
			params: timeframe.TimeFrameParserParams{
				FromDate: "2024-07-15",
				ToDate:   "2024-07-15",
				Tz:       "America/New_York",
			},
			expectedFrom:   time.Date(2024, 7, 15, 0, 0, 0, 0, mustLoadLocation("America/New_York")).UTC(),
			expectedTo:     time.Date(2024, 7, 15, 14, 59, 59, 0, time.UTC), // Truncated to hour 14:00 + 1 hour - 1 second, in UTC
			expectedBucket: timeframe.TimeFrameBucketSizeHour,
		},

		// Yesterday Range Tests
		{
			name: "Yesterday Range - UTC",
			params: timeframe.TimeFrameParserParams{
				FromDate: "2024-07-14",
				ToDate:   "2024-07-14",
				Tz:       "UTC",
			},
			expectedFrom:   time.Date(2024, 7, 14, 0, 0, 0, 0, time.UTC),
			expectedTo:     time.Date(2024, 7, 14, 23, 59, 59, 999999999, time.UTC),
			expectedBucket: timeframe.TimeFrameBucketSizeHour,
		},
		{
			name: "Yesterday Range - Europe/Berlin",
			params: timeframe.TimeFrameParserParams{
				FromDate: "2024-07-14",
				ToDate:   "2024-07-14",
				Tz:       "Europe/Berlin",
			},
			expectedFrom:   time.Date(2024, 7, 14, 0, 0, 0, 0, mustLoadLocation("Europe/Berlin")).UTC(),
			expectedTo:     time.Date(2024, 7, 14, 23, 59, 59, 999999999, mustLoadLocation("Europe/Berlin")).UTC(),
			expectedBucket: timeframe.TimeFrameBucketSizeHour,
		},

		// Last 7 Days Range Tests (7 days ago to current time)
		{
			name: "Last 7 Days - UTC",
			params: timeframe.TimeFrameParserParams{
				FromDate: "2024-07-08",
				ToDate:   "", // Empty ToDate means "now"
				Tz:       "UTC",
			},
			expectedFrom:   time.Date(2024, 7, 8, 0, 0, 0, 0, time.UTC),
			expectedTo:     time.Date(2024, 7, 15, 23, 59, 59, 0, time.UTC), // Truncated to day boundary + 1 day - 1 second
			expectedBucket: timeframe.TimeFrameBucketSizeDay,
		},

		// Last 30 Days Range Tests (30 days ago to current time)
		{
			name: "Last 30 Days - UTC",
			params: timeframe.TimeFrameParserParams{
				FromDate: "2024-06-15",
				ToDate:   "", // Empty ToDate means "now"
				Tz:       "UTC",
			},
			expectedFrom:   time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
			expectedTo:     time.Date(2024, 7, 15, 23, 59, 59, 0, time.UTC), // Truncated to day boundary + 1 day - 1 second
			expectedBucket: timeframe.TimeFrameBucketSizeDay,
		},

		// Month to Date Range Tests (start of current month to current time)
		{
			name: "Month to Date - UTC",
			params: timeframe.TimeFrameParserParams{
				FromDate: "2024-07-01",
				ToDate:   "", // Empty ToDate means "now"
				Tz:       "UTC",
			},
			expectedFrom:   time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
			expectedTo:     time.Date(2024, 7, 15, 23, 59, 59, 0, time.UTC), // Truncated to day boundary + 1 day - 1 second
			expectedBucket: timeframe.TimeFrameBucketSizeDay,
		},

		// Last Month Range Tests (start of last month to end of last month)
		{
			name: "Last Month - UTC",
			params: timeframe.TimeFrameParserParams{
				FromDate: "2024-06-01",
				ToDate:   "2024-06-30",
				Tz:       "UTC",
			},
			expectedFrom:   time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
			expectedTo:     time.Date(2024, 6, 30, 23, 59, 59, 999999999, time.UTC),
			expectedBucket: timeframe.TimeFrameBucketSizeDay,
		},

		// Year to Date Range Tests (start of current year to current time)
		{
			name: "Year to Date - UTC",
			params: timeframe.TimeFrameParserParams{
				FromDate: "2024-01-01",
				ToDate:   "2024-07-15", // Use explicit date matching fixed time
				Tz:       "UTC",
			},
			expectedFrom:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			expectedTo:     time.Date(2024, 7, 31, 23, 59, 59, 0, time.UTC), // Monthly bucket: truncated to July 1st + 1 month - 1 second
			expectedBucket: timeframe.TimeFrameBucketSizeMonth,              // ~6 months = monthly bucket
		},

		// Last 12 Months Range Tests (12 months ago to current time)
		{
			name: "Last 12 Months - UTC",
			params: timeframe.TimeFrameParserParams{
				FromDate: "2023-07-15",
				ToDate:   "2024-07-15", // Use explicit date matching fixed time
				Tz:       "UTC",
			},
			expectedFrom:   time.Date(2023, 7, 15, 0, 0, 0, 0, time.UTC),    // User-specified date, no truncation
			expectedTo:     time.Date(2024, 7, 31, 23, 59, 59, 0, time.UTC), // Monthly bucket: truncated to July 1st + 1 month - 1 second
			expectedBucket: timeframe.TimeFrameBucketSizeMonth,              // ~12 months = monthly bucket
		},

		// All Time Range Tests
		{
			name: "All Time with First Event - UTC",
			params: timeframe.TimeFrameParserParams{
				FromDate:            "2022-01-01",
				ToDate:              "2025-12-31",
				Tz:                  "UTC",
				AllTimeFirstEventAt: time.Date(2022, 1, 15, 0, 0, 0, 0, time.UTC),
			},
			expectedFrom:   time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
			expectedTo:     time.Date(2024, 7, 31, 23, 59, 59, 0, time.UTC), // Future date clamped to current time + buffer, then monthly truncation
			expectedBucket: timeframe.TimeFrameBucketSizeMonth,
		},

		// Custom Date Range Tests
		{
			name: "Custom Date Range - Specific Dates",
			params: timeframe.TimeFrameParserParams{
				FromDate: "2024-07-01",
				ToDate:   "2024-07-10",
				Tz:       "UTC",
			},
			expectedFrom:   time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
			expectedTo:     time.Date(2024, 7, 10, 23, 59, 59, 999999999, time.UTC),
			expectedBucket: timeframe.TimeFrameBucketSizeDay,
		},

		// Error test cases
		{
			name: "Invalid Timezone",
			params: timeframe.TimeFrameParserParams{
				FromDate: "2024-07-15",
				ToDate:   "2024-07-15",
				Tz:       "Invalid/Timezone",
			},
			expectError: true,
		},
		{
			name: "Invalid Date Format",
			params: timeframe.TimeFrameParserParams{
				FromDate: "invalid_date",
				ToDate:   "2024-07-15",
				Tz:       "UTC",
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Parse time frame
			tf, err := parser.ParseTimeFrame(tc.params)

			// Check error expectations
			if tc.expectError {
				assert.Error(t, err)
				return
			}

			// Assert no error if we don't expect one
			require.NoError(t, err)

			// Assert time frame properties with tolerance
			if tc.expectedBucket != "" {
				assert.Equal(t, tc.expectedBucket, tf.BucketSize, "Bucket size should match expected")
			}

			// Calculate the difference in seconds between expected and actual times
			fromDiff := tc.expectedFrom.Sub(tf.From).Seconds()
			toDiff := tc.expectedTo.Sub(tf.To).Seconds()

			// Allow a 2-second tolerance for time differences
			const maxAllowedDiffSeconds = 2

			assert.InDelta(t, 0, fromDiff, maxAllowedDiffSeconds,
				"From time should be within %v second(s) of expected. Expected: %v, Got: %v",
				maxAllowedDiffSeconds, tc.expectedFrom, tf.From)

			assert.InDelta(t, 0, toDiff, maxAllowedDiffSeconds,
				"To time should be within %v second(s) of expected. Expected: %v, Got: %v",
				maxAllowedDiffSeconds, tc.expectedTo, tf.To)
		})
	}
}

// TestTimeFrameParserTimeWindowBuffer tests the TimeWindowBuffer constant
func TestTimeFrameParserTimeWindowBuffer(t *testing.T) {
	// Verify that the TimeWindowBuffer constant is positive
	assert.True(t, timeframe.TimeWindowBuffer > 0,
		"TimeWindowBuffer should be positive to provide a buffer for ongoing timeframes")

	// TimeWindowBuffer explanation:
	// A positive buffer (5 minutes) extends the time window for ongoing ranges to include
	// recent data that might still be incoming. This unified approach:
	//   1. Ensures we don't miss recent events at time boundaries
	//   2. Compensates for slight delays in data collection and processing
	//   3. Handles clock synchronization issues across distributed systems
	//   4. Provides a simpler, more intuitive time handling mechanism

	// Create a fixed time for testing
	fixedTime := time.Date(2024, 7, 15, 14, 30, 0, 0, time.UTC)

	// Create a time provider that returns our fixed time
	timeProvider := &TestTimeProvider{CurrentTime: fixedTime}

	// Create a parser with our time provider
	parser := timeframe.NewTimeFrameParser(timeProvider)

	// Parse a "Last 7 Days" time frame which should use the buffer
	tf, err := parser.ParseTimeFrame(timeframe.TimeFrameParserParams{
		FromDate: "2024-07-08",
		ToDate:   "", // Empty ToDate means "now"
		Tz:       "UTC",
	})

	// Verify no error
	require.NoError(t, err)

	// Check bucket size is Daily
	assert.Equal(t, timeframe.TimeFrameBucketSizeDay, tf.BucketSize, "Last7Days should use daily bucketing")

	// The "To" time should be truncated to day boundary + 1 day - 1 second
	expectedStart := time.Date(2024, 7, 8, 0, 0, 0, 0, time.UTC)
	expectedEnd := time.Date(2024, 7, 15, 23, 59, 59, 0, time.UTC)

	// Allow a 1-second tolerance for time comparisons
	const tolerance = time.Second
	assert.WithinDuration(t, expectedStart, tf.From, tolerance,
		"The From time should be 7 days before the current time")
	assert.WithinDuration(t, expectedEnd, tf.To, tolerance,
		"The To time should be truncated to day boundary + 1 day - 1 second")
}
