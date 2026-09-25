package events

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"fusionaly/internal/config"
	"fusionaly/internal/settings"
)

// keyVisitCountsRebuilt marks that the stored bounces, exits, and page
// visitors were rebuilt with the visit rules of v2.4.3.
const keyVisitCountsRebuilt = "visit_counts_rebuilt_v1"

// RebuildVisitCountsOnce rebuilds the stored bounces, exits, and page
// visitors from the events table, one time per install. Versions before
// v2.4.3 counted them wrong. The event processor calls it, so no event is
// counted while the rebuild runs.
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

	logger.Info("Rebuilt bounces, exits, and page visitors",
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
	bounces  map[int64]int
	exits    map[pageBucket]int
	visitors map[pageBucket]int
}

// RebuildVisitCounts recounts one website's bounces, exits, and page
// visitors with the same rules as live processing:
//   - a visit ends after SessionTimeoutSeconds without an event
//   - a visit is a bounce when it opens with a page view and has no other
//   - a visit's last page view is its exit
//   - a page counts each visitor once, at their first view of it
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
	tally := visitTally{bounces: map[int64]int{}, exits: map[pageBucket]int{}, visitors: map[pageBucket]int{}}
	timeout := time.Duration(config.GetConfig().SessionTimeoutSeconds) * time.Second

	rows, err := db.Model(&Event{}).
		Select("user_signature, hostname, pathname, event_type, timestamp").
		Where("website_id = ?", websiteID).
		Order("user_signature, timestamp, id").
		Rows()
	if err != nil {
		return tally, fmt.Errorf("failed to read events: %w", err)
	}
	defer rows.Close()

	var (
		signature         string
		lastEvent         time.Time
		visitStart        time.Time
		opensWithPageView bool
		pageViews         int
		lastPage          pageBucket
		seenPages         map[string]bool
	)
	endVisit := func() {
		if opensWithPageView && pageViews == 1 {
			tally.bounces[bucketOf(visitStart)]++
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
			visitStart, opensWithPageView, pageViews = e.Timestamp, e.EventType == EventTypePageView, 0
		}
		if newVisitor {
			signature, seenPages = e.UserSignature, map[string]bool{}
		}
		lastEvent = e.Timestamp

		if e.EventType != EventTypePageView {
			continue
		}
		pageViews++
		lastPage = pageBucket{e.Hostname, e.Pathname, bucketOf(e.Timestamp)}
		if page := e.Hostname + "\x00" + e.Pathname; !seenPages[page] {
			seenPages[page] = true
			tally.visitors[lastPage]++
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
		exits, visitors := tally.exits[key], tally.visitors[key]
		if err := tx.Table("page_stats").Where("id = ?", p.ID).
			Updates(map[string]any{"exits": exits, "visitors_count": visitors}).Error; err != nil {
			return fmt.Errorf("failed to update page stat %d: %w", p.ID, err)
		}
	}

	var sites []statRow
	if err := tx.Table("site_stats").Select("id, hour").Where("website_id = ?", websiteID).Scan(&sites).Error; err != nil {
		return fmt.Errorf("failed to read site stats: %w", err)
	}
	for _, s := range sites {
		if err := tx.Table("site_stats").Where("id = ?", s.ID).
			Update("bounce_count", tally.bounces[bucketOf(s.Hour)]).Error; err != nil {
			return fmt.Errorf("failed to update site stat %d: %w", s.ID, err)
		}
	}
	return nil
}
