package analytics

import (
	"fusionaly/internal/timeframe"
)

import (
	"fmt"

	"gorm.io/gorm"
)

// AggregatedVisitorsInTimeFrame returns visitor counts aggregated over a time frame
func AggregatedVisitorsInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) ([]timeframe.DateStat, error) {
	// Get raw aggregated visitor data
	result, err := aggregatedVisitorsInTimeFrameRaw(db, params)
	if err != nil {
		return nil, err
	}

	// Build consistent time series with all points including zeros
	return params.TimeFrame.BuildTimeSeriesPoints(result), nil
}

// aggregatedVisitorsInTimeFrameRaw fetches raw aggregated visitor data
func aggregatedVisitorsInTimeFrameRaw(db *gorm.DB, params WebsiteScopedQueryParams) ([]timeframe.DateStat, error) {
	var results []timeframe.DateStat

	// Get the appropriate GROUP BY expression based on the time frame bucket size
	groupByExpression, err := params.TimeFrame.GetSQLiteGroupByExpression()
	if err != nil {
		return nil, err
	}

	// Query to get aggregated visitors from SiteStat
	query := fmt.Sprintf(`
        SELECT
            %s AS date,
            COALESCE(SUM(visitors), 0) AS count
        FROM
            site_stats
        WHERE
            hour >= ? AND hour <= ?
            AND website_id = ?
        GROUP BY
            %s
        ORDER BY
            date ASC
    `, groupByExpression, groupByExpression)

	// Execute the query
	err = db.Raw(query,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		params.WebsiteID,
	).Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("error fetching aggregated visitors: %w", err)
	}

	return results, nil
}
