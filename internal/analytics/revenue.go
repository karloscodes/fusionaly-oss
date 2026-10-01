package analytics

import (
	"fmt"
	"strings"
	"time"

	"fusionaly/internal/events"
	"fusionaly/internal/timeframe"

	"gorm.io/gorm"
)

// RevenueMetrics holds revenue-related metrics. The totals count only
// purchases in the main currency. OtherCurrencies lists the rest.
type RevenueMetrics struct {
	TotalRevenue      float64         `json:"total_revenue"`
	TotalSales        int64           `json:"total_sales"`
	AverageOrderValue float64         `json:"average_order_value"`
	ConversionRate    float64         `json:"conversion_rate"`
	Currency          string          `json:"currency"`
	OtherCurrencies   []CurrencyTotal `json:"other_currencies"`
}

// CurrencyTotal is the revenue and the number of sales in one currency.
type CurrencyTotal struct {
	Currency     string  `json:"currency"`
	TotalRevenue float64 `json:"total_revenue"`
	TotalSales   int64   `json:"total_sales"`
}

// RevenueIn returns the revenue in the given currency.
func (m *RevenueMetrics) RevenueIn(currency string) float64 {
	if m.Currency == currency {
		return m.TotalRevenue
	}
	for _, other := range m.OtherCurrencies {
		if other.Currency == currency {
			return other.TotalRevenue
		}
	}
	return 0
}

// PerVisitor returns the main currency revenue per visitor.
func (m *RevenueMetrics) PerVisitor(totalVisitors int64) float64 {
	if totalVisitors <= 0 {
		return 0
	}
	return m.TotalRevenue / float64(totalVisitors)
}

const defaultCurrency = "USD"

// A purchase is a "revenue:purchased" custom event, in any letter case,
// with JSON metadata and a price above 0, in cents. Its amount is price times
// quantity (1 when absent).
const purchaseWhereSQL = `LOWER(custom_event_name) = 'revenue:purchased'
	AND (CASE WHEN json_valid(custom_event_meta) THEN CAST(json_extract(custom_event_meta, '$.price') AS REAL) ELSE 0 END) > 0`

const purchaseCentsSQL = `CAST(json_extract(custom_event_meta, '$.price') AS REAL) *
	COALESCE(CAST(json_extract(custom_event_meta, '$.quantity') AS INTEGER), 1)`

// duplicateLookback is how far before the range we look for an earlier event
// of the same order. A buyer who reloads the thank-you page just after
// midnight then does not add the order to the next day again.
const duplicateLookback = 24 * time.Hour

// purchasesSQL starts a query with three tables:
//
//   - purchases: one row per purchase in the range. Events with the same
//     order_id are one purchase, at the earliest event. Events without an
//     order_id are one purchase each. The currency is in upper case, USD
//     when absent.
//   - main_currency: the currency with the most purchases. A tie goes to
//     the first currency in alphabetical order.
//   - main_purchases: the purchases in the main currency.
//
// The tile, the chart, the top revenue events, and the event revenue totals
// all read these tables, so their numbers always agree.
const purchasesSQL = `
	WITH purchase_events AS (
		SELECT
			id,
			timestamp,
			user_signature,
			custom_event_name,
			` + purchaseCentsSQL + ` AS cents,
			UPPER(COALESCE(NULLIF(TRIM(json_extract(custom_event_meta, '$.currency')), ''), '` + defaultCurrency + `')) AS currency,
			ROW_NUMBER() OVER (
				PARTITION BY COALESCE(NULLIF(CAST(json_extract(custom_event_meta, '$.order_id') AS TEXT), ''), 'event:' || id)
				ORDER BY timestamp, id
			) AS copy
		FROM events
		WHERE website_id = ?
		AND timestamp BETWEEN ? AND ?
		AND event_type = ?
		AND ` + purchaseWhereSQL + `
	),
	purchases AS (
		SELECT * FROM purchase_events WHERE copy = 1 AND timestamp >= ?
	),
	main_currency AS (
		SELECT currency FROM purchases GROUP BY currency ORDER BY COUNT(*) DESC, currency ASC LIMIT 1
	),
	main_purchases AS (
		SELECT * FROM purchases WHERE currency = (SELECT currency FROM main_currency)
	)`

// purchasesArgs returns the arguments for purchasesSQL.
func purchasesArgs(params WebsiteScopedQueryParams) []interface{} {
	from := params.TimeFrame.From.UTC()
	return []interface{}{
		params.WebsiteID,
		from.Add(-duplicateLookback),
		params.TimeFrame.To.UTC(),
		events.EventTypeCustomEvent,
		from,
	}
}

