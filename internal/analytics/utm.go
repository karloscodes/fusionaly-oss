package analytics

import (
	"gorm.io/gorm"
)

// GetTopUTMMediumsInTimeFrame fetches top UTM mediums
func GetTopUTMMediumsInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) ([]MetricCountResult, error) {
	var results []MetricCountResult

	query := `
		SELECT 
			utm_medium AS name, 
			SUM(visitors_count) AS count
		FROM utm_stats
		WHERE hour BETWEEN ? AND ?
        AND website_id = ?
		GROUP BY utm_medium
		HAVING count > 0
		ORDER BY count DESC
		LIMIT ?
	`

	err := db.Raw(query,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		params.WebsiteID,
		params.Limit,
	).Scan(&results).Error
	if err != nil {
		return []MetricCountResult{}, nil
	}

	return results, nil
}

// GetTopUTMSourcesInTimeFrame fetches top UTM sources
func GetTopUTMSourcesInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) ([]MetricCountResult, error) {
	var results []MetricCountResult

	query := `
		SELECT 
			utm_source AS name, 
			SUM(visitors_count) AS count
		FROM utm_stats
		WHERE hour BETWEEN ? AND ?
        AND website_id = ?
        AND utm_source != ''
		GROUP BY utm_source
		HAVING count > 0
		ORDER BY count DESC
		LIMIT ?
	`

	err := db.Raw(query,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		params.WebsiteID,
		params.Limit,
	).Scan(&results).Error
	if err != nil {
		return []MetricCountResult{}, nil
	}

	return results, nil
}

// GetTopUTMCampaignsInTimeFrame fetches top UTM campaigns
func GetTopUTMCampaignsInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) ([]MetricCountResult, error) {
	var results []MetricCountResult

	query := `
		SELECT 
			utm_campaign AS name, 
			SUM(visitors_count) AS count
		FROM utm_stats
		WHERE hour BETWEEN ? AND ?
        AND website_id = ?
        AND utm_campaign != ''
		GROUP BY utm_campaign
		HAVING count > 0
		ORDER BY count DESC
		LIMIT ?
	`

	err := db.Raw(query,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		params.WebsiteID,
		params.Limit,
	).Scan(&results).Error
	if err != nil {
		return []MetricCountResult{}, nil
	}

	return results, nil
}

// GetTopUTMTermsInTimeFrame fetches top UTM terms
func GetTopUTMTermsInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) ([]MetricCountResult, error) {
	var results []MetricCountResult

	query := `
		SELECT 
			utm_term AS name, 
			SUM(visitors_count) AS count
		FROM utm_stats
		WHERE hour BETWEEN ? AND ?
        AND website_id = ?
        AND utm_term != ''
		GROUP BY utm_term
		HAVING count > 0
		ORDER BY count DESC
		LIMIT ?
	`

	err := db.Raw(query,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		params.WebsiteID,
		params.Limit,
	).Scan(&results).Error
	if err != nil {
		return []MetricCountResult{}, nil
	}

	return results, nil
}

// GetTopUTMContentsInTimeFrame fetches top UTM contents
func GetTopUTMContentsInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) ([]MetricCountResult, error) {
	var results []MetricCountResult

	query := `
		SELECT 
			utm_content AS name, 
			SUM(visitors_count) AS count
		FROM utm_stats
		WHERE hour BETWEEN ? AND ?
        AND website_id = ?
        AND utm_content != ''
		GROUP BY utm_content
		HAVING count > 0
		ORDER BY count DESC
		LIMIT ?
	`

	err := db.Raw(query,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		params.WebsiteID,
		params.Limit,
	).Scan(&results).Error
	if err != nil {
		return []MetricCountResult{}, nil
	}

	return results, nil
}
