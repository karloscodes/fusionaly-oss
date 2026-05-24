// Package feed provides the cross-site activity feed for Fusionaly.
// Philosophy: Show what changed. Not charts. Not dashboards. Just events worth knowing.
package feed

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// ItemType represents the type of feed item
type ItemType string

const (
	ItemTypeTrafficSpike    ItemType = "traffic_spike"
	ItemTypeTrafficDrop     ItemType = "traffic_drop"
	ItemTypeNewReferrer     ItemType = "new_referrer"
	ItemTypeGoalHit         ItemType = "goal_hit"
	ItemTypeMilestone       ItemType = "milestone"
	ItemTypeTrendingContent ItemType = "trending_content"
	ItemTypeDailySummary    ItemType = "daily_summary"
	ItemTypeMonthlySummary  ItemType = "monthly_summary"
	ItemTypeDroppingPages   ItemType = "dropping_pages"
	ItemTypeBestSources     ItemType = "best_sources"
)

// FeedItem represents a single item in the activity feed
type FeedItem struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	WebsiteID   uint      `gorm:"not null;index:idx_feed_website_detected,priority:1" json:"websiteId"`
	ItemType    ItemType  `gorm:"not null;size:50" json:"itemType"`
	Title       string    `gorm:"not null;size:100" json:"title"`
	Description string    `gorm:"not null;size:500" json:"description"`
	DetectedAt  time.Time `gorm:"not null;index:idx_feed_website_detected,priority:2,sort:desc" json:"detectedAt"`
	PeriodStart time.Time `gorm:"not null" json:"periodStart"`
	PeriodEnd   time.Time `gorm:"not null" json:"periodEnd"`
	Metadata    string    `gorm:"type:text" json:"metadata,omitempty"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"createdAt"`
}

func (FeedItem) TableName() string {
	return "feed_items"
}

// MetadataMap returns the metadata as a map
func (f *FeedItem) MetadataMap() map[string]any {
	if f.Metadata == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(f.Metadata), &m); err != nil {
		return nil
	}
	return m
}

// SetMetadata sets metadata from a map
func (f *FeedItem) SetMetadata(m map[string]any) {
	if m == nil {
		f.Metadata = ""
		return
	}
	b, err := json.Marshal(m)
	if err != nil {
		f.Metadata = ""
		return
	}
	f.Metadata = string(b)
}

// AutoMigrate creates or updates the feed tables
func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&FeedItem{}); err != nil {
		return err
	}
	return db.AutoMigrate(&FeedBaseline{})
}

// GetUserFeed retrieves feed items for all websites the user has access to
func GetUserFeed(db *gorm.DB, websiteIDs []uint, limit int) ([]FeedItem, error) {
	var items []FeedItem

	if len(websiteIDs) == 0 {
		return items, nil
	}

	err := db.Where("website_id IN ?", websiteIDs).
		Order("detected_at DESC").
		Limit(limit).
		Find(&items).Error

	return items, err
}

// CreateItem creates a new feed item, avoiding duplicates
func CreateItem(db *gorm.DB, item *FeedItem) error {
	// Check for duplicate (same website, type, and period start)
	var existing FeedItem
	result := db.Where(
		"website_id = ? AND item_type = ? AND period_start = ?",
		item.WebsiteID, item.ItemType, item.PeriodStart,
	).First(&existing)

	if result.Error == nil {
		// Already exists, skip
		return nil
	}

	return db.Create(item).Error
}

// CleanupOldItems removes feed items older than the specified duration
func CleanupOldItems(db *gorm.DB, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	return db.Where("detected_at < ?", cutoff).Delete(&FeedItem{}).Error
}

// CleanupLegacyDrops removes traffic_drop items that predate the retuned drop
// detector — drops on sites below the MinDropVisitors volume floor. The old rule
// used a scale-free z-score and flagged meaningless dips on tiny sites (an 11→1
// "Slow day"). The new rule only fires on sites averaging >= MinDropVisitors, so
// any stored drop with a smaller baseline is noise we now suppress. Safe to run
// repeatedly: the current detector never creates such rows, so it only ever
// deletes legacy noise, never a legitimate drop.
func CleanupLegacyDrops(db *gorm.DB) error {
	var drops []FeedItem
	if err := db.Where("item_type = ?", ItemTypeTrafficDrop).Find(&drops).Error; err != nil {
		return err
	}

	var staleIDs []uint
	for _, d := range drops {
		// JSON numbers decode as float64; a missing/zero baseline is also legacy noise.
		avg, _ := d.MetadataMap()["avgVisitors"].(float64)
		if avg < MinDropVisitors {
			staleIDs = append(staleIDs, d.ID)
		}
	}

	if len(staleIDs) == 0 {
		return nil
	}
	return db.Where("id IN ?", staleIDs).Delete(&FeedItem{}).Error
}

// ClearAllItems removes all feed items (for dev reset)
func ClearAllItems(db *gorm.DB) error {
	return db.Where("1 = 1").Delete(&FeedItem{}).Error
}
