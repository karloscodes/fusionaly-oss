package analytics

import (
	"fmt"

	"gorm.io/gorm"
)

// GetTopURLsInTimeFrame fetches top URLs from PageStat
func GetTopURLsInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) ([]MetricCountResult, error) {
	var rawResults []struct {
		URL   string
		Count int64
	}

	query := `
    SELECT 
        hostname || pathname as url, 
        SUM(visitors_count) as count
    FROM page_stats
    WHERE hour BETWEEN ? AND ?
    AND website_id = ?
    GROUP BY hostname, pathname
    HAVING count > 0
    ORDER BY count DESC
    LIMIT ?
    `

	err := db.Raw(query,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		params.WebsiteID,
		params.Limit,
	).Scan(&rawResults).Error
	if err != nil {
		return nil, fmt.Errorf("error fetching top URLs from PageStat: %w", err)
	}

	results := make([]MetricCountResult, len(rawResults))
	for i, r := range rawResults {
		results[i] = MetricCountResult{Name: r.URL, Count: r.Count}
	}

	return results, nil
}

// GetTopBrowsersInTimeFrame fetches top browsers from BrowserStat
func GetTopBrowsersInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) ([]MetricCountResult, error) {
	var rawResults []struct {
		Browser string
		Count   int64
	}

	query := `
    SELECT 
        browser as browser, 
        SUM(visitors_count) as count
    FROM browser_stats
    WHERE hour BETWEEN ? AND ?
    AND website_id = ?
    GROUP BY browser
    HAVING count > 0
    ORDER BY count DESC
    LIMIT ?
    `

	err := db.Raw(query,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		params.WebsiteID,
		params.Limit,
	).Scan(&rawResults).Error
	if err != nil {
		return nil, fmt.Errorf("error fetching top browsers from BrowserStat: %w", err)
	}

	results := make([]MetricCountResult, len(rawResults))
	for i, r := range rawResults {
		results[i] = MetricCountResult{Name: r.Browser, Count: r.Count}
	}

	return results, nil
}

// GetTopOsInTimeFrame fetches top operating systems from OSStat
func GetTopOsInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) ([]MetricCountResult, error) {
	var rawResults []struct {
		OS    string
		Count int64
	}

	query := `
    SELECT 
        CASE 
            WHEN LOWER(operating_system) LIKE '%mac%' OR LOWER(operating_system) LIKE '%darwin%' THEN 'MacOS'
            WHEN LOWER(operating_system) LIKE '%linux%' OR LOWER(operating_system) LIKE '%gnu/linux%' THEN 'Linux'
            WHEN LOWER(operating_system) LIKE '%ios%' OR LOWER(operating_system) LIKE '%iphone os%' THEN 'iOS'
            WHEN LOWER(operating_system) LIKE '%android%' THEN 'Android'
            WHEN LOWER(operating_system) LIKE '%windows%' THEN 'Windows'
            ELSE operating_system
        END as os, 
        SUM(visitors_count) as count
    FROM os_stats
    WHERE hour BETWEEN ? AND ?
    AND website_id = ?
    GROUP BY 
        CASE 
            WHEN LOWER(operating_system) LIKE '%mac%' OR LOWER(operating_system) LIKE '%darwin%' THEN 'MacOS'
            WHEN LOWER(operating_system) LIKE '%linux%' OR LOWER(operating_system) LIKE '%gnu/linux%' THEN 'Linux'
            WHEN LOWER(operating_system) LIKE '%ios%' OR LOWER(operating_system) LIKE '%iphone os%' THEN 'iOS'
            WHEN LOWER(operating_system) LIKE '%android%' THEN 'Android'
            WHEN LOWER(operating_system) LIKE '%windows%' THEN 'Windows'
            ELSE operating_system
        END
    HAVING count > 0
    ORDER BY count DESC
    LIMIT ?
    `

	err := db.Raw(query,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		params.WebsiteID,
		params.Limit,
	).Scan(&rawResults).Error
	if err != nil {
		return nil, fmt.Errorf("error fetching top operating systems from OSStat: %w", err)
	}

	results := make([]MetricCountResult, len(rawResults))
	for i, r := range rawResults {
		results[i] = MetricCountResult{Name: r.OS, Count: r.Count}
	}

	return results, nil
}

