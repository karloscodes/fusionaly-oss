package analytics

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// LiveWindow is how far back "visitors right now" looks.
const LiveWindow = 5 * time.Minute

// GetLiveVisitors returns the distinct visitors with an event of any type in
// the LiveWindow before now. Events reach the table after the event
// processing job, so the count can lag by one job interval.
func GetLiveVisitors(db *gorm.DB, websiteID int, now time.Time) (int64, error) {
	var count int64
	err := db.Raw(`
        SELECT COUNT(DISTINCT user_signature)
        FROM events
        WHERE website_id = ? AND timestamp > ? AND timestamp <= ?
    `, websiteID, now.UTC().Add(-LiveWindow), now.UTC()).Scan(&count).Error
	if err != nil {
		return 0, fmt.Errorf("error counting live visitors: %w", err)
	}
	return count, nil
}
