package analytics

import (
	"fmt"
	"time"

	"fusionaly/internal/timeframe"

	"gorm.io/gorm"
)

// AggregatedSessionsInTimeFrame returns session counts aggregated over a time frame
func AggregatedSessionsInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) ([]timeframe.DateStat, error) {
	// Get raw aggregated session data
	result, err := aggregatedSessionsInTimeFrameRaw(db, params)
	if err != nil {
		return nil, err
	}

	// Build consistent time series with all points including zeros
	return params.TimeFrame.BuildTimeSeriesPoints(result), nil
}

// aggregatedSessionsInTimeFrameRaw fetches raw aggregated session data
func aggregatedSessionsInTimeFrameRaw(db *gorm.DB, params WebsiteScopedQueryParams) ([]timeframe.DateStat, error) {
	var results []timeframe.DateStat

	// Get the appropriate GROUP BY expression based on the time frame bucket size
	groupByExpression, err := params.TimeFrame.GetSQLiteGroupByExpression()
	if err != nil {
		return nil, err
	}

	// Query to get aggregated sessions from SiteStat
	query := fmt.Sprintf(`
        SELECT
            %s AS date,
            COALESCE(SUM(sessions), 0) AS count
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
		return nil, fmt.Errorf("error fetching aggregated sessions: %w", err)
	}

	return results, nil
}

// SiteStat represents aggregated site-wide statistics including sessions
type SiteStat struct {
	ID          uint `gorm:"primaryKey;autoIncrement"`
	WebsiteID   uint `gorm:"uniqueIndex:idx_site_hour;not null"`
	PageViews   int  `gorm:"not null;default:0"`
	Visitors    int  `gorm:"not null;default:0"`
	Sessions    int  `gorm:"not null;default:0"`
	BounceCount int  `gorm:"not null;default:0"`
	// VisitSeconds sums the length of the visits that start in this bucket:
	// from a visit's first to its last event, of any type.
	VisitSeconds float64 `gorm:"not null;default:0"`
	// Visitors counts the visitors whose first page view of their UTC day is
	// in this bucket. LastVisitors counts those whose last page view of the
	// day is in it, and HourVisitors those whose first page view of the UTC
	// hour is in it. Together they give the visitors of any local day and
	// hour (see visitors.go).
	LastVisitors int       `gorm:"not null;default:0"`
	HourVisitors int       `gorm:"not null;default:0"`
	Hour         time.Time `gorm:"uniqueIndex:idx_site_hour;type:datetime;not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
