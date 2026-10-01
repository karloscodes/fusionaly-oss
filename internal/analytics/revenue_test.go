package analytics_test

import (
	"log/slog"
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

func TestRevenueCurrencies(t *testing.T) {
	tf, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
		FromTime: at("0h"), ToTime: at("24h").Add(-time.Nanosecond), TimeFrameSize: timeframe.HourlyTimeFrame,
	}, time.UTC)
	require.NoError(t, err)
	params := analytics.WebsiteScopedQueryParams{WebsiteID: 1, TimeFrame: tf, Limit: 10}

	t.Run("with purchases in many currencies", func(t *testing.T) {
		db := setupFlowDB(t,
			purchase("u1", "revenue:purchased", `{"price":1000,"currency":"EUR"}`, at("10h")),
			purchase("u2", "revenue:purchased", `{"price":2000,"currency":"eur"}`, at("11h")),
			purchase("u3", "revenue:purchased", `{"price":9900,"currency":"USD"}`, at("12h")),
			purchase("u4", "revenue:purchased", `{"price":500,"quantity":2,"currency":"GBP"}`, at("13h")),
		)
		require.NoError(t, db.Create(&analytics.SiteStat{WebsiteID: 1, Visitors: 10, PageViews: 10, Sessions: 10, Hour: at("10h")}).Error)

		metrics, err := analytics.GetRevenueMetrics(db, params)
		require.NoError(t, err)
		chart, err := analytics.AggregatedRevenueInTimeFrame(db, params)
		require.NoError(t, err)
		top, err := analytics.GetTopRevenueEvents(db, params)
		require.NoError(t, err)
		totals, err := analytics.GetEventRevenueTotals(db, params)
		require.NoError(t, err)
		perVisitor, err := analytics.GetRevenuePerVisitor(db, params)
		require.NoError(t, err)

		t.Run("the main currency has the most purchases, in upper case", func(t *testing.T) {
			assert.Equal(t, "EUR", metrics.Currency)
		})

		t.Run("the tile counts only main currency purchases", func(t *testing.T) {
			assert.InDelta(t, 30.00, metrics.TotalRevenue, 0.001)
			assert.Equal(t, int64(2), metrics.TotalSales)
			assert.InDelta(t, 15.00, metrics.AverageOrderValue, 0.001)
		})

		t.Run("the other currencies have their totals and sale counts", func(t *testing.T) {
			assert.Equal(t, []analytics.CurrencyTotal{
				{Currency: "GBP", TotalRevenue: 10.00, TotalSales: 1},
				{Currency: "USD", TotalRevenue: 99.00, TotalSales: 1},
			}, metrics.OtherCurrencies)
		})

		t.Run("the conversion rate counts buyers in all currencies", func(t *testing.T) {
			assert.InDelta(t, 40.0, metrics.ConversionRate, 0.001)
		})

		t.Run("revenue per visitor counts only the main currency", func(t *testing.T) {
			assert.InDelta(t, 3.00, perVisitor, 0.001)
		})

		t.Run("the chart counts only main currency purchases", func(t *testing.T) {
			cents := 0
			for _, point := range chart {
				cents += point.Count
			}

			assert.Equal(t, 3000, cents)
		})

		t.Run("the top revenue events count only main currency purchases", func(t *testing.T) {
			require.Len(t, top, 1)
			assert.Equal(t, int64(2), top[0].Count)
		})

		t.Run("the event revenue total counts only main currency purchases", func(t *testing.T) {
			assert.InDelta(t, 30.00, totals["revenue:purchased"], 0.001)
		})
	})

	t.Run("with a tie, the main currency is the first in alphabetical order", func(t *testing.T) {
		db := setupFlowDB(t,
			purchase("u1", "revenue:purchased", `{"price":1000,"currency":"USD"}`, at("10h")),
			purchase("u2", "revenue:purchased", `{"price":2000,"currency":"EUR"}`, at("11h")),
		)

		metrics, err := analytics.GetRevenueMetrics(db, params)

		require.NoError(t, err)
		assert.Equal(t, "EUR", metrics.Currency)
		assert.InDelta(t, 20.00, metrics.TotalRevenue, 0.001)
	})

	t.Run("without a currency, a purchase is in USD", func(t *testing.T) {
		db := setupFlowDB(t,
			purchase("u1", "revenue:purchased", `{"price":1000}`, at("10h")),
			purchase("u2", "revenue:purchased", `{"price":2000,"currency":""}`, at("11h")),
			purchase("u3", "revenue:purchased", `{"price":4000,"currency":"EUR"}`, at("12h")),
		)

		metrics, err := analytics.GetRevenueMetrics(db, params)

		require.NoError(t, err)
		assert.Equal(t, "USD", metrics.Currency)
		assert.InDelta(t, 30.00, metrics.TotalRevenue, 0.001)
	})

	t.Run("without purchases, the currency is USD and there are no other currencies", func(t *testing.T) {
		db := setupFlowDB(t)

		metrics, err := analytics.GetRevenueMetrics(db, params)

		require.NoError(t, err)
		assert.Equal(t, "USD", metrics.Currency)
		assert.Empty(t, metrics.OtherCurrencies)
		assert.Zero(t, metrics.TotalRevenue)
	})
}

