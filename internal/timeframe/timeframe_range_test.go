package timeframe_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/timeframe"
)

func TestTimeFrameRange(t *testing.T) {
	now := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	parser := timeframe.NewTimeFrameParser(&MockTimeProvider{FixedTime: now})

	t.Run("a long custom range ends on its last day, not at the end of that month", func(t *testing.T) {
		tf, err := parser.ParseTimeFrame(timeframe.TimeFrameParserParams{FromDate: "2026-01-31", ToDate: "2026-05-15", Tz: "UTC"})
		require.NoError(t, err)

		assert.True(t, tf.To.Equal(time.Date(2026, 5, 15, 23, 59, 59, 999999999, time.UTC)), "To: %s", tf.To)
	})

	t.Run("a long range that starts late in a month charts every month", func(t *testing.T) {
		tf, err := parser.ParseTimeFrame(timeframe.TimeFrameParserParams{FromDate: "2026-01-31", ToDate: "2026-05-15", Tz: "UTC"})
		require.NoError(t, err)

		var months []string
		for _, p := range tf.GenerateDateTimePointsReference() {
			months = append(months, p.SQLiteBucketTimeFormat)
		}

		assert.Equal(t, []string{"2026-01", "2026-02", "2026-03", "2026-04", "2026-05"}, months)
	})

	t.Run("year to date ends now, not at the end of this month", func(t *testing.T) {
		tf, err := parser.ParseTimeFrame(timeframe.TimeFrameParserParams{FromDate: "2026-01-01", ToDate: "2026-10-01", Tz: "UTC"})
		require.NoError(t, err)

		assert.True(t, tf.To.Equal(now.Add(timeframe.TimeWindowBuffer)), "To: %s", tf.To)
	})
}
