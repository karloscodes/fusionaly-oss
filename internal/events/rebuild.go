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

// keyVisitCountsRebuilt marks that the stored bounces, exits, and page
// visitors were rebuilt with the visit rules of v2.4.3.
const keyVisitCountsRebuilt = "visit_counts_rebuilt_v1"

// RebuildVisitCountsOnce rebuilds the stored visit counts from the events
// table, one time per install. Versions before v2.4.3 counted them wrong.
// Later fixes apply forward only and do not run it again. The event
// processor calls it, so no event is counted while the rebuild runs.
func RebuildVisitCountsOnce(db *gorm.DB, logger *slog.Logger) error {
	if done, _ := settings.GetSetting(db, keyVisitCountsRebuilt); done != "" {
		return nil
	}

	var websiteIDs []uint
	if err := db.Model(&Event{}).Distinct().Pluck("website_id", &websiteIDs).Error; err != nil {
		return fmt.Errorf("failed to list websites with events: %w", err)
	}

	started := time.Now()
	for _, id := range websiteIDs {
		if err := RebuildVisitCounts(db, id); err != nil {
			return fmt.Errorf("website %d: %w", id, err)
		}
	}

	logger.Info("Rebuilt visit counts",
		slog.Int("websites", len(websiteIDs)),
		slog.Duration("took", time.Since(started)))
	return settings.CreateOrUpdateSetting(db, keyVisitCountsRebuilt, time.Now().UTC().Format(time.RFC3339))
}

// pageBucket is one row of page_stats: a page in one half hour.
type pageBucket struct {
	hostname, pathname string
	hour               int64 // Unix seconds of the half-hour bucket
}

type visitTally struct {
	visitors    map[int64]int // site visitors, at their first page view
	sessions    map[int64]int // visits, at their first page view
	bounces     map[int64]int
	entrances   map[pageBucket]int
	exits       map[pageBucket]int
	pageVisitor map[pageBucket]int
}

// RebuildVisitCounts recounts one website's visit counts with the same
// rules as live processing:
//   - a visit ends after SessionTimeoutSeconds without an event of any type
//   - a visitor and a visit count at their first page view; that page view
//     is the visit's entrance
//   - a visit is a bounce when it has exactly one page view and no
//     engagement (a custom event other than scroll:*)
//   - a visit's last page view is its exit
//   - a page counts each visitor once, at their first view of it
//
// Source stats (ref_stats, utm_stats, query_param_stats) are not rebuilt.
// A visit's source can be its utm_source or ref, and events keep neither
// the URL nor the source, only the raw referrer.
func RebuildVisitCounts(db *gorm.DB, websiteID uint) error {
	tally, err := tallyVisits(db, websiteID)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		return writeVisitTally(tx, websiteID, tally)
	})
}

func bucketOf(t time.Time) int64 {
	return truncateToHalfHour(t.UTC()).Unix()
}

func tallyVisits(db *gorm.DB, websiteID uint) (visitTally, error) {
	tally := visitTally{
		visitors: map[int64]int{}, sessions: map[int64]int{}, bounces: map[int64]int{},
		entrances: map[pageBucket]int{}, exits: map[pageBucket]int{},
		pageVisitor: map[pageBucket]int{},
	}
	timeout := time.Duration(config.GetConfig().SessionTimeoutSeconds) * time.Second

	rows, err := db.Model(&Event{}).
		Select("user_signature, hostname, pathname, event_type, custom_event_name, timestamp").
		Where("website_id = ?", websiteID).
		Order("user_signature, timestamp, id").
		Rows()
	if err != nil {
		return tally, fmt.Errorf("failed to read events: %w", err)
	}
	defer rows.Close()

	var (
		signature   string
		lastEvent   time.Time
		pageViews   int // in the current visit
		engaged     bool
		entry       pageBucket
		lastPage    pageBucket
		seenPages   map[string]bool
		visitorSeen bool
	)
	endVisit := func() {
		if pageViews == 1 && !engaged {
			tally.bounces[entry.hour]++
		}
		if pageViews > 0 {
			tally.exits[lastPage]++
		}
	}

	for rows.Next() {
		var e Event
		if err := db.ScanRows(rows, &e); err != nil {
			return tally, fmt.Errorf("failed to scan event: %w", err)
		}

		newVisitor := e.UserSignature != signature || signature == ""
		if newVisitor || e.Timestamp.Sub(lastEvent) > timeout {
			if !lastEvent.IsZero() {
				endVisit()
			}
			pageViews, engaged = 0, false
		}
		if newVisitor {
			signature, seenPages, visitorSeen = e.UserSignature, map[string]bool{}, false
		}
		lastEvent = e.Timestamp

		if e.EventType == EventTypeCustomEvent {
			engaged = engaged || isEngagement(e.CustomEventName)
		}
		if e.EventType != EventTypePageView {
			continue // a page hide only keeps the visit alive
		}
		page := pageBucket{e.Hostname, e.Pathname, bucketOf(e.Timestamp)}
		if !visitorSeen {
			visitorSeen = true
			tally.visitors[page.hour]++
		}
		if pageViews == 0 {
			entry = page
			tally.sessions[page.hour]++
			tally.entrances[page]++
		}
		pageViews++
		lastPage = page
		if key := e.Hostname + "\x00" + e.Pathname; !seenPages[key] {
			seenPages[key] = true
			tally.pageVisitor[page]++
		}
	}
	if err := rows.Err(); err != nil {
		return tally, fmt.Errorf("failed to read events: %w", err)
	}
	if !lastEvent.IsZero() {
		endVisit()
	}
	return tally, nil
}

