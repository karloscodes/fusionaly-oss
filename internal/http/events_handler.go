package http

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"log/slog"
	"gorm.io/gorm"

	"fusionaly/internal/events"
	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/flash"
	"github.com/karloscodes/cartridge/inertia"
	"github.com/karloscodes/cartridge/structs"
	"fusionaly/internal/visitors"
	"fusionaly/internal/websites"
)

type PaginationData struct {
	CurrentPage int   `json:"current_page"`
	TotalPages  int   `json:"total_pages"`
	TotalItems  int64 `json:"total_items"`
	PerPage     int   `json:"per_page"`
}

type Event struct {
	Timestamp      time.Time        `json:"timestamp"`
	RawURL         string           `json:"raw_url"`
	Referrer       string           `json:"referrer"`
	EventType      events.EventType `json:"event_type"`
	User           string           `json:"user"`
	CustomEventKey string           `json:"custom_event_key,omitempty"`
}

type EventsResponse struct {
	Events     []Event        `json:"events"`
	Pagination PaginationData `json:"pagination"`
}

func EventsIndexAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	// Get website ID from context (set by middleware)
	websiteId, ok := ctx.Locals("website_id").(int)
	if !ok {
		// Handle case where website_id is not set - check if any websites exist
		ctx.Logger.Warn("Website ID not found or invalid in context, checking for existing websites")

		// Get first website if no website_id is set
		firstWebsite, err := websites.GetFirstWebsite(db)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				ctx.Logger.Info("No websites found in database - redirecting to website creation")
				// Redirect to website creation page when no websites exist
				return ctx.Redirect("/admin/websites/new", fiber.StatusFound)
			}
			ctx.Logger.Error("Failed to get first website", slog.Any("error", err))
			flash.SetFlash(ctx.Ctx, "error", "Error getting website data")
			return ctx.Redirect("/admin/websites", fiber.StatusFound)
		}

		ctx.Logger.Info("Found first website", slog.Uint64("id", uint64(firstWebsite.ID)), slog.String("domain", firstWebsite.Domain))
		websiteId = int(firstWebsite.ID)
	}

	// Get pagination parameters
	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	limit := 50 // Events per page
	offset := (page - 1) * limit

	// Get filter parameters
	urlFilter := ctx.Query("url", "")
	referrerFilter := ctx.Query("referrer", "")
	userFilter := ctx.Query("user", "")
	typeFilter := ctx.Query("type", "")
	customEventNameFilter := ctx.Query("event_key", "")
	rangeFilter := ctx.Query("range", "last_7_days")

	// Calculate date range from range filter
	fromDate, toDate := calculateDateRange(rangeFilter)

	// Get filtered events using events context
	result, err := events.GetFilteredEvents(db, events.EventFilters{
		WebsiteID:             uint(websiteId),
		FromDate:              fromDate,
		ToDate:                toDate,
		URLFilter:             urlFilter,
		ReferrerFilter:        referrerFilter,
		UserFilter:            userFilter,
		TypeFilter:            typeFilter,
		CustomEventNameFilter: customEventNameFilter,
		Limit:                 limit,
		Offset:                offset,
	})
	if err != nil {
		ctx.Logger.Error("Failed to fetch events", slog.Any("error", err))
		flash.SetFlash(ctx.Ctx, "error", "Failed to fetch events")
		return ctx.Redirect("/admin/websites", fiber.StatusFound)
	}

	mappedEvents := make([]Event, len(result.Events))
	for i, event := range result.Events {
		mappedEvents[i] = Event{
			Timestamp:      event.Timestamp,
			RawURL:         event.Hostname + event.Pathname,
			Referrer:       event.ReferrerHostname + event.ReferrerPathname,
			EventType:      event.EventType,
			User:           visitors.VisitorAlias(event.UserSignature),
			CustomEventKey: event.CustomEventName,
		}
	}

	totalPages := (int(result.Total) + limit - 1) / limit

	response := EventsResponse{
		Events: mappedEvents,
		Pagination: PaginationData{
			CurrentPage: page,
			TotalPages:  totalPages,
			TotalItems:  result.Total,
			PerPage:     limit,
		},
	}

	// Prepare props with response data (csrfToken and flash auto-injected by cartridgeinertia.RenderPage)
	props := structs.Map(response)
	props["current_website_id"] = websiteId

	return inertia.RenderPage(ctx.Ctx, "Events", props)
}

