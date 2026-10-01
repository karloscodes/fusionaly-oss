package analytics

import (
	"fmt"

	"gorm.io/gorm"

	"fusionaly/internal/events"
	"fusionaly/internal/timeframe"
)

// Visitors come from the events table at query time, not from site_stats.
// A visitor is a distinct signature with a page view. The signature rotates
// at midnight UTC, so one person is one visitor per UTC day at most.
//
// The rules:
//   - a day counts the distinct visitors of the viewer's local day
//   - a range of several days adds the days ("unique per day, summed")
//   - an hour counts the distinct visitors with a page view in that hour,
//     so a person active at 9:00 and 15:00 counts in both hours
//
// site_stats.visitors counts a visitor once, at the first page view of the
// UTC day. That is wrong for local days and for hours, so these queries do
// not use it.

// dailyVisitorsQuery returns one row per local day: day, visitors.
func dailyVisitorsQuery(tf *timeframe.TimeFrame) string {
	day := tf.LocalDayExpression("timestamp")
	return fmt.Sprintf(`
        SELECT %s AS day, COUNT(DISTINCT user_signature) AS visitors
        FROM events
        WHERE website_id = @website_id
            AND timestamp >= @from AND timestamp <= @to
            AND event_type = @page_view
        GROUP BY day
    `, day)
}

func visitorQueryArgs(params WebsiteScopedQueryParams) map[string]any {
	return map[string]any{
		"website_id": params.WebsiteID,
		"from":       params.TimeFrame.From.UTC(),
		"to":         params.TimeFrame.To.UTC(),
		"page_view":  events.EventTypePageView,
	}
}

// GetTotalVisitorsInTimeFrame returns the visitors of the range: the sum of
// the distinct visitors of each local day.
func GetTotalVisitorsInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) (int64, error) {
	var total int64
	query := "SELECT COALESCE(SUM(visitors), 0) FROM (" + dailyVisitorsQuery(params.TimeFrame) + ")"
	if err := db.Raw(query, visitorQueryArgs(params)).Scan(&total).Error; err != nil {
		return 0, fmt.Errorf("error calculating total visitors: %w", err)
	}
	return total, nil
}

// AggregatedVisitorsInTimeFrame returns visitor counts aggregated over a time frame
func AggregatedVisitorsInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) ([]timeframe.DateStat, error) {
	result, err := aggregatedVisitorsInTimeFrameRaw(db, params)
	if err != nil {
		return nil, err
	}

	// Build consistent time series with all points including zeros
	return params.TimeFrame.BuildTimeSeriesPoints(result), nil
}

// aggregatedVisitorsInTimeFrameRaw counts distinct visitors per UTC hour for
// hour buckets. Larger buckets add the visitors of their local days.
func aggregatedVisitorsInTimeFrameRaw(db *gorm.DB, params WebsiteScopedQueryParams) ([]timeframe.DateStat, error) {
	var results []timeframe.DateStat
	tf := params.TimeFrame

	var query string
	if tf.BucketSize == timeframe.TimeFrameBucketSizeHour {
		hour, err := tf.GroupByExpressionFor("timestamp")
		if err != nil {
			return nil, err
		}
		query = fmt.Sprintf(`
            SELECT %s AS date, COUNT(DISTINCT user_signature) AS count
            FROM events
            WHERE website_id = @website_id
                AND timestamp >= @from AND timestamp <= @to
                AND event_type = @page_view
            GROUP BY date
            ORDER BY date ASC
        `, hour)
	} else {
		// The day column is already local, so the bucket needs no zone shift.
		localDays := *tf
		localDays.Tz = nil
		bucket, err := localDays.GroupByExpressionFor("day")
		if err != nil {
			return nil, err
		}
		query = fmt.Sprintf(`
            SELECT %s AS date, SUM(visitors) AS count
            FROM (%s)
            GROUP BY date
            ORDER BY date ASC
        `, bucket, dailyVisitorsQuery(tf))
	}

	if err := db.Raw(query, visitorQueryArgs(params)).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("error fetching aggregated visitors: %w", err)
	}
	return results, nil
}