// GetRevenueMetrics calculates revenue metrics for events with "revenue:purchased"
// naming convention. totalVisitors is the visitors of the same period, for the
// conversion rate. The caller counts them once and shares the count.
func GetRevenueMetrics(db *gorm.DB, params WebsiteScopedQueryParams, totalVisitors int64) (*RevenueMetrics, error) {
	var totals []struct {
		Currency     string
		TotalRevenue float64
		TotalSales   int64
	}
	query := purchasesSQL + `
		SELECT currency, SUM(cents) / 100.0 AS total_revenue, COUNT(*) AS total_sales
		FROM purchases
		GROUP BY currency
		ORDER BY total_sales DESC, currency ASC`
	if err := db.Raw(query, purchasesArgs(params)...).Scan(&totals).Error; err != nil {
		return nil, fmt.Errorf("error calculating revenue metrics: %w", err)
	}

	var buyers int64
	query = purchasesSQL + `SELECT COUNT(DISTINCT user_signature) FROM purchases`
	if err := db.Raw(query, purchasesArgs(params)...).Scan(&buyers).Error; err != nil {
		return nil, fmt.Errorf("error counting buyers: %w", err)
	}

	metrics := &RevenueMetrics{Currency: defaultCurrency, OtherCurrencies: []CurrencyTotal{}}
	for i, total := range totals {
		if i == 0 {
			metrics.Currency = total.Currency
			metrics.TotalRevenue = total.TotalRevenue
			metrics.TotalSales = total.TotalSales
			continue
		}
		metrics.OtherCurrencies = append(metrics.OtherCurrencies, CurrencyTotal(total))
	}

	// Without sales, the main currency comes from the previous period. A shop
	// that sells in euros then shows 0 euros, and the change is -100%.
	if len(totals) == 0 {
		currency, err := previousMainCurrency(db, params)
		if err != nil {
			return nil, err
		}
		if currency != "" {
			metrics.Currency = currency
		}
	}

	if metrics.TotalSales > 0 {
		metrics.AverageOrderValue = metrics.TotalRevenue / float64(metrics.TotalSales)
	}

	// The share of visitors who bought, in any currency. A visitor who buys
	// twice counts once.
	if totalVisitors > 0 {
		metrics.ConversionRate = (float64(buyers) / float64(totalVisitors)) * 100
	}

	return metrics, nil
}

// previousMainCurrency returns the main currency of the period before
// params, or "" when that period has no purchases.
func previousMainCurrency(db *gorm.DB, params WebsiteScopedQueryParams) (string, error) {
	from, to := PreviousPeriod(params.TimeFrame.From, params.TimeFrame.To, params.TimeFrame.Tz)
	previous := *params.TimeFrame
	previous.From, previous.To = from, to
	params.TimeFrame = &previous

	var currencies []string
	query := purchasesSQL + `SELECT currency FROM main_currency`
	if err := db.Raw(query, purchasesArgs(params)...).Scan(&currencies).Error; err != nil {
		return "", fmt.Errorf("error finding the previous main currency: %w", err)
	}
	if len(currencies) == 0 {
		return "", nil
	}
	return currencies[0], nil
}

// GetTopRevenueEvents returns the most frequent revenue events
func GetTopRevenueEvents(db *gorm.DB, params WebsiteScopedQueryParams) ([]MetricCountResult, error) {
	var results []MetricCountResult

	query := purchasesSQL + `
		SELECT custom_event_name AS name, COUNT(*) AS count
		FROM main_purchases
		GROUP BY custom_event_name
		ORDER BY count DESC
		LIMIT ?`

	args := append(purchasesArgs(params), params.Limit)
	if err := db.Raw(query, args...).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("error fetching top revenue events: %w", err)
	}

	return results, nil
}

// GetEventRevenueTotals returns the total revenue generated per custom event within the timeframe.
// Purchases follow the rules of purchasesSQL. Other events with a price count as they are.
func GetEventRevenueTotals(db *gorm.DB, params WebsiteScopedQueryParams) (map[string]float64, error) {
	var rows []struct {
		Name    string
		Revenue float64
	}

	query := purchasesSQL + `
		SELECT custom_event_name AS name, SUM(cents) / 100.0 AS revenue
		FROM main_purchases
		GROUP BY custom_event_name
		UNION ALL
		SELECT
			custom_event_name AS name,
			SUM(
				CASE
					WHEN json_valid(custom_event_meta) = 1 AND json_extract(custom_event_meta, '$.price') IS NOT NULL
					THEN (CAST(json_extract(custom_event_meta, '$.price') AS REAL) / 100.0) *
						COALESCE(CAST(json_extract(custom_event_meta, '$.quantity') AS INTEGER), 1)
					ELSE 0
				END
			) AS revenue
		FROM events
		WHERE website_id = ?
		AND timestamp BETWEEN ? AND ?
		AND event_type = ?
		AND LOWER(custom_event_name) <> 'revenue:purchased'
		GROUP BY custom_event_name`

	args := append(purchasesArgs(params),
		params.WebsiteID,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		events.EventTypeCustomEvent,
	)
	if err := db.Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("error fetching event revenue totals: %w", err)
	}

	totals := make(map[string]float64, len(rows))
	for _, row := range rows {
		// Only include events with a positive revenue value.
		if row.Revenue > 0 {
			totals[row.Name] = row.Revenue
		}
	}

	return totals, nil
}

// AggregatedRevenueInTimeFrame returns revenue sums aggregated over a time frame from revenue:purchased events
func AggregatedRevenueInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) ([]timeframe.DateStat, error) {
	result, err := aggregatedRevenueInTimeFrameRaw(db, params)
	if err != nil {
		return nil, err
	}

	return params.TimeFrame.BuildTimeSeriesPoints(result), nil
}

// aggregatedRevenueInTimeFrameRaw sums the main currency purchases, in cents, per bucket.
func aggregatedRevenueInTimeFrameRaw(db *gorm.DB, params WebsiteScopedQueryParams) ([]timeframe.DateStat, error) {
	var results []timeframe.DateStat

	groupByExpression, err := params.TimeFrame.GetSQLiteGroupByExpression()
	if err != nil {
		return nil, err
	}

	// Replace 'hour' with 'timestamp' in the group by expression since events table uses timestamp
	groupByExpression = strings.Replace(groupByExpression, "hour", "timestamp", -1)

	query := purchasesSQL + fmt.Sprintf(`
		SELECT
			%s AS date,
			CAST(ROUND(COALESCE(SUM(cents), 0)) AS INTEGER) AS count
		FROM main_purchases
		GROUP BY %s
		ORDER BY date ASC
	`, groupByExpression, groupByExpression)

	if err := db.Raw(query, purchasesArgs(params)...).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("error fetching aggregated revenue from Events: %w", err)
	}

	return results, nil
}
