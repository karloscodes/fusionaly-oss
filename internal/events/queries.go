package events

import (
	"time"

	"gorm.io/gorm"
)

// EventFilters represents filtering options for events
type EventFilters struct {
	WebsiteID            uint
	FromDate             time.Time
	ToDate               time.Time
	URLFilter            string
	ReferrerFilter       string
	UserFilter           string
	TypeFilter           string // "page" or "event"
	CustomEventNameFilter string
	Limit                int
	Offset               int
}

// EventsResult represents paginated events result
type EventsResult struct {
	Events []Event
	Total  int64
}

// GetFilteredEvents retrieves filtered and paginated events
func GetFilteredEvents(db *gorm.DB, filters EventFilters) (EventsResult, error) {
	query := db.Model(&Event{}).
		Where("website_id = ?", filters.WebsiteID).
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

	// Get total count for pagination
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return EventsResult{}, err
	}

	// Get paginated events
	var events []Event
	if err := query.Order("timestamp DESC").
		Limit(filters.Limit).
		Offset(filters.Offset).
		Find(&events).Error; err != nil {
		return EventsResult{}, err
	}

	return EventsResult{
		Events: events,
		Total:  total,
	}, nil
}

// GetEventCountInTimeRange counts events for a website in a time range
func GetEventCountInTimeRange(db *gorm.DB, websiteID uint, from, to time.Time) (int64, error) {
	var count int64
	err := db.Model(&Event{}).
		Where("website_id = ? AND timestamp BETWEEN ? AND ?", websiteID, from, to).
		Count(&count).Error
	return count, err
}