// GetTopCountriesInTimeFrame fetches top countries from CountryStat
func GetTopCountriesInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) ([]MetricCountResult, error) {
	var rawResults []struct {
		Country string
		Count   int64
	}

	query := `
    SELECT 
        country as country, 
        SUM(visitors_count) as count
    FROM country_stats
    WHERE hour BETWEEN ? AND ?
    AND website_id = ?
    GROUP BY country
    HAVING count > 0
    ORDER BY count DESC
    LIMIT ?
    `

	err := db.Raw(query,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		params.WebsiteID,
		params.Limit,
	).Scan(&rawResults).Error
	if err != nil {
		return nil, fmt.Errorf("error fetching top countries from CountryStat: %w", err)
	}

	results := make([]MetricCountResult, len(rawResults))
	for i, r := range rawResults {
		results[i] = MetricCountResult{Name: r.Country, Count: r.Count}
	}

	return results, nil
}

// GetTopDeviceTypesInTimeFrame fetches top device types from DeviceStat
func GetTopDeviceTypesInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) ([]MetricCountResult, error) {
	var rawResults []struct {
		Device string
		Count  int64
	}

	query := `
    SELECT 
        device_type as device, 
        SUM(visitors_count) as count
    FROM device_stats
    WHERE hour BETWEEN ? AND ?
    AND website_id = ?
    GROUP BY device_type
    HAVING count > 0
    ORDER BY count DESC
    LIMIT ?
    `

	err := db.Raw(query,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		params.WebsiteID,
		params.Limit,
	).Scan(&rawResults).Error
	if err != nil {
		return nil, fmt.Errorf("error fetching top device types from DeviceStat: %w", err)
	}

	results := make([]MetricCountResult, len(rawResults))
	for i, r := range rawResults {
		results[i] = MetricCountResult{Name: r.Device, Count: r.Count}
	}

	return results, nil
}

// GetTopCustomEventsInTimeFrame fetches top custom events from EventStat
func GetTopCustomEventsInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) ([]MetricCountResult, error) {
	var rawResults []struct {
		CustomEvent string
		Count       int64
	}

	query := `
    SELECT 
        event_key as custom_event, 
        SUM(visitors_count) as count
    FROM event_stats
    WHERE hour BETWEEN ? AND ?
    AND website_id = ?
    GROUP BY event_key
    HAVING SUM(visitors_count) > 0
    ORDER BY count DESC
    LIMIT ?
    `

	err := db.Raw(query,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		params.WebsiteID,
		params.Limit,
	).Scan(&rawResults).Error
	if err != nil {
		return nil, fmt.Errorf("error fetching top custom events from EventStat: %w", err)
	}

	results := make([]MetricCountResult, len(rawResults))
	for i, r := range rawResults {
		results[i] = MetricCountResult{Name: r.CustomEvent, Count: r.Count}
	}

	return results, nil
}

// GetTopEntryPagesInTimeFrame fetches top entry pages from PageStat
func GetTopEntryPagesInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) ([]MetricCountResult, error) {
	var results []MetricCountResult

	query := `
    SELECT 
        hostname || pathname as name, 
        SUM(entrances) as count
    FROM page_stats
    WHERE hour BETWEEN ? AND ?
    AND website_id = ?
    GROUP BY hostname, pathname
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
		return nil, fmt.Errorf("error fetching top entry pages from PageStat: %w", err)
	}

	return results, nil
}

// GetTopExitPagesInTimeFrame fetches top exit pages from PageStat
func GetTopExitPagesInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) ([]MetricCountResult, error) {
	var results []MetricCountResult

	query := `
    SELECT 
        hostname || pathname as name, 
        SUM(exits) as count
    FROM page_stats
    WHERE hour BETWEEN ? AND ?
    AND website_id = ?
    GROUP BY hostname, pathname
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
		return nil, fmt.Errorf("error fetching top exit pages from PageStat: %w", err)
	}

	return results, nil
}
