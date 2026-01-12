package analytics

import (
	"fmt"
	"strings"

	"fusionaly/internal/timeframe"

	"log/slog"
	"gorm.io/gorm"
)

// DeviceConversionRate holds the rate for a specific device type
type DeviceConversionRate struct {
	DeviceType      string  `json:"device_type"`
	ConversionRate  float64 `json:"conversion_rate"`
	VisitorCount    int64   `json:"visitor_count"`
	ConversionCount int64   `json:"conversion_count"`
}

// GetConversionRatesByDevice returns conversion rates broken down by device type
func GetConversionRatesByDevice(db *gorm.DB, params WebsiteScopedQueryParams, conversionGoals []string) ([]DeviceConversionRate, error) {
	logger := slog.Default()

	// Return empty results early to avoid the SQL error with device_type in event_stats
	logger.Warn("GetConversionRatesByDevice is temporarily disabled due to schema limitations")
	return []DeviceConversionRate{}, nil

	/* Original implementation commented out
		if len(conversionGoals) == 0 {
			logger.Info("GetConversionRatesByDevice called with no conversion goals", slog.Int("websiteID", params.WebsiteID))
			return []DeviceConversionRate{}, nil // No goals to analyze
		}

		// 1. Get total visitors per device
		type DeviceVisitorCount struct {
			DeviceType string
			Count      int64
		}
		var visitorCounts []DeviceVisitorCount
		// Use COALESCE for device_type similar to GetTopDeviceTypesInTimeFrame
		visitorQuery := `
	        SELECT
	            COALESCE(device_type, 'unknown') as device_type,
	            SUM(visitors_count) as count
	        FROM device_stats
	        WHERE website_id = ?
	        AND hour BETWEEN ? AND ?
	        GROUP BY device_type
	    `
		if err := db.Raw(visitorQuery, params.WebsiteID, params.TimeFrame.From, params.TimeFrame.To).Scan(&visitorCounts).Error; err != nil {
			logger.Error("Error fetching visitor counts by device for conversion rate", slog.Any("error", err), slog.Int("websiteID", params.WebsiteID))
			return nil, fmt.Errorf("failed to get visitors by device: %w", err)
		}

		visitorMap := make(map[string]int64)
		for _, vc := range visitorCounts {
			if vc.DeviceType == "" {
				vc.DeviceType = "unknown"
			}
			visitorMap[vc.DeviceType] = vc.Count
		}

		// 2. Get conversion counts per device
		type DeviceConversionCount struct {
			DeviceType string
			Count      int64
		}
		var conversionCounts []DeviceConversionCount
		// Use COALESCE for device_type
		conversionQuery := `
	        SELECT
	            COALESCE(device_type, 'unknown') as device_type,
	            SUM(visitors_count) as count
	        FROM event_stats
	        WHERE website_id = ?
	        AND event_name IN (?)
	        AND hour BETWEEN ? AND ?
	        GROUP BY device_type
	    `
		if err := db.Raw(conversionQuery, params.WebsiteID, conversionGoals, params.TimeFrame.From, params.TimeFrame.To).Scan(&conversionCounts).Error; err != nil {
			logger.Error("Error fetching conversion counts by device", slog.Any("error", err), slog.Int("websiteID", params.WebsiteID))
			return nil, fmt.Errorf("failed to get conversions by device: %w", err)
		}

		conversionMap := make(map[string]int64)
		for _, cc := range conversionCounts {
			if cc.DeviceType == "" {
				cc.DeviceType = "unknown"
			}
			conversionMap[cc.DeviceType] = cc.Count
		}

		// 3. Calculate rates
		results := []DeviceConversionRate{}
		// Iterate through devices found in visitor counts (or a predefined list like ["desktop", "mobile", "tablet", "unknown"])
		for deviceType, visitors := range visitorMap {
			if visitors == 0 {
				continue
			} // Skip if no visitors for this device type
			conversions := conversionMap[deviceType] // Defaults to 0 if not found
			rate := 0.0
			if visitors > 0 {
				rate = (float64(conversions) / float64(visitors)) * 100
			}
			results = append(results, DeviceConversionRate{
				DeviceType:      deviceType,
				ConversionRate:  rate,
				VisitorCount:    visitors,
				ConversionCount: conversions,
			})
		}

		// Sort for consistency (e.g., by visitor count desc)
		sort.Slice(results, func(i, j int) bool {
			return results[i].VisitorCount > results[j].VisitorCount
		})

		return results, nil
	*/
}

// AggregatedGoalConversionsInTimeFrame returns goal conversion counts aggregated over a time frame using EventStat
func AggregatedGoalConversionsInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams, conversionGoals []string) ([]timeframe.DateStat, error) {
	if len(conversionGoals) == 0 {
		// Return empty time series if no goals are configured
		return params.TimeFrame.BuildTimeSeriesPoints([]timeframe.DateStat{}), nil
	}

	result, err := aggregatedGoalConversionsInTimeFrameRaw(db, params, conversionGoals)
	if err != nil {
		return nil, err
	}

	return params.TimeFrame.BuildTimeSeriesPoints(result), nil
}

// aggregatedGoalConversionsInTimeFrameRaw fetches raw aggregated goal conversion data from EventStat
func aggregatedGoalConversionsInTimeFrameRaw(db *gorm.DB, params WebsiteScopedQueryParams, conversionGoals []string) ([]timeframe.DateStat, error) {
	var results []timeframe.DateStat

	groupByExpression, err := params.TimeFrame.GetSQLiteGroupByExpression()
	if err != nil {
		return nil, err
	}

	// Use a more reliable query format that works better with SQLite's date handling
	query := fmt.Sprintf(`
        SELECT
            %s AS date,
            COALESCE(SUM(visitors_count), 0) AS count
        FROM
            event_stats
        WHERE
            hour >= ? AND hour <= ?
            AND website_id = ?
            AND event_name IN (%s)
        GROUP BY
            %s
        ORDER BY
            date ASC
    `, groupByExpression, generatePlaceholders(len(conversionGoals)), groupByExpression)

	// Build query arguments
	args := []interface{}{
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		params.WebsiteID,
	}

	// Add goal names to query arguments
	for _, goal := range conversionGoals {
		args = append(args, goal)
	}

	// Execute query
	err = db.Raw(query, args...).Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("error fetching aggregated goal conversions from EventStat: %w", err)
	}

	return results, nil
}

// generatePlaceholders generates a string of SQL placeholders for IN clause
func generatePlaceholders(count int) string {
	if count == 0 {
		return ""
	}
	placeholders := make([]string, count)
	for i := 0; i < count; i++ {
		placeholders[i] = "?"
	}
	return strings.Join(placeholders, ", ")
}
