package analytics

import (
	"fmt"
	"strings"

	"fusionaly/internal/events"
	"fusionaly/internal/timeframe"

	"gorm.io/gorm"
)

// RevenueMetrics holds revenue-related metrics
type RevenueMetrics struct {
	TotalRevenue      float64 `json:"total_revenue"`
	TotalSales        int64   `json:"total_sales"`
	AverageOrderValue float64 `json:"average_order_value"`
	ConversionRate    float64 `json:"conversion_rate"`
	Currency          string  `json:"currency"`
}

// GetRevenueMetrics calculates revenue metrics for events with "revenue:purchased" naming convention
func GetRevenueMetrics(db *gorm.DB, params WebsiteScopedQueryParams) (*RevenueMetrics, error) {
	// Get total sales count and revenue from events with revenue naming convention
	var result struct {
		TotalRevenue float64
		TotalSales   int64
		Currency     string
	}

	query := `
		SELECT 
			COALESCE(SUM(
				CASE 
					WHEN json_valid(custom_event_meta) = 1 AND json_extract(custom_event_meta, '$.price') IS NOT NULL 
					THEN (CAST(json_extract(custom_event_meta, '$.price') AS REAL) / 100.0) * 
						 COALESCE(CAST(json_extract(custom_event_meta, '$.quantity') AS INTEGER), 1)
					ELSE 0 
				END
			), 0) as total_revenue,
			COUNT(*) as total_sales,
			CASE 
				WHEN json_valid(custom_event_meta) = 1 
				THEN COALESCE(json_extract(custom_event_meta, '$.currency'), 'USD')
				ELSE 'USD'
			END as currency
		FROM events 
		WHERE website_id = ? 
		AND timestamp BETWEEN ? AND ?
		AND event_type = ?
		AND LOWER(custom_event_name) LIKE 'revenue:purchased'
		AND json_valid(custom_event_meta) = 1
		AND json_extract(custom_event_meta, '$.price') IS NOT NULL
		AND CAST(json_extract(custom_event_meta, '$.price') AS REAL) > 0
	`

	err := db.Raw(query,
		params.WebsiteID,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		events.EventTypeCustomEvent,
	).Scan(&result).Error

	if err != nil {
		return nil, fmt.Errorf("error calculating revenue metrics: %w", err)
	}

	// Calculate average order value
	averageOrderValue := 0.0
	if result.TotalSales > 0 {
		averageOrderValue = result.TotalRevenue / float64(result.TotalSales)
	}

	// Calculate conversion rate (sales / total visitors)
	totalVisitors, err := GetTotalVisitorsInTimeFrame(db, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get total visitors for conversion rate: %w", err)
	}

	conversionRate := 0.0
	if totalVisitors > 0 {
		conversionRate = (float64(result.TotalSales) / float64(totalVisitors)) * 100
	}

	// Set default currency if none found
	currency := result.Currency
	if currency == "" {
		currency = "USD"
	}

	return &RevenueMetrics{
		TotalRevenue:      result.TotalRevenue,
		TotalSales:        result.TotalSales,
		AverageOrderValue: averageOrderValue,
		ConversionRate:    conversionRate,
		Currency:          currency,
	}, nil
}

// GetTopRevenueEvents returns the most frequent revenue events
func GetTopRevenueEvents(db *gorm.DB, params WebsiteScopedQueryParams) ([]MetricCountResult, error) {
	var results []MetricCountResult

	query := `
		SELECT 
			custom_event_name as name,
			COUNT(*) as count
		FROM events 
		WHERE website_id = ? 
		AND timestamp BETWEEN ? AND ?
		AND event_type = ?
		AND LOWER(custom_event_name) LIKE 'revenue:purchased'
		GROUP BY custom_event_name
		ORDER BY count DESC
		LIMIT ?
	`

	err := db.Raw(query,
		params.WebsiteID,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		events.EventTypeCustomEvent,
		params.Limit,
	).Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("error fetching top revenue events: %w", err)
	}

	return results, nil
}

// GetEventRevenueTotals returns the total revenue generated per custom event within the timeframe.
func GetEventRevenueTotals(db *gorm.DB, params WebsiteScopedQueryParams) (map[string]float64, error) {
	var rows []struct {
		Name    string
		Revenue float64
	}

	query := `
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
		GROUP BY custom_event_name
	`

	if err := db.Raw(query,
		params.WebsiteID,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		events.EventTypeCustomEvent,
	).Scan(&rows).Error; err != nil {
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

// GetRevenuePerVisitor calculates revenue per visitor for the given time frame
func GetRevenuePerVisitor(db *gorm.DB, params WebsiteScopedQueryParams) (float64, error) {
	// Get revenue metrics
	revenueMetrics, err := GetRevenueMetrics(db, params)
	if err != nil {
		return 0, fmt.Errorf("failed to get revenue metrics: %w", err)
	}

	// Get total visitors
	totalVisitors, err := GetTotalVisitorsInTimeFrame(db, params)
	if err != nil {
		return 0, fmt.Errorf("failed to get total visitors: %w", err)
	}

	// Calculate revenue per visitor
	if totalVisitors > 0 {
		return revenueMetrics.TotalRevenue / float64(totalVisitors), nil
	}

	return 0, nil
}

// AggregatedRevenueInTimeFrame returns revenue sums aggregated over a time frame from revenue:purchased events
func AggregatedRevenueInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) ([]timeframe.DateStat, error) {
	result, err := aggregatedRevenueInTimeFrameRaw(db, params)
	if err != nil {
		return nil, err
	}

	return params.TimeFrame.BuildTimeSeriesPoints(result), nil
}

// aggregatedRevenueInTimeFrameRaw fetches raw aggregated revenue data from Events table
func aggregatedRevenueInTimeFrameRaw(db *gorm.DB, params WebsiteScopedQueryParams) ([]timeframe.DateStat, error) {
	var results []timeframe.DateStat

	groupByExpression, err := params.TimeFrame.GetSQLiteGroupByExpression()
	if err != nil {
		return nil, err
	}

	// Replace 'hour' with 'timestamp' in the group by expression since events table uses timestamp
	groupByExpression = strings.Replace(groupByExpression, "hour", "timestamp", -1)

	// Query to sum revenue from revenue:purchased events by extracting price from JSON metadata
	query := fmt.Sprintf(`
        SELECT
            %s AS date,
            COALESCE(SUM(
                CASE 
                    WHEN json_valid(custom_event_meta) = 1 AND json_extract(custom_event_meta, '$.price') IS NOT NULL 
                    THEN CAST(json_extract(custom_event_meta, '$.price') AS INTEGER)
                    ELSE 0
                END
            ), 0) AS count
        FROM
            events
        WHERE
            timestamp >= ? AND timestamp <= ?
            AND website_id = ?
            AND event_type = ?
            AND custom_event_name = 'revenue:purchased'
        GROUP BY
            %s
        ORDER BY
            date ASC
    `, groupByExpression, groupByExpression)

	// Execute query
	err = db.Raw(query, params.TimeFrame.From.UTC(), params.TimeFrame.To.UTC(), params.WebsiteID, events.EventTypeCustomEvent).Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("error fetching aggregated revenue from Events: %w", err)
	}

	return results, nil
}
