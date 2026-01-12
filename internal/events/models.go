package events

import "time"

// EventType represents the type of event.
type EventType int

const (
	EventTypePageView    EventType = 1
	EventTypeCustomEvent EventType = 2
)

// Event represents a tracked page view or custom event in the main database.
type Event struct {
	ID               uint   `gorm:"primaryKey;autoIncrement"`
	WebsiteID        uint   `gorm:"index:idx_website_timestamp;not null"`
	UserSignature    string `gorm:"index;size:64;not null"`
	Hostname         string `gorm:"index;not null"`
	Pathname         string `gorm:"index;not null"`
	ReferrerHostname string `gorm:"index"`
	ReferrerPathname string
	EventType        EventType `gorm:"not null;default:1"`
	CustomEventName  string    `gorm:"index"`
	CustomEventMeta  string    `gorm:"type:text"`
	Timestamp        time.Time `gorm:"index:idx_website_timestamp;not null"`
	CreatedAt        time.Time
}

// EventProcessingData holds enriched data for updating aggregates.
// It is produced by the event processing pipeline.
type EventProcessingData struct {
	EventID          uint
	WebsiteID        uint
	UserSignature    string
	Hostname         string
	Pathname         string
	ReferrerHostname string
	ReferrerPathname string
	DeviceType       string
	Browser          string
	OperatingSystem  string
	Country          string
	UTMSource        string
	UTMMedium        string
	UTMCampaign      string
	UTMTerm          string
	UTMContent       string
	QueryParams      map[string]string // All query string parameters
	CustomEventName  string
	CustomEventKey   string
	EventType        EventType
	IsNewVisitor     bool
	IsNewSession     bool
	Timestamp        time.Time
	IsEntrance       bool
	IsExit           bool
	IsBounce         bool
	HasUTM           bool
}
