package http

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/inertia"
	"gorm.io/gorm"

	"fusionaly/internal/feed"
)

// HomeFeedAction renders the admin home page: your sites + the "what's new"
// activity feed + a conversion calendar. This is the product's front door.
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

	// Conversion calendar (last 365 days, all websites combined).
	// Only counts events configured as goals.
	calendarData, totalConversions, hasGoals := buildConversionCalendar(db, websiteIDs)

	return inertia.RenderPage(ctx.Ctx, "Home", inertia.Props{
		"feedItems":        enrichedItems,
		"websites":         websites,
		"calendarData":     calendarData,
		"totalConversions": totalConversions,
		"hasGoals":         hasGoals,
	})
}

// buildConversionCalendar returns per-day goal conversion counts for the last
// 365 days across the given websites, the total, and whether any goals exist.
func buildConversionCalendar(db *gorm.DB, websiteIDs []uint) ([]map[string]any, int64, bool) {
	if len(websiteIDs) == 0 {
		return nil, 0, false
	}

	// Load website_goals setting to get goal event names
	var goalsJSON string
	db.Table("settings").Where("key = ?", "website_goals").Pluck("value", &goalsJSON)

	// Parse goals JSON: {"goals": {"1": ["signup", "purchase"], "2": ["download"]}}
	var allGoalNames []string
	if goalsJSON != "" {
		var parsed struct {
			Goals map[string][]string `json:"goals"`
		}
		if err := json.Unmarshal([]byte(goalsJSON), &parsed); err == nil {
			seen := make(map[string]bool)
			for _, goals := range parsed.Goals {
				for _, goal := range goals {
					if !seen[goal] {
						seen[goal] = true
						allGoalNames = append(allGoalNames, goal)
					}
				}
			}
		}
	}

	hasGoals := len(allGoalNames) > 0
	if !hasGoals {
		return nil, 0, false
	}

	yearAgo := time.Now().AddDate(-1, 0, 0).Truncate(24 * time.Hour)

	type dayCount struct {
		Date  string
		Count int64
	}
	var dayCounts []dayCount
	db.Table("event_stats").
		Select("DATE(hour) as date, SUM(visitors_count) as count").
		Where("website_id IN ? AND hour >= ? AND event_name IN ?", websiteIDs, yearAgo, allGoalNames).
		Group("DATE(hour)").
		Order("date ASC").
		Scan(&dayCounts)

	calendarData := make([]map[string]any, len(dayCounts))
	var totalConversions int64
	for i, dc := range dayCounts {
		calendarData[i] = map[string]any{
			"date":  dc.Date,
			"count": dc.Count,
		}
		totalConversions += dc.Count
	}

	return calendarData, totalConversions, true
}
