package analytics

import (
	"gorm.io/gorm"
)

// GetTopQueryParamValuesInTimeFrame fetches top values for a specific query parameter
func GetTopQueryParamValuesInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams, paramName string) ([]MetricCountResult, error) {
	var results []MetricCountResult

	query := `
		SELECT 
			param_value AS name, 
			SUM(visitors_count) AS count
		FROM query_param_stats
		WHERE hour BETWEEN ? AND ?
        AND website_id = ?
        AND param_name = ?
        AND param_value != ''
		GROUP BY param_value
		HAVING count > 0
		ORDER BY count DESC
		LIMIT ?
	`

	err := db.Raw(query,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		params.WebsiteID,
		paramName,
		params.Limit,
	).Scan(&results).Error
	if err != nil {
		return []MetricCountResult{}, nil
	}

	return results, nil
}