// writeVisitTally replaces the stored counts with the tally. It matches rows
// by their bucket's instant, not by the stored text, so rows written with
// another time zone offset still match.
func writeVisitTally(tx *gorm.DB, websiteID uint, tally visitTally) error {
	type statRow struct {
		ID       uint
		Hostname string
		Pathname string
		Hour     time.Time
	}

	var pages []statRow
	if err := tx.Table("page_stats").Select("id, hostname, pathname, hour").Where("website_id = ?", websiteID).Scan(&pages).Error; err != nil {
		return fmt.Errorf("failed to read page stats: %w", err)
	}
	for _, p := range pages {
		key := pageBucket{p.Hostname, p.Pathname, bucketOf(p.Hour)}
		err := tx.Table("page_stats").Where("id = ?", p.ID).Updates(map[string]any{
			"exits": tally.exits[key], "visitors_count": tally.pageVisitor[key], "entrances": tally.entrances[key],
		}).Error
		if err != nil {
			return fmt.Errorf("failed to update page stat %d: %w", p.ID, err)
		}
	}

	var sites []statRow
	if err := tx.Table("site_stats").Select("id, hour").Where("website_id = ?", websiteID).Scan(&sites).Error; err != nil {
		return fmt.Errorf("failed to read site stats: %w", err)
	}
	for _, s := range sites {
		hour := bucketOf(s.Hour)
		err := tx.Table("site_stats").Where("id = ?", s.ID).Updates(map[string]any{
			"bounce_count": tally.bounces[hour], "visitors": tally.visitors[hour], "sessions": tally.sessions[hour],
		}).Error
		if err != nil {
			return fmt.Errorf("failed to update site stat %d: %w", s.ID, err)
		}
	}
	return nil
}

// dayValues holds the rebuilt values of one UTC day: column, bucket, value.
type dayValues map[string]map[int64]float64

// rebuildOnce runs a rebuild for every website with events, one time per
// install. The event processor calls it before it processes new events, so
// no event counts twice. A failed run is retried on the next one: each day
// is set, not added to, so a rerun is safe.
func rebuildOnce(dbManager cartridge.DBManager, logger *slog.Logger, key, name string, rebuild func(cartridge.DBManager, uint) error) error {
	db := dbManager.GetConnection()
	if done, _ := settings.GetSetting(db, key); done != "" {
		return nil
	}

	var websiteIDs []uint
	if err := db.Model(&Event{}).Distinct().Pluck("website_id", &websiteIDs).Error; err != nil {
		return fmt.Errorf("failed to list websites with events: %w", err)
	}

	started := time.Now()
	for _, id := range websiteIDs {
		if err := rebuild(dbManager, id); err != nil {
			return fmt.Errorf("website %d: %w", id, err)
		}
	}

	logger.Info("Rebuilt "+name,
		slog.Int("websites", len(websiteIDs)),
		slog.Duration("took", time.Since(started)))
	return settings.CreateOrUpdateSetting(db, key, time.Now().UTC().Format(time.RFC3339))
}

// rebuildByDay sets site_stats columns of one website from its events, one
// UTC day per write. A signature lasts one UTC day, so no visit or visitor
// spans two days, and ingestion goes on between the days. tally returns the
// values of one day; a bucket without a value gets 0.
func rebuildByDay(dbManager cartridge.DBManager, websiteID uint, tally func(*gorm.DB, uint, time.Time) (dayValues, error)) error {
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
		values, err := tally(db, websiteID, day)
		if err != nil {
			return fmt.Errorf("day %s: %w", day.Format(time.DateOnly), err)
		}
		err = cartridge.Write(context.Background(), dbManager, func(tx *gorm.DB) error {
			return writeDay(tx, websiteID, day, values, rowOfBucket)
		})
		if err != nil {
			return fmt.Errorf("day %s: %w", day.Format(time.DateOnly), err)
		}
	}
	return nil
}

// writeDay sets the values of one day's buckets.
func writeDay(tx *gorm.DB, websiteID uint, day time.Time, values dayValues, rowOfBucket map[int64]uint) error {
	for bucket := day; bucket.Before(day.Add(24 * time.Hour)); bucket = bucket.Add(30 * time.Minute) {
		row := map[string]any{}
		empty := true
		for column, byBucket := range values {
			row[column] = byBucket[bucket.Unix()]
			empty = empty && byBucket[bucket.Unix()] == 0
		}

		id, ok := rowOfBucket[bucket.Unix()]
		if !ok && empty {
			continue
		}
		if !ok {
			now := time.Now().UTC()
			err := tx.Exec(`INSERT INTO site_stats (website_id, hour, created_at, updated_at) VALUES (?, ?, ?, ?)`,
				websiteID, bucket, now, now).Error
			if err != nil {
				return fmt.Errorf("failed to add site stat: %w", err)
			}
			if err := tx.Raw("SELECT last_insert_rowid()").Scan(&id).Error; err != nil {
				return fmt.Errorf("failed to add site stat: %w", err)
			}
			rowOfBucket[bucket.Unix()] = id
		}
		if err := tx.Table("site_stats").Where("id = ?", id).Updates(row).Error; err != nil {
			return fmt.Errorf("failed to update site stat %d: %w", id, err)
		}
	}
	return nil
}