// calculateDateRange calculates from and to dates based on range filter
func calculateDateRange(rangeFilter string) (time.Time, time.Time) {
	now := time.Now()
	var fromDate, toDate time.Time

	switch rangeFilter {
	case "today":
		fromDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		toDate = now
	case "yesterday":
		yesterday := now.AddDate(0, 0, -1)
		fromDate = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
		toDate = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 23, 59, 59, 999999999, yesterday.Location())
	case "last_7_days":
		fromDate = now.AddDate(0, 0, -7)
		toDate = now
	case "last_30_days":
		fromDate = now.AddDate(0, 0, -30)
		toDate = now
	case "last_90_days":
		fromDate = now.AddDate(0, 0, -90)
		toDate = now
	case "this_month":
		fromDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		toDate = now
	case "last_month":
		firstOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		firstOfLastMonth := firstOfThisMonth.AddDate(0, -1, 0)
		lastOfLastMonth := firstOfThisMonth.AddDate(0, 0, -1)
		fromDate = firstOfLastMonth
		toDate = time.Date(lastOfLastMonth.Year(), lastOfLastMonth.Month(), lastOfLastMonth.Day(), 23, 59, 59, 999999999, lastOfLastMonth.Location())
	default:
		// Default to last 7 days
		fromDate = now.AddDate(0, 0, -7)
		toDate = now
	}

	return fromDate, toDate
}

// WebsiteEventsAction handles the events page for a specific website at /admin/websites/:id/events
func WebsiteEventsAction(ctx *cartridge.Context) error {
	// Get website ID from URL params
	websiteId, err := ctx.ParamsInt("id")
	if err != nil {
		ctx.Logger.Error("Invalid website ID in URL", slog.Any("error", err))
		return ctx.Redirect("/admin/websites", fiber.StatusFound)
	}

	db := ctx.DB()

	// Get website to verify it exists and get domain
	website, err := websites.GetWebsiteByID(db, uint(websiteId))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.Logger.Warn("Website not found", slog.Int("websiteId", websiteId))
			flash.SetFlash(ctx.Ctx, "error", "Website not found")
			return ctx.Redirect("/admin/websites", fiber.StatusFound)
		}
		ctx.Logger.Error("Failed to get website", slog.Any("error", err))
		return ctx.Redirect("/admin/websites", fiber.StatusFound)
	}

	// Get pagination parameters
	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	limit := 50 // Events per page
	offset := (page - 1) * limit

	// Get filter parameters
	urlFilter := ctx.Query("url", "")
	referrerFilter := ctx.Query("referrer", "")
	userFilter := ctx.Query("user", "")
	typeFilter := ctx.Query("type", "")
	customEventNameFilter := ctx.Query("event_key", "")
	rangeFilter := ctx.Query("range", "last_7_days")

	// Calculate date range from range filter
	fromDate, toDate := calculateDateRange(rangeFilter)

	// Get filtered events using events context
	result, err := events.GetFilteredEvents(db, events.EventFilters{
		WebsiteID:             uint(websiteId),
		FromDate:              fromDate,
		ToDate:                toDate,
		URLFilter:             urlFilter,
		ReferrerFilter:        referrerFilter,
		UserFilter:            userFilter,
		TypeFilter:            typeFilter,
		CustomEventNameFilter: customEventNameFilter,
		Limit:                 limit,
		Offset:                offset,
	})
	if err != nil {
		ctx.Logger.Error("Failed to fetch events", slog.Any("error", err))
		flash.SetFlash(ctx.Ctx, "error", "Failed to fetch events")
		return ctx.Redirect("/admin/websites", fiber.StatusFound)
	}

	mappedEvents := make([]Event, len(result.Events))
	for i, event := range result.Events {
		mappedEvents[i] = Event{
			Timestamp:      event.Timestamp,
			RawURL:         event.Hostname + event.Pathname,
			Referrer:       event.ReferrerHostname + event.ReferrerPathname,
			EventType:      event.EventType,
			User:           visitors.VisitorAlias(event.UserSignature),
			CustomEventKey: event.CustomEventName,
		}
	}

	totalPages := (int(result.Total) + limit - 1) / limit

	response := EventsResponse{
		Events: mappedEvents,
		Pagination: PaginationData{
			CurrentPage: page,
			TotalPages:  totalPages,
			TotalItems:  result.Total,
			PerPage:     limit,
		},
	}

	// Fetch websites for the selector
	websitesData, err := websites.GetWebsitesForSelector(db)
	if err != nil {
		ctx.Logger.Error("Failed to fetch websites for selector", slog.Any("error", err))
		websitesData = []map[string]interface{}{} // Set to empty array on error
	}

	// Prepare props with response data (csrfToken and flash auto-injected by cartridgeinertia.RenderPage)
	props := structs.Map(response)
	props["current_website_id"] = websiteId
	props["website_domain"] = website.Domain
	props["websites"] = websitesData

	return inertia.RenderPage(ctx.Ctx, "Events", props)
}
