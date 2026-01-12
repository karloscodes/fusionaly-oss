// Package analytics provides aggregate statistics models and query functions
// for analyzing website traffic, user behavior, and conversion metrics.
//
// The package is organized into focused modules:
//   - analytics.go: Aggregate table model definitions
//   - conversions.go: Goal conversion and device-based analytics
//   - flows.go: User flow and navigation path analysis
//   - metrics.go: Top-N queries (URLs, browsers, countries, etc.)
//   - referrers.go: Referrer tracking and normalization
//   - revenue.go: Revenue metrics and purchase analytics
//   - totals.go: Aggregate totals (visitors, sessions, page views, etc.)
//   - utm.go: UTM campaign parameter analytics
package analytics

import (
	"time"
)

// MetricCountResult represents a generic key-count pair for query results
type MetricCountResult struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// ===== Aggregate Table Definitions =====

// RefStat represents aggregated referrer statistics
type RefStat struct {
	ID             uint      `gorm:"primaryKey;autoIncrement"`
	WebsiteID      uint      `gorm:"uniqueIndex:idx_ref_unique;not null"`
	Hostname       string    `gorm:"uniqueIndex:idx_ref_unique;not null"`
	Pathname       string    `gorm:"uniqueIndex:idx_ref_unique"`
	VisitorsCount  int       `gorm:"not null;default:0"`
	PageViewsCount int       `gorm:"not null;default:0"`
	Hour           time.Time `gorm:"uniqueIndex:idx_ref_unique;type:datetime;not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// BrowserStat represents aggregated browser statistics
type BrowserStat struct {
	ID             uint      `gorm:"primaryKey;autoIncrement"`
	WebsiteID      uint      `gorm:"uniqueIndex:idx_browser_unique;not null"`
	Browser        string    `gorm:"uniqueIndex:idx_browser_unique;not null"`
	VisitorsCount  int       `gorm:"not null;default:0"`
	PageViewsCount int       `gorm:"not null;default:0"`
	Hour           time.Time `gorm:"uniqueIndex:idx_browser_unique;type:datetime;not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// OSStat represents aggregated operating system statistics
type OSStat struct {
	ID              uint      `gorm:"primaryKey;autoIncrement"`
	WebsiteID       uint      `gorm:"uniqueIndex:idx_os_unique;not null"`
	OperatingSystem string    `gorm:"uniqueIndex:idx_os_unique;not null"`
	VisitorsCount   int       `gorm:"not null;default:0"`
	PageViewsCount  int       `gorm:"not null;default:0"`
	Hour            time.Time `gorm:"uniqueIndex:idx_os_unique;type:datetime;not null"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// DeviceStat represents aggregated device type statistics
type DeviceStat struct {
	ID             uint      `gorm:"primaryKey;autoIncrement"`
	WebsiteID      uint      `gorm:"uniqueIndex:idx_device_unique;not null"`
	DeviceType     string    `gorm:"uniqueIndex:idx_device_unique;not null"`
	VisitorsCount  int       `gorm:"not null;default:0"`
	PageViewsCount int       `gorm:"not null;default:0"`
	Hour           time.Time `gorm:"uniqueIndex:idx_device_unique;type:datetime;not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// CountryStat represents aggregated country statistics
type CountryStat struct {
	ID             uint      `gorm:"primaryKey;autoIncrement"`
	WebsiteID      uint      `gorm:"uniqueIndex:idx_country_unique;not null"`
	Country        string    `gorm:"uniqueIndex:idx_country_unique;not null"`
	VisitorsCount  int       `gorm:"not null;default:0"`
	PageViewsCount int       `gorm:"not null;default:0"`
	Hour           time.Time `gorm:"uniqueIndex:idx_country_unique;type:datetime;not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// UTMStat represents aggregated UTM parameter statistics
type UTMStat struct {
	ID             uint      `gorm:"primaryKey;autoIncrement"`
	WebsiteID      uint      `gorm:"uniqueIndex:idx_utm_unique;not null"`
	UTMSource      string    `gorm:"uniqueIndex:idx_utm_unique"`
	UTMMedium      string    `gorm:"uniqueIndex:idx_utm_unique"`
	UTMCampaign    string    `gorm:"uniqueIndex:idx_utm_unique"`
	UTMTerm        string    `gorm:"uniqueIndex:idx_utm_unique"`
	UTMContent     string    `gorm:"uniqueIndex:idx_utm_unique"`
	VisitorsCount  int       `gorm:"not null;default:0"`
	PageViewsCount int       `gorm:"not null;default:0"`
	Hour           time.Time `gorm:"uniqueIndex:idx_utm_unique;type:datetime;not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// EventStat represents aggregated custom event statistics
type EventStat struct {
	ID             uint      `gorm:"primaryKey;autoIncrement"`
	WebsiteID      uint      `gorm:"uniqueIndex:idx_event_unique;not null"`
	EventName      string    `gorm:"uniqueIndex:idx_event_unique;not null"`
	EventKey       string    `gorm:"uniqueIndex:idx_event_unique"`
	VisitorsCount  int       `gorm:"not null;default:0"`
	PageViewsCount int       `gorm:"not null;default:0"`
	Hour           time.Time `gorm:"uniqueIndex:idx_event_unique;type:datetime;not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// QueryParamStat represents aggregated query string parameter statistics
type QueryParamStat struct {
	ID             uint      `gorm:"primaryKey;autoIncrement"`
	WebsiteID      uint      `gorm:"uniqueIndex:idx_query_param_unique;not null"`
	ParamName      string    `gorm:"uniqueIndex:idx_query_param_unique;not null"`
	ParamValue     string    `gorm:"uniqueIndex:idx_query_param_unique;not null"`
	VisitorsCount  int       `gorm:"not null;default:0"`
	PageViewsCount int       `gorm:"not null;default:0"`
	Hour           time.Time `gorm:"uniqueIndex:idx_query_param_unique;type:datetime;not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// FlowTransitionStat represents aggregated page-to-page transitions for user flow analysis
// Transitions are stored with step positions to enable Sankey diagram rendering
type FlowTransitionStat struct {
	ID           uint      `gorm:"primaryKey;autoIncrement"`
	WebsiteID    uint      `gorm:"uniqueIndex:idx_flow_transition_unique;not null"`
	StepPosition int       `gorm:"uniqueIndex:idx_flow_transition_unique;not null"` // 1, 2, 3, etc.
	SourcePage   string    `gorm:"uniqueIndex:idx_flow_transition_unique;not null"` // hostname + pathname
	TargetPage   string    `gorm:"uniqueIndex:idx_flow_transition_unique;not null"` // hostname + pathname
	Transitions  int       `gorm:"not null;default:0"`                              // count of transitions
	Hour         time.Time `gorm:"uniqueIndex:idx_flow_transition_unique;type:datetime;not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
