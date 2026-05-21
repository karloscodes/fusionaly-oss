package feed

import (
	"math"
	"time"

	"gorm.io/gorm"
)

// FeedBaseline stores learned statistics for adaptive detection.
// Each website/metric/hour-of-week combination has its own baseline,
// allowing detection to adapt to weekly patterns.
type FeedBaseline struct {
	ID          uint      `gorm:"primaryKey"`
	WebsiteID   uint      `gorm:"not null;uniqueIndex:idx_baseline_unique,priority:1"`
	Metric      string    `gorm:"not null;size:100;uniqueIndex:idx_baseline_unique,priority:2"` // "pageviews", "goal_signup", "page_/blog"
	HourOfWeek  int       `gorm:"not null;uniqueIndex:idx_baseline_unique,priority:3"`          // 0-167
	Mean        float64   `gorm:"default:0"`
	StdDev      float64   `gorm:"default:0"`
	SampleCount int       `gorm:"default:0"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

func (FeedBaseline) TableName() string {
	return "feed_baselines"
}

// GetOrCreateBaseline retrieves or creates a baseline for the given key.
func GetOrCreateBaseline(db *gorm.DB, websiteID uint, metric string, hourOfWeek int) *FeedBaseline {
	var baseline FeedBaseline
	result := db.Where("website_id = ? AND metric = ? AND hour_of_week = ?",
		websiteID, metric, hourOfWeek).First(&baseline)

	if result.Error != nil {
		baseline = FeedBaseline{
			WebsiteID:   websiteID,
			Metric:      metric,
			HourOfWeek:  hourOfWeek,
			Mean:        DefaultMean,
			StdDev:      DefaultStdDev,
			SampleCount: 0,
			UpdatedAt:   time.Now(),
		}
		db.Create(&baseline)
	}

	return &baseline
}

// UpdateBaseline updates the baseline with a new observation using EMA.
// Uses Exponential Moving Average for mean and Welford's algorithm for stddev.
func UpdateBaseline(db *gorm.DB, baseline *FeedBaseline, newValue float64) {
	// Exponential moving average for mean
	baseline.Mean = (1-BaselineSmoothingFactor)*baseline.Mean + BaselineSmoothingFactor*newValue

	// Welford's online algorithm for stddev (simplified EMA version)
	diff := newValue - baseline.Mean
	baseline.StdDev = math.Sqrt(
		(1-BaselineSmoothingFactor)*baseline.StdDev*baseline.StdDev +
			BaselineSmoothingFactor*diff*diff,
	)

	baseline.SampleCount++
	baseline.UpdatedAt = time.Now()

	db.Save(baseline)
}

// GetEffectiveBaseline returns the mean and stddev to use for detection.
// Uses defaults during cold start (insufficient samples).
func GetEffectiveBaseline(baseline *FeedBaseline) (mean, stddev float64) {
	if baseline.SampleCount < MinSamplesForBaseline {
		return DefaultMean, DefaultStdDev
	}
	return baseline.Mean, baseline.StdDev
}

// AutoMigrateBaselines creates or updates the feed_baselines table.
func AutoMigrateBaselines(db *gorm.DB) error {
	return db.AutoMigrate(&FeedBaseline{})
}

// LearnBaseline is a convenience function that gets or creates a baseline
// and updates it with a new value in one call.
func LearnBaseline(db *gorm.DB, websiteID uint, metric string, hourOfWeek int, value float64) {
	baseline := GetOrCreateBaseline(db, websiteID, metric, hourOfWeek)
	UpdateBaseline(db, baseline, value)
}

// GetBaselineStats returns mean and stddev for detection, handling cold start.
func GetBaselineStats(db *gorm.DB, websiteID uint, metric string, hourOfWeek int) (mean, stddev float64) {
	baseline := GetOrCreateBaseline(db, websiteID, metric, hourOfWeek)
	return GetEffectiveBaseline(baseline)
}

// CleanupStaleBaselines removes baselines not updated within the retention period.
// This cleans up baselines for pages/goals that are no longer active.
func CleanupStaleBaselines(db *gorm.DB, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	return db.Where("updated_at < ?", cutoff).Delete(&FeedBaseline{}).Error
}

// LearnFromYesterday updates baselines for all websites based on yesterday's data.
// Should be called daily after detection runs.
func LearnFromYesterday(db *gorm.DB) error {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1).Truncate(24 * time.Hour)
	hourOfWeek := HourOfWeek(yesterday.Add(12 * time.Hour))

	var websiteIDs []uint
	if err := db.Table("websites").Pluck("id", &websiteIDs).Error; err != nil {
		return err
	}

	for _, websiteID := range websiteIDs {
		learnWebsiteBaselines(db, websiteID, yesterday, hourOfWeek)
	}

	return nil
}

func learnWebsiteBaselines(db *gorm.DB, websiteID uint, yesterday time.Time, hourOfWeek int) {
	// Learn daily visitors
	var visitors int64
	db.Table("site_stats").
		Where("website_id = ? AND DATE(hour) = DATE(?)", websiteID, yesterday).
		Select("COALESCE(SUM(visitors), 0)").Scan(&visitors)

	if visitors > 0 {
		LearnBaseline(db, websiteID, "daily_visitors", hourOfWeek, float64(visitors))
	}

	// Learn goal baselines
	type goalRow struct {
		EventName string
		Count     int64
	}
	var goals []goalRow
	db.Table("event_stats").
		Select("event_name, SUM(visitors_count) as count").
		Where("website_id = ? AND DATE(hour) = DATE(?)", websiteID, yesterday).
		Group("event_name").
		Having("count >= 1").
		Scan(&goals)

	for _, goal := range goals {
		LearnBaseline(db, websiteID, "goal_"+goal.EventName, hourOfWeek, float64(goal.Count))
	}

	// Learn page baselines (top 20)
	type pageRow struct {
		Pathname string
		Visitors int64
	}
	var pages []pageRow
	db.Table("page_stats").
		Select("pathname, SUM(visitors_count) as visitors").
		Where("website_id = ? AND DATE(hour) = DATE(?) AND pathname != '/'", websiteID, yesterday).
		Group("pathname").
		Having("visitors >= 1").
		Order("visitors DESC").
		Limit(20).
		Scan(&pages)

	for _, page := range pages {
		LearnBaseline(db, websiteID, "page_"+page.Pathname, hourOfWeek, float64(page.Visitors))
	}
}
