package analytics

import (
	"fmt"
	"time"

	"fusionaly/internal/events"
	"fusionaly/internal/timeframe"

	"gorm.io/gorm"
)

// AggregatedPageViewsInTimeFrame returns page view counts aggregated over a time frame using SiteStat
func AggregatedPageViewsInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) ([]timeframe.DateStat, error) {
	result, err := aggregatedPageViewsInTimeFrameRaw(db, params)
	if err != nil {
		return nil, err
	}

	return params.TimeFrame.BuildTimeSeriesPoints(result), nil
}

// aggregatedPageViewsInTimeFrameRaw fetches raw aggregated page view data from SiteStat
func aggregatedPageViewsInTimeFrameRaw(db *gorm.DB, params WebsiteScopedQueryParams) ([]timeframe.DateStat, error) {
	var results []timeframe.DateStat

	groupByExpression, err := params.TimeFrame.GetSQLiteGroupByExpression()
	if err != nil {
		return nil, err
	}

	// Use a more reliable query format that works better with SQLite's date handling
	query := fmt.Sprintf(`
        SELECT
            %s AS date,
            COALESCE(SUM(page_views), 0) AS count
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
	// Execute query
	err = db.Raw(query,
		params.TimeFrame.From.UTC(),
		params.TimeFrame.To.UTC(),
		params.WebsiteID,
	).Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("error fetching aggregated page views from SiteStat: %w", err)
	}

	return results, nil
}

// GetFirstPageView returns the first page view event for a website
func GetFirstPageView(db *gorm.DB, websiteID int) (*events.Event, error) {
	var event events.Event
	err := db.Where("website_id = ? AND event_type = ?", websiteID, events.EventTypePageView).
		Order("timestamp ASC").
		First(&event).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error fetching first page view: %w", err)
	}

	return &event, nil
}

// PageStat represents aggregated page view statistics
type PageStat struct {
	ID             uint      `gorm:"primaryKey;autoIncrement"`
	WebsiteID      uint      `gorm:"uniqueIndex:idx_page_unique;not null"`
	Hostname       string    `gorm:"uniqueIndex:idx_page_unique;not null"`
	Pathname       string    `gorm:"uniqueIndex:idx_page_unique;not null"`
	PageViewsCount int       `gorm:"not null;default:0"`
	VisitorsCount  int       `gorm:"not null;default:0"`
	Entrances      int       `gorm:"not null;default:0"`
	Exits          int       `gorm:"not null;default:0"`
	Hour           time.Time `gorm:"uniqueIndex:idx_page_unique;type:datetime;not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
