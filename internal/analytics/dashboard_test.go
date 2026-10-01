package analytics_test

import (
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/analytics"
	"fusionaly/internal/events"
	"fusionaly/internal/timeframe"
	"fusionaly/internal/websites"
)

func TestDashboardVisitorNumbers(t *testing.T) {
	today := &timeframe.TimeFrame{From: at("24h"), To: at("48h").Add(-time.Nanosecond), BucketSize: timeframe.TimeFrameBucketSizeHour, Tz: time.UTC}
	evts := []events.Event{
		purchase("t1", "revenue:purchased", `{"price":1000,"currency":"EUR"}`, at("34h")),
		purchase("t2", "revenue:purchased", `{"price":3000,"currency":"EUR"}`, at("35h")),
		purchase("y1", "revenue:purchased", `{"price":2000,"currency":"EUR"}`, at("10h")),
	}
	for i := 1; i <= 4; i++ {
		evts = append(evts, pageView(fmt.Sprintf("t%d", i), "/", at("33h")))
	}
	for i := 1; i <= 10; i++ {
		evts = append(evts, pageView(fmt.Sprintf("y%d", i), "/", at("09h")))
	}
	db := setupFlowDB(t, evts...)
	require.NoError(t, db.Create(&websites.Website{ID: 1, Domain: "example.com", CreatedAt: at("0h")}).Error)

	metrics, err := analytics.FetchDashboardMetrics(db, today, 1, slog.Default())
	require.NoError(t, err)
	comparison := analytics.FetchComparisonMetrics(db, today, 1, metrics, slog.Default())

	t.Run("the total and the chart count the same visitors", func(t *testing.T) {
		charted := 0
		for _, point := range metrics.Visitors {
			charted += point.Count
		}

		assert.Equal(t, int64(4), metrics.TotalVisitors)
		assert.Equal(t, 4, charted)
	})

	t.Run("the conversion rate is the share of visitors who bought", func(t *testing.T) {
		assert.InDelta(t, 50.0, metrics.RevenueMetrics.ConversionRate, 0.001, "2 of 4 visitors bought")
	})

	t.Run("revenue per visitor divides the revenue by the visitors", func(t *testing.T) {
		assert.InDelta(t, 10.0, metrics.RevenuePerVisitor, 0.001, "40 EUR for 4 visitors")
	})

	t.Run("the comparison counts the visitors of the previous period", func(t *testing.T) {
		require.NotNil(t, comparison.VisitorsChange)
		assert.InDelta(t, -60.0, *comparison.VisitorsChange, 0.001, "4 visitors now against 10 before")
	})

	t.Run("the comparison counts the revenue of the previous period", func(t *testing.T) {
		require.NotNil(t, comparison.RevenueChange)
		assert.InDelta(t, 100.0, *comparison.RevenueChange, 0.001, "40 EUR now against 20 EUR before")
	})
}
