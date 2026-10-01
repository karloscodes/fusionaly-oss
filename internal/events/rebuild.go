package events

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"fusionaly/internal/config"
	"fusionaly/internal/settings"
)

// keyVisitCountsRebuilt marks that the stored visit counts were rebuilt
// with the visit rules of v2.6.2. v1 marked the rebuild of v2.4.3, which did
// not yet count a visit that opens with a custom event.
const keyVisitCountsRebuilt = "visit_counts_rebuilt_v2"

// RebuildVisitCountsOnce rebuilds the stored visit counts from the events
// table, one time per install. Versions before v2.6.2 counted them wrong.
// The event processor calls it, so no event is counted while the rebuild
// runs.
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

// pageBucket is one row of page_stats: a page in one half hour. ref_stats
// rows use it too, with the referrer's hostname and pathname.
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
	refVisitors map[pageBucket]int
}

// RebuildVisitCounts recounts one website's visit counts with the same
// rules as live processing:
//   - a visit ends after SessionTimeoutSeconds without an event of any type
//   - a visitor and a visit count at their first page view, and the visit's
//     referrer gets its visitor there; that page view is the visit's entrance
//   - a visit is a bounce when it has exactly one page view
//   - a visit's last page view is its exit
//   - a page counts each visitor once, at their first view of it
//
// UTM and query parameter stats are not rebuilt: events do not keep the URL.
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
		pageVisitor: map[pageBucket]int{}, refVisitors: map[pageBucket]int{},
	}
	timeout := time.Duration(config.GetConfig().SessionTimeoutSeconds) * time.Second

	rows, err := db.Model(&Event{}).
		Select("user_signature, hostname, pathname, referrer_hostname, referrer_pathname, event_type, timestamp").
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
		entry       pageBucket
		lastPage    pageBucket
		seenPages   map[string]bool
		visitorSeen bool
	)
	endVisit := func() {
		if pageViews == 1 {
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
			pageViews = 0
		}
		if newVisitor {
			signature, seenPages, visitorSeen = e.UserSignature, map[string]bool{}, false
		}
		lastEvent = e.Timestamp

		if e.EventType != EventTypePageView {
			continue
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
			tally.refVisitors[pageBucket{e.ReferrerHostname, e.ReferrerPathname, page.hour}]++
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

	var refs []statRow
	if err := tx.Table("ref_stats").Select("id, hostname, pathname, hour").Where("website_id = ?", websiteID).Scan(&refs).Error; err != nil {
		return fmt.Errorf("failed to read ref stats: %w", err)
	}
	for _, r := range refs {
		key := pageBucket{r.Hostname, r.Pathname, bucketOf(r.Hour)}
		if err := tx.Table("ref_stats").Where("id = ?", r.ID).Update("visitors_count", tally.refVisitors[key]).Error; err != nil {
			return fmt.Errorf("failed to update ref stat %d: %w", r.ID, err)
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
