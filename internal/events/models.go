package events

import "time"

// EventType represents the type of event.
type EventType int

const (
	EventTypePageView    EventType = 1
	EventTypeCustomEvent EventType = 2
	// EventTypePageHide marks the time a page view was hidden (tab switch,
	// close, navigation). It only extends its visit, for the average visit
	// duration, and keeps the visit alive. It is no page view, no custom
	// event, and no engagement: no counter, list, or flow reads it.
	EventTypePageHide EventType = 3
)

// Valid tells if the event type is one that ingestion accepts.
func (t EventType) Valid() bool {
	return t == EventTypePageView || t == EventTypeCustomEvent || t == EventTypePageHide
}

// Event represents a tracked page view or custom event in the main database.
type Event struct {
	ID               uint   `gorm:"primaryKey;autoIncrement"`
	WebsiteID        uint   `gorm:"index:idx_website_timestamp;index:idx_events_visitors,priority:1;not null"`
	UserSignature    string `gorm:"index;index:idx_events_visitors,priority:4;size:64;not null"`
	Hostname         string `gorm:"index;not null"`
	Pathname         string `gorm:"index;not null"`
	ReferrerHostname string `gorm:"index"`
	ReferrerPathname string
	EventType        EventType `gorm:"index:idx_events_visitors,priority:3;not null;default:1"`
	CustomEventName  string    `gorm:"index"`
	CustomEventMeta  string    `gorm:"type:text"`
	// idx_events_visitors covers the visitor counts (distinct signatures of
	// page views per website and time range), so they never read the rows.
	Timestamp time.Time `gorm:"index:idx_website_timestamp;index:idx_events_visitors,priority:2;not null"`
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
	UnbounceAt       *time.Time   // the visit's only page view: no longer a bounce
	IsBounce         bool         // the visit's first page view, with no engagement yet
}

// PageViewRef identifies an earlier page view by where it was counted.
type PageViewRef struct {
	Hostname  string
	Pathname  string
	Timestamp time.Time
}
