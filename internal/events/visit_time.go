package events

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/karloscodes/cartridge"
	"gorm.io/gorm"

	"fusionaly/internal/config"
	"fusionaly/internal/settings"
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
// table, one time per install. The event processor calls it before it
// processes new events, so no visit time counts twice. A failed run is
// retried on the next one; each day is set, not added to, so a rerun is safe.
func RebuildVisitTimeOnce(dbManager cartridge.DBManager, logger *slog.Logger) error {
	db := dbManager.GetConnection()
	if done, _ := settings.GetSetting(db, keyVisitTimeRebuilt); done != "" {
		return nil
	}

	var websiteIDs []uint
	if err := db.Model(&Event{}).Distinct().Pluck("website_id", &websiteIDs).Error; err != nil {
		return fmt.Errorf("failed to list websites with events: %w", err)
	}

	started := time.Now()
	for _, id := range websiteIDs {
		if err := RebuildVisitTime(dbManager, id); err != nil {
			return fmt.Errorf("website %d: %w", id, err)
		}
	}

	logger.Info("Rebuilt visit time",
		slog.Int("websites", len(websiteIDs)),
		slog.Duration("took", time.Since(started)))
	return settings.CreateOrUpdateSetting(db, keyVisitTimeRebuilt, time.Now().UTC().Format(time.RFC3339))
}

// RebuildVisitTime sets one website's visit time from its events. A
// signature lasts one UTC day, so no visit spans two days, and each day is
// one short write: ingestion goes on while it runs.
func RebuildVisitTime(dbManager cartridge.DBManager, websiteID uint) error {
	db := dbManager.GetConnection()

	var first, last Event
	if err := db.Select("timestamp").Where("website_id = ?", websiteID).Order("timestamp").Limit(1).Find(&first).Error; err != nil {
		return fmt.Errorf("failed to find the first event: %w", err)
	}
	if err := db.Select("timestamp").Where("website_id = ?", websiteID).Order("timestamp DESC").Limit(1).Find(&last).Error; err != nil {
		return fmt.Errorf("failed to find the last event: %w", err)
	}
	if first.Timestamp.IsZero() {
		return nil
	}

	// Match the stored rows by their bucket's instant, not by the stored
	// text, so rows written with another time zone offset still match.
	var rows []struct {
		ID   uint
		Hour time.Time
	}
	if err := db.Table("site_stats").Select("id, hour").Where("website_id = ?", websiteID).Scan(&rows).Error; err != nil {
		return fmt.Errorf("failed to read site stats: %w", err)
	}
	rowOfBucket := make(map[int64]uint, len(rows))
	for _, r := range rows {
		rowOfBucket[bucketOf(r.Hour)] = r.ID
	}

	for day := first.Timestamp.UTC().Truncate(24 * time.Hour); !day.After(last.Timestamp.UTC()); day = day.Add(24 * time.Hour) {
		seconds, err := tallyVisitTime(db, websiteID, day)
		if err != nil {
			return fmt.Errorf("day %s: %w", day.Format(time.DateOnly), err)
		}
		err = cartridge.Write(context.Background(), dbManager, func(tx *gorm.DB) error {
			return writeVisitTime(tx, websiteID, day, seconds, rowOfBucket)
		})
		if err != nil {
			return fmt.Errorf("day %s: %w", day.Format(time.DateOnly), err)
		}
	}
	return nil
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

// writeVisitTime sets the visit time of one day's buckets: the tallied time,
// or 0 for a bucket without a visit.
func writeVisitTime(tx *gorm.DB, websiteID uint, day time.Time, seconds map[int64]float64, rowOfBucket map[int64]uint) error {
	now := time.Now().UTC()
	for bucket := day; bucket.Before(day.Add(24 * time.Hour)); bucket = bucket.Add(30 * time.Minute) {
		value := seconds[bucket.Unix()]
		if id, ok := rowOfBucket[bucket.Unix()]; ok {
			if err := tx.Table("site_stats").Where("id = ?", id).Update("visit_seconds", value).Error; err != nil {
				return fmt.Errorf("failed to update site stat %d: %w", id, err)
			}
			continue
		}
		if value == 0 {
			continue
		}
		err := tx.Exec(`
			INSERT INTO site_stats (website_id, hour, page_views, visitors, sessions, bounce_count, visit_seconds, created_at, updated_at)
			VALUES (?, ?, 0, 0, 0, 0, ?, ?, ?)
			ON CONFLICT (website_id, hour) DO UPDATE SET visit_seconds = excluded.visit_seconds
		`, websiteID, bucket, value, now, now).Error
		if err != nil {
			return fmt.Errorf("failed to add site stat: %w", err)
		}
	}
	return nil
}
