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
	// SessionStart is the time of the visit's first event. It links the events
	// of one visit, so a later page view can correct the visit's bounce and
	// exit. Nil for events recorded before it existed.
	SessionStart *time.Time `gorm:"index"`
	CreatedAt    time.Time
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
	IsNewPageVisitor bool // first view of this page by this visitor
	HasUTM           bool
	// Set when this page view continues a visit whose earlier page views were
	// counted provisionally (see processed.go).
	PreviousPageView *PageViewRef // the visit's previous page view: no longer its exit
	UnbounceAt       *time.Time   // the visit's entry: no longer a bounce
}

// PageViewRef identifies an earlier page view by where it was counted.
type PageViewRef struct {
	Hostname  string
	Pathname  string
	Timestamp time.Time
}
