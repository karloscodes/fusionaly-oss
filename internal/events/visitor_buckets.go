package events

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/karloscodes/cartridge"
	"gorm.io/gorm"
)

// updateVisitorBuckets keeps the visitor's last page view of the day and
// their first page view of each UTC hour in site_stats. Page views of one
// visitor arrive in time order, so this page view is now their last, and it
// is their first of its hour when the page view before it was in another hour.
func updateVisitorBuckets(tx *gorm.DB, data *EventProcessingData) error {
	bucket := truncateToHalfHour(data.Timestamp.UTC())
	earlier := data.EarlierPageViewAt

	lastInc, hourInc := 1, 1
	if earlier != nil {
		earlierBucket := truncateToHalfHour(earlier.UTC())
		if earlierBucket.Equal(bucket) {
			lastInc = 0
		} else {
			err := tx.Exec(`UPDATE site_stats SET last_visitors = last_visitors - 1
				WHERE website_id = ? AND hour = ? AND last_visitors > 0`,
				data.WebsiteID, earlierBucket).Error
			if err != nil {
				return err
			}
		}
		if earlier.UTC().Truncate(time.Hour).Equal(data.Timestamp.UTC().Truncate(time.Hour)) {
			hourInc = 0
		}
	}

	return tx.Exec(`UPDATE site_stats SET last_visitors = last_visitors + ?, hour_visitors = hour_visitors + ?
		WHERE website_id = ? AND hour = ?`,
		lastInc, hourInc, data.WebsiteID, bucket).Error
}

// keyVisitorBucketsRebuilt marks that site_stats.last_visitors and
// hour_visitors were filled from the events table. Versions before v2.12.0
// stored neither.
const keyVisitorBucketsRebuilt = "visitor_buckets_rebuilt_v1"

// RebuildVisitorBucketsOnce fills the visitor buckets of every website from
// the events table, one time per install (see rebuildOnce).
func RebuildVisitorBucketsOnce(dbManager cartridge.DBManager, logger *slog.Logger) error {
	return rebuildOnce(dbManager, logger, keyVisitorBucketsRebuilt, "visitor buckets", RebuildVisitorBuckets)
}

// RebuildVisitorBuckets sets one website's last_visitors and hour_visitors
// from its events, with the rules of live processing (updateVisitorBuckets).
func RebuildVisitorBuckets(dbManager cartridge.DBManager, websiteID uint) error {
	return rebuildByDay(dbManager, websiteID, tallyVisitorBuckets)
}

// tallyVisitorBuckets counts, for one UTC day, each visitor once in the
// bucket of their last page view, and once per UTC hour in the bucket of
// their first page view of that hour.
func tallyVisitorBuckets(db *gorm.DB, websiteID uint, day time.Time) (dayValues, error) {
	last, hour := map[int64]float64{}, map[int64]float64{}

	rows, err := db.Model(&Event{}).
		Select("user_signature, timestamp").
		Where("website_id = ? AND event_type = ? AND timestamp >= ? AND timestamp < ?",
			websiteID, EventTypePageView, day, day.Add(24*time.Hour)).
		Order("user_signature, timestamp, id").
		Rows()
	if err != nil {
		return nil, fmt.Errorf("failed to read page views: %w", err)
	}
	defer rows.Close()

	var (
		signature        string
		lastView, inHour time.Time
	)
	for rows.Next() {
		var e Event
		if err := db.ScanRows(rows, &e); err != nil {
			return nil, fmt.Errorf("failed to scan page view: %w", err)
		}

		if e.UserSignature != signature {
			if signature != "" {
				last[bucketOf(lastView)]++
			}
			signature, inHour = e.UserSignature, time.Time{}
		}
		if h := e.Timestamp.UTC().Truncate(time.Hour); !h.Equal(inHour) {
			hour[bucketOf(e.Timestamp)]++
			inHour = h
		}
		lastView = e.Timestamp
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read page views: %w", err)
	}
	if signature != "" {
		last[bucketOf(lastView)]++
	}
	return dayValues{"last_visitors": last, "hour_visitors": hour}, nil
}
