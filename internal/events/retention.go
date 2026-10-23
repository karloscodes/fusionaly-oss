package events

import (
	"fmt"
	"time"

	"github.com/karloscodes/cartridge"

	"fusionaly/internal/config"
	"fusionaly/internal/pkg/batch"
)

// KeptSince returns the start of the oldest UTC day whose raw events are all
// kept, or the zero time when FUSIONALY_EVENTS_RETENTION_DAYS is 0, which
// keeps every event.
//
// A recount must not touch the summaries of older days: their events are
// gone, so the recount would write zeros over them. A visit never crosses
// midnight UTC, because the visitor signature changes then, so a cut at the
// start of a UTC day splits no visit.
//
// KeptSince is one day after the cut of DeleteOldEvents. Versions before
// v2.6.2 stored event times with the server's offset, and SQLite compares
// the stored text, so the cut can be off by up to 14 hours for those events.
func KeptSince(now time.Time) time.Time {
	cut := deleteBefore(now)
	if cut.IsZero() {
		return cut
	}
	return cut.Add(24 * time.Hour)
}

// deleteBefore returns the start of the UTC day before which DeleteOldEvents
// deletes events, or the zero time when it keeps every event.
func deleteBefore(now time.Time) time.Time {
	days := config.GetConfig().EventsRetentionDays
	if days <= 0 {
		return time.Time{}
	}
	return now.UTC().Truncate(24*time.Hour).AddDate(0, 0, -days)
}

// DeleteOldEvents deletes the raw events older than
// FUSIONALY_EVENTS_RETENTION_DAYS days, from the start of a UTC day, except
// the events with a price, which revenue reads. The summaries keep every
// chart. It returns the number of events it deleted.
func DeleteOldEvents(dbManager cartridge.DBManager, now time.Time) (int64, error) {
	cut := deleteBefore(now)
	if cut.IsZero() {
		return 0, nil
	}

	var websiteIDs []uint
	if err := dbManager.GetConnection().Table("websites").Pluck("id", &websiteIDs).Error; err != nil {
		return 0, fmt.Errorf("failed to list websites: %w", err)
	}

	// One website at a time, so each batch reads the (website_id, timestamp)
	// index instead of the whole table.
	query := `DELETE FROM events WHERE id IN (SELECT id FROM events
		WHERE website_id = ? AND timestamp < ?
		AND (custom_event_meta IS NULL OR NOT ` + PricedEventSQL + `) LIMIT ?)`

	var total int64
	for _, id := range websiteIDs {
		deleted, err := batch.Delete(dbManager, query, id, cut)
		total += deleted
		if err != nil {
			return total, fmt.Errorf("failed to delete old events of website %d: %w", id, err)
		}
	}
	return total, nil
}
