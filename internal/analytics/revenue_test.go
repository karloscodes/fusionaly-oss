package analytics_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/analytics"
	"fusionaly/internal/events"
	"fusionaly/internal/timeframe"
)

func purchase(user, name, meta string, ts time.Time) events.Event {
	return events.Event{WebsiteID: 1, UserSignature: user, Hostname: "example.com", Pathname: "/checkout",
		EventType: events.EventTypeCustomEvent, CustomEventName: name, CustomEventMeta: meta, Timestamp: ts}
}

func TestRevenue(t *testing.T) {
	tf, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
		FromTime: at("0h"), ToTime: at("24h").Add(-time.Nanosecond), TimeFrameSize: timeframe.HourlyTimeFrame,
	}, time.UTC)
	require.NoError(t, err)
	params := analytics.WebsiteScopedQueryParams{WebsiteID: 1, TimeFrame: tf, Limit: 10}
	db := setupFlowDB(t,
		purchase("u1", "revenue:purchased", `{"price":1000,"quantity":3,"currency":"USD"}`, at("10h")),
		purchase("u1", "Revenue:Purchased", `{"price":500,"currency":"USD"}`, at("11h")),
		purchase("u2", "revenue:purchased", `{"price":0,"currency":"USD"}`, at("12h")),
		purchase("u2", "revenue:purchased", `not json`, at("12h")),
		pageView("u1", "/", at("10h")),
		pageView("u2", "/", at("10h")),
	)

	metrics, err := analytics.GetRevenueMetrics(db, params)
	require.NoError(t, err)
	chart, err := analytics.AggregatedRevenueInTimeFrame(db, params)
	require.NoError(t, err)
	top, err := analytics.GetTopRevenueEvents(db, params)
	require.NoError(t, err)

	t.Run("the tile counts price times quantity of valid purchases", func(t *testing.T) {
		assert.InDelta(t, 35.00, metrics.TotalRevenue, 0.001)
		assert.Equal(t, int64(2), metrics.TotalSales)
	})

	t.Run("the chart adds up to the tile", func(t *testing.T) {
		cents := 0
		for _, point := range chart {
			cents += point.Count
		}

		assert.Equal(t, 3500, cents)
	})

	t.Run("the top revenue events count the same purchases", func(t *testing.T) {
		total := int64(0)
		for _, r := range top {
			total += r.Count
		}

		assert.Equal(t, int64(2), total)
	})

	t.Run("the conversion rate counts each buying visitor once", func(t *testing.T) {
		assert.InDelta(t, 50.0, metrics.ConversionRate, 0.001, "1 of 2 visitors bought, twice")
	})
}
