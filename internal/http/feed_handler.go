package http

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/inertia"
	"gorm.io/gorm"

	"fusionaly/internal/feed"
)

// HomeFeedAction renders the admin home page: your sites + the "what's new"
// activity feed + a visitor calendar. This is the product's front door.
func HomeFeedAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	// Get all website IDs (single-user instance)
	var websiteIDs []uint
	if err := db.Table("websites").Pluck("id", &websiteIDs).Error; err != nil {
		ctx.Logger.Error("Failed to get website IDs", slog.Any("error", err))
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get websites",
		})
	}

	// Get websites with event counts for display
	type websiteRow struct {
		ID         uint   `json:"id"`
		Domain     string `json:"domain"`
		CreatedAt  string `json:"created_at"`
		EventCount int64  `json:"event_count"`
	}
	var websiteRows []websiteRow
	if err := db.Table("websites").
		Select("websites.id, websites.domain, websites.created_at, COALESCE(COUNT(events.id), 0) as event_count").
		Joins("LEFT JOIN events ON events.website_id = websites.id").
		Group("websites.id").
		Order("websites.created_at DESC").
		Scan(&websiteRows).Error; err != nil {
		ctx.Logger.Error("Failed to get websites with stats", slog.Any("error", err))
		websiteRows = []websiteRow{}
	}

	websites := make([]map[string]any, 0, len(websiteRows))
	websiteMap := make(map[uint]string) // For feed enrichment
	for _, w := range websiteRows {
		websites = append(websites, map[string]any{
			"id":          w.ID,
			"domain":      w.Domain,
			"created_at":  w.CreatedAt,
			"event_count": w.EventCount,
		})
		websiteMap[w.ID] = w.Domain
	}

	// Get feed items (100 max, cleanup handles anything older than 90 days)
	items, err := feed.GetUserFeed(db, websiteIDs, 100)
	if err != nil {
		ctx.Logger.Error("Failed to get feed items", slog.Any("error", err))
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get feed items",
		})
	}

	// Enrich feed items with website domain
	type enrichedItem struct {
		feed.FeedItem
		WebsiteDomain string `json:"websiteDomain"`
	}
	enrichedItems := make([]enrichedItem, 0, len(items)) // Always non-nil
	for _, item := range items {
		enrichedItems = append(enrichedItems, enrichedItem{
			FeedItem:      item,
			WebsiteDomain: websiteMap[item.WebsiteID],
		})
	}

	// Visitor calendar (last 365 days, all websites combined).
	calendarData, totalVisitors := buildVisitorCalendar(db, websiteIDs)

	return ctx.Inertia("Home", inertia.Props{
		"feedItems":     enrichedItems,
		"websites":      websites,
		"calendarData":  calendarData,
		"totalVisitors": totalVisitors,
	})
}

// buildVisitorCalendar returns per-day visitor counts for the last 365 days
// across the given websites, plus the total. Visitors come from the daily
// site_stats rollup (the `visitors` column, grouped by day).
func buildVisitorCalendar(db *gorm.DB, websiteIDs []uint) ([]map[string]any, int64) {
	if len(websiteIDs) == 0 {
		return []map[string]any{}, 0
	}

	yearAgo := time.Now().AddDate(-1, 0, 0).Truncate(24 * time.Hour)

	type dayCount struct {
		Date  string
		Count int64
	}
	var dayCounts []dayCount
	db.Table("site_stats").
		Select("DATE(hour) as date, SUM(visitors) as count").
		Where("website_id IN ? AND hour >= ?", websiteIDs, yearAgo).
		Group("DATE(hour)").
		Order("date ASC").
		Scan(&dayCounts)

	calendarData := make([]map[string]any, len(dayCounts))
	var totalVisitors int64
	for i, dc := range dayCounts {
		calendarData[i] = map[string]any{
			"date":  dc.Date,
			"count": dc.Count,
		}
		totalVisitors += dc.Count
	}

	return calendarData, totalVisitors
}