func TestRevenueComparisonUsesTheMainCurrency(t *testing.T) {
	tf := &timeframe.TimeFrame{From: at("24h"), To: at("48h").Add(-time.Nanosecond), BucketSize: timeframe.TimeFrameBucketSizeDay, Tz: time.UTC}
	db := setupFlowDB(t,
		purchase("u1", "revenue:purchased", `{"price":1000,"currency":"USD"}`, at("2h")),
		purchase("u2", "revenue:purchased", `{"price":500,"currency":"EUR"}`, at("3h")),
		purchase("u3", "revenue:purchased", `{"price":700,"currency":"EUR"}`, at("4h")),
	)
	current := &analytics.DashboardMetrics{RevenueMetrics: &analytics.RevenueMetrics{TotalRevenue: 20.00, Currency: "USD"}}

	comparison := analytics.FetchComparisonMetrics(db, tf, 1, current, slog.Default())

	require.NotNil(t, comparison.RevenueChange)
	assert.InDelta(t, 100.0, *comparison.RevenueChange, 0.001, "20 USD now against 10 USD before")
}

func TestRevenueDuplicateOrders(t *testing.T) {
	tf, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
		FromTime: at("0h"), ToTime: at("24h").Add(-time.Nanosecond), TimeFrameSize: timeframe.HourlyTimeFrame,
	}, time.UTC)
	require.NoError(t, err)
	params := analytics.WebsiteScopedQueryParams{WebsiteID: 1, TimeFrame: tf, Limit: 10}
	db := setupFlowDB(t,
		purchase("u1", "revenue:purchased", `{"price":1000,"order_id":"A1","currency":"USD"}`, at("10h")),
		purchase("u1", "revenue:purchased", `{"price":1000,"order_id":"A1","currency":"USD"}`, at("11h")),
		purchase("u1", "Revenue:Purchased", `{"price":1000,"order_id":"A1","currency":"USD"}`, at("12h")),
		purchase("u2", "revenue:purchased", `{"price":300,"currency":"USD"}`, at("13h")),
		purchase("u2", "revenue:purchased", `{"price":300,"currency":"USD"}`, at("13h")),
		purchase("u3", "revenue:purchased", `{"price":5000,"order_id":42,"currency":"USD"}`, at("-1h")),
		purchase("u3", "revenue:purchased", `{"price":5000,"order_id":42,"currency":"USD"}`, at("1h")),
	)

	metrics, err := analytics.GetRevenueMetrics(db, params)
	require.NoError(t, err)
	chart, err := analytics.AggregatedRevenueInTimeFrame(db, params)
	require.NoError(t, err)
	top, err := analytics.GetTopRevenueEvents(db, params)
	require.NoError(t, err)
	totals, err := analytics.GetEventRevenueTotals(db, params)
	require.NoError(t, err)

	t.Run("the tile counts each order once", func(t *testing.T) {
		assert.InDelta(t, 16.00, metrics.TotalRevenue, 0.001)
		assert.Equal(t, int64(3), metrics.TotalSales)
	})

	t.Run("the chart counts each order at its earliest event", func(t *testing.T) {
		require.Len(t, chart, 24, "one point per hour")

		assert.Equal(t, 1000, chart[10].Count)
		assert.Equal(t, 0, chart[11].Count)
		assert.Equal(t, 600, chart[13].Count)
		assert.Equal(t, 0, chart[1].Count, "the order came before the range")
	})

	t.Run("the top revenue events count each order once", func(t *testing.T) {
		require.Len(t, top, 1)
		assert.Equal(t, "revenue:purchased", top[0].Name)
		assert.Equal(t, int64(3), top[0].Count)
	})

	t.Run("the event revenue total counts each order once", func(t *testing.T) {
		assert.InDelta(t, 16.00, totals["revenue:purchased"], 0.001)
		assert.NotContains(t, totals, "Revenue:Purchased")
	})
}
