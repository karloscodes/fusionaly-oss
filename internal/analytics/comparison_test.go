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

		prevFrom, prevTo := analytics.PreviousPeriod(from, to, madrid)

		assert.True(t, prevFrom.Equal(time.Date(2026, 8, 2, 0, 0, 0, 0, madrid)), "starts on a day boundary: %s", prevFrom)
		assert.Equal(t, time.Nanosecond, from.Sub(prevTo), "ends just before the current period starts")
	})

	t.Run("today so far compares with yesterday up to the same time", func(t *testing.T) {
		from := time.Date(2026, 9, 25, 0, 0, 0, 0, madrid)
		to := time.Date(2026, 9, 25, 9, 5, 0, 0, madrid)

		prevFrom, prevTo := analytics.PreviousPeriod(from, to, madrid)

		assert.True(t, prevFrom.Equal(time.Date(2026, 9, 24, 0, 0, 0, 0, madrid)), "yesterday morning: %s", prevFrom)
		assert.True(t, prevTo.Equal(time.Date(2026, 9, 24, 9, 5, 0, 0, madrid)), "not yesterday evening: %s", prevTo)
	})

	t.Run("the last 7 days so far compare with the 7 days before, up to the same time", func(t *testing.T) {
		from := time.Date(2026, 9, 19, 0, 0, 0, 0, madrid)
		to := time.Date(2026, 9, 25, 0, 35, 0, 0, madrid)

		prevFrom, prevTo := analytics.PreviousPeriod(from, to, madrid)

		assert.True(t, prevFrom.Equal(time.Date(2026, 9, 12, 0, 0, 0, 0, madrid)), "%s", prevFrom)
		assert.True(t, prevTo.Equal(time.Date(2026, 9, 18, 0, 35, 0, 0, madrid)), "%s", prevTo)
	})

	t.Run("a day across the end of daylight saving shifts by calendar days", func(t *testing.T) {
		from := time.Date(2026, 10, 26, 0, 0, 0, 0, madrid)
		to := time.Date(2026, 10, 26, 23, 59, 59, 999999999, madrid)

		prevFrom, prevTo := analytics.PreviousPeriod(from, to, madrid)

		assert.True(t, prevFrom.Equal(time.Date(2026, 10, 25, 0, 0, 0, 0, madrid)), "%s", prevFrom)
		assert.Equal(t, time.Nanosecond, from.Sub(prevTo))
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

	t.Run("no change when the previous period is too small to compare", func(t *testing.T) {
		got := analytics.CalculateComparisonMetrics(analytics.ComparisonData{
			CurrentVisitors: 3, PreviousVisitors: 1,
			CurrentViews: 9, PreviousViews: 4,
			CurrentSessions: 3, PreviousSessions: 1,
			CurrentBounceRate: 0.5, PreviousBounceRate: 1,
			CurrentAvgTime: 60, PreviousAvgTime: 10,
		})

		assert.Nil(t, got.VisitorsChange, "1 to 3 is not +200%")
		assert.Nil(t, got.ViewsChange)
		assert.Nil(t, got.SessionsChange)
		assert.Nil(t, got.BounceRateChange)
		assert.Nil(t, got.AvgTimeChange)
	})
}
