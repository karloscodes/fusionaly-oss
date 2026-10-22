package events

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/karloscodes/cartridge"
	"gorm.io/gorm"

	"fusionaly/internal/config"
)

// Visit time is the length of a visit: from its first to its last event, of
// any type. site_stats.visit_seconds sums it in the bucket of each visit's
// first page view, where the visit counts as a session. The dashboard's
// average visit duration is visit_seconds over sessions.

// addVisitTime adds the time an event adds to its visit to site_stats, in
// the bucket of the visit's first page view, where the visit counts as a
// session. No event is earlier than the visitor's latest event, so a visit
// lasts from its start to its latest event: the first page view adds the
// time so far, and each later event adds the time since the event before
// it. eventID is the stored event, or 0 for a page hide.
func addVisitTime(tx *gorm.DB, websiteID uint, userSignature string, v visit, eventID uint) error {
	if v.sessionStart == nil {
		return nil // the visit began before visits were recorded
	}

	var first Event
	err := tx.Select("id", "timestamp").
		Where("website_id = ? AND user_signature = ? AND session_start = ? AND event_type = ?",
			websiteID, userSignature, v.sessionStart.UTC(), EventTypePageView).
		Order("timestamp, id").
		Limit(1).
		Find(&first).Error
	if err != nil {
		return fmt.Errorf("failed to find the visit's first page view: %w", err)
	}
	if first.ID == 0 {
		return nil // no page view yet: the first one adds the time so far
	}

	added := v.at.Sub(v.previous)
	if first.ID == eventID {
		added = v.at.Sub(*v.sessionStart)
	}
	if added <= 0 {
		return nil
	}

	now := time.Now().UTC()
	err = tx.Exec(`
		INSERT INTO site_stats (website_id, hour, page_views, visitors, sessions, bounce_count, visit_seconds, created_at, updated_at)
		VALUES (?, ?, 0, 0, 0, 0, ?, ?, ?)
		ON CONFLICT (website_id, hour) DO UPDATE SET
			visit_seconds = site_stats.visit_seconds + excluded.visit_seconds,
			updated_at = excluded.updated_at
	`, websiteID, truncateToHalfHour(first.Timestamp.UTC()), added.Seconds(), now, now).Error
	if err != nil {
		return fmt.Errorf("failed to add visit time: %w", err)
	}
	return nil
}

// keyVisitTimeRebuilt marks that site_stats.visit_seconds was filled from the
// events table. Versions before v2.12.0 stored no visit time.
const keyVisitTimeRebuilt = "visit_time_rebuilt_v1"

// RebuildVisitTimeOnce fills the visit time of every website from the events
// table, one time per install (see rebuildOnce).
func RebuildVisitTimeOnce(dbManager cartridge.DBManager, logger *slog.Logger) error {
	return rebuildOnce(dbManager, logger, keyVisitTimeRebuilt, "visit time", RebuildVisitTime)
}

// RebuildVisitTime sets one website's visit time from its events.
func RebuildVisitTime(dbManager cartridge.DBManager, websiteID uint) error {
	return rebuildByDay(dbManager, websiteID, func(db *gorm.DB, websiteID uint, day time.Time) (dayValues, error) {
		seconds, err := tallyVisitTime(db, websiteID, day)
		return dayValues{"visit_seconds": seconds}, err
	})
}

// tallyVisitTime sums the visit time of one UTC day, by the bucket of each
// visit's first page view, with the visit rules of live processing.
func tallyVisitTime(db *gorm.DB, websiteID uint, day time.Time) (map[int64]float64, error) {
	timeout := time.Duration(config.GetConfig().SessionTimeoutSeconds) * time.Second
	tally := map[int64]float64{}

	rows, err := db.Model(&Event{}).
		Select("user_signature, event_type, timestamp").
		Where("website_id = ? AND timestamp >= ? AND timestamp < ?", websiteID, day, day.Add(24*time.Hour)).
		Order("user_signature, timestamp, id").
		Rows()
	if err != nil {
		return nil, fmt.Errorf("failed to read events: %w", err)
	}
	defer rows.Close()

	var (
		signature   string
		start, last time.Time
		entry       int64 // the bucket of the visit's first page view
		hasPageView bool
	)
	endVisit := func() {
		if hasPageView {
			tally[entry] += last.Sub(start).Seconds()
		}
	}

	for rows.Next() {
		var e Event
		if err := db.ScanRows(rows, &e); err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		if e.UserSignature != signature || e.Timestamp.Sub(last) > timeout {
			if !last.IsZero() {
				endVisit()
			}
			signature, start, hasPageView = e.UserSignature, e.Timestamp, false
		}
		last = e.Timestamp

		if e.EventType == EventTypePageView && !hasPageView {
			hasPageView, entry = true, bucketOf(e.Timestamp)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read events: %w", err)
	}
	if !last.IsZero() {
		endVisit()
	}
	return tally, nil
}
