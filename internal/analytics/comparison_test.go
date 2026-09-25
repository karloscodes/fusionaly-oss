package analytics_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/analytics"
)

func TestPreviousPeriod(t *testing.T) {
	madrid, err := time.LoadLocation("Europe/Madrid")
	require.NoError(t, err)

	t.Run("the 30 days before a 30-day range, with no shared moment", func(t *testing.T) {
		from := time.Date(2026, 9, 1, 0, 0, 0, 0, madrid)
		to := time.Date(2026, 9, 30, 23, 59, 59, 999999999, madrid)

		prevFrom, prevTo := analytics.PreviousPeriod(from, to)

		assert.True(t, prevFrom.Equal(time.Date(2026, 8, 2, 0, 0, 0, 0, madrid)), "starts on a bucket boundary: %s", prevFrom)
		assert.True(t, prevTo.Before(from), "ends before the current period starts")
		assert.Equal(t, time.Nanosecond, from.Sub(prevTo))
	})

	t.Run("a range that ends now compares with the same length before it", func(t *testing.T) {
		from := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
		to := time.Date(2026, 9, 25, 14, 30, 0, 0, time.UTC)

		prevFrom, prevTo := analytics.PreviousPeriod(from, to)

		assert.Equal(t, to.Sub(from), prevTo.Add(time.Nanosecond).Sub(prevFrom))
	})
}

func TestCalculateComparisonMetrics(t *testing.T) {
	t.Run("bounce rate changes in percentage points", func(t *testing.T) {
		got := analytics.CalculateComparisonMetrics(analytics.ComparisonData{
			CurrentSessions: 100, PreviousSessions: 100,
			CurrentBounceRate: 0.46, PreviousBounceRate: 0.40,
		})

		require.NotNil(t, got.BounceRateChange)
		assert.InDelta(t, 6.0, *got.BounceRateChange, 0.0001, "40% to 46% is +6 points, not +15%")
	})

	t.Run("a bounce rate of 0% is still a rate to compare with", func(t *testing.T) {
		got := analytics.CalculateComparisonMetrics(analytics.ComparisonData{
			CurrentSessions: 10, PreviousSessions: 10,
			CurrentBounceRate: 0.2, PreviousBounceRate: 0,
		})

		require.NotNil(t, got.BounceRateChange)
		assert.InDelta(t, 20.0, *got.BounceRateChange, 0.0001)
	})

	t.Run("no change without visits in the previous period", func(t *testing.T) {
		got := analytics.CalculateComparisonMetrics(analytics.ComparisonData{
			CurrentVisitors: 50, CurrentSessions: 60, CurrentBounceRate: 0.5,
		})

		assert.Nil(t, got.VisitorsChange)
		assert.Nil(t, got.BounceRateChange)
	})

	t.Run("visitors change is relative", func(t *testing.T) {
		got := analytics.CalculateComparisonMetrics(analytics.ComparisonData{CurrentVisitors: 150, PreviousVisitors: 100})

		require.NotNil(t, got.VisitorsChange)
		assert.InDelta(t, 50.0, *got.VisitorsChange, 0.0001)
	})
}
