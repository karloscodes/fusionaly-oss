package events

import (
	"time"

	"gorm.io/gorm"
)

// EventFilters represents filtering options for events
type EventFilters struct {
	WebsiteID             uint
	FromDate              time.Time
	ToDate                time.Time
	URLFilter             string
	ReferrerFilter        string
	UserFilter            string
	TypeFilter            string // "page" or "event"
	CustomEventNameFilter string
	Limit                 int
	Offset                int
}

// EventsResult represents paginated events result
type EventsResult struct {
	Events []Event
	// Total counts the matching events, up to countAhead past the page.
	// TotalCapped says that more match.
	Total       int64
	TotalCapped bool
}

// countAhead is how far past the page the count reads. A filter on text (a
// URL, a referrer, an event name) can use no index, so an exact count read
// every event of the range: 0.4 s for 30 days of 1M events, on each page.
// The page shows "1,000+ events" instead, and the count grows as the user
// pages on.
const countAhead = 1000

// GetFilteredEvents retrieves filtered and paginated events
func GetFilteredEvents(db *gorm.DB, filters EventFilters) (EventsResult, error) {
	query := db.Model(&Event{}).
		Where("website_id = ? AND event_type != ?", filters.WebsiteID, EventTypePageHide).
		Where("timestamp BETWEEN ? AND ?", filters.FromDate, filters.ToDate)

	// Apply URL filter
	if filters.URLFilter != "" {
		query = query.Where("(hostname || pathname) LIKE ?", "%"+filters.URLFilter+"%")
	}

	// Apply referrer filter
	if filters.ReferrerFilter != "" {
		query = query.Where("(referrer_hostname || referrer_pathname) LIKE ?", "%"+filters.ReferrerFilter+"%")
	}

	// Apply user filter
	if filters.UserFilter != "" {
		query = query.Where("user_signature LIKE ?", "%"+filters.UserFilter+"%")
	}

	// Apply type filter
	if filters.TypeFilter != "" {
		if filters.TypeFilter == "page" {
			query = query.Where("event_type = ?", EventTypePageView)
		} else if filters.TypeFilter == "event" {
			query = query.Where("event_type = ?", EventTypeCustomEvent)
		}
	}

	// Apply custom event name filter
	if filters.CustomEventNameFilter != "" {
		query = query.Where("custom_event_name LIKE ?", "%"+filters.CustomEventNameFilter+"%")
	}

	// The page and the count start from the same filters.
	query = query.Session(&gorm.Session{})

	var events []Event
	if err := query.Order("timestamp DESC").
		Limit(filters.Limit).
		Offset(filters.Offset).
		Find(&events).Error; err != nil {
		return EventsResult{}, err
	}

	// A page that is not full is the last one, so it gives the total.
	if len(events) < filters.Limit && (len(events) > 0 || filters.Offset == 0) {
		return EventsResult{Events: events, Total: int64(filters.Offset + len(events))}, nil
	}

	counted := filters.Offset + countAhead
	var total int64
	if err := db.Table("(?) AS matches", query.Select("id").Limit(counted+1)).Count(&total).Error; err != nil {
		return EventsResult{}, err
	}
	return EventsResult{
		Events:      events,
		Total:       min(total, int64(counted)),
		TotalCapped: total > int64(counted),
	}, nil
}

// GetEventCountInTimeRange counts events for a website in a time range
func GetEventCountInTimeRange(db *gorm.DB, websiteID uint, from, to time.Time) (int64, error) {
	var count int64
	err := db.Model(&Event{}).
		Where("website_id = ? AND event_type != ? AND timestamp BETWEEN ? AND ?", websiteID, EventTypePageHide, from, to).
		Count(&count).Error
	return count, err
}
