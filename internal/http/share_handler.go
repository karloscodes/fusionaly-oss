package http

import (
	"fmt"
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/flash"
	"github.com/karloscodes/cartridge/inertia"
	"github.com/karloscodes/cartridge/structs"

	"fusionaly/internal/analytics"
	"fusionaly/internal/annotations"
	"fusionaly/internal/timeframe"
	"fusionaly/internal/websites"
)

// PublicDashboardAction renders a read-only public dashboard
func PublicDashboardAction(ctx *cartridge.Context) error {
	token := ctx.Params("token")
	if token == "" {
		return ctx.Status(fiber.StatusNotFound).SendString("Not found")
	}

	website, err := websites.GetWebsiteByShareToken(ctx.DB(), token)
	if err != nil {
		ctx.Logger.Debug("Public dashboard not found", slog.String("token", token))
		return ctx.Status(fiber.StatusNotFound).SendString("Dashboard not found")
	}

	// Cache public dashboards for 5 minutes - reduces DB load, CDN-friendly
	ctx.Set("Cache-Control", "public, max-age=300")

	// Parse timezone from cookie, default to UTC
	tz := ctx.Cookies("_tz")
	if tz == "" {
		tz = "UTC"
	}

	// Fixed 30-day timeframe for public dashboards
	timeFrame := timeframe.Last30Days(tz)
	websiteId := int(website.ID)
	db := ctx.DB()

	metrics, err := fetchMetrics(db, timeFrame, websiteId, ctx.Logger)
	if err != nil {
		ctx.Logger.Error("Error fetching public dashboard metrics", slog.Any("error", err))
		return ctx.Status(fiber.StatusInternalServerError).SendString("Error loading dashboard")
	}

	// Fetch annotations for this website and timeframe
	annotationsList, err := annotations.GetAnnotationsForTimeframe(db, uint(websiteId), timeFrame.From, timeFrame.To)
	if err != nil {
		ctx.Logger.Error("Failed to fetch annotations for public dashboard", slog.Any("error", err))
		annotationsList = []annotations.Annotation{}
	}

	props := structs.Map(metrics)
	props["website_domain"] = website.Domain
	props["bucket_size"] = string(timeFrame.BucketSize)
	props["is_public_view"] = true
	props["annotations"] = annotationsList

	// Add comparison data for trends
	props["comparison"] = inertia.Defer(func() interface{} {
		return fetchComparisonMetrics(db, timeFrame, websiteId, metrics, ctx.Logger)
	})

	// Add user flow data
	queryParams := analytics.NewWebsiteScopedQueryParams(timeFrame, websiteId)
	props["user_flow"] = inertia.Defer(func() interface{} {
		flowData, err := analytics.GetUserFlowData(db, queryParams, 5)
		if err != nil {
			ctx.Logger.Error("Error fetching user flow data for public dashboard", slog.Any("error", err))
			return []analytics.UserFlowLink{}
		}
		return flowData
	})

	return inertia.RenderPage(ctx.Ctx, "PublicDashboard", props)
}

// EnableShareAction enables public sharing for a website
func EnableShareAction(ctx *cartridge.Context) error {
	websiteID, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).SendString("Invalid website ID")
	}

	_, err = websites.EnableSharing(ctx.DB(), uint(websiteID))
	if err != nil {
		ctx.Logger.Error("Failed to enable sharing", slog.Any("error", err), slog.Int("websiteID", websiteID))
		flash.SetFlash(ctx.Ctx, "error", "Failed to enable sharing")
	} else {
		flash.SetFlash(ctx.Ctx, "success", "Public sharing enabled")
	}

	return ctx.Redirect(fmt.Sprintf("/admin/websites/%d/dashboard", websiteID), fiber.StatusFound)
}

// DisableShareAction disables public sharing for a website
func DisableShareAction(ctx *cartridge.Context) error {
	websiteID, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).SendString("Invalid website ID")
	}

	err = websites.DisableSharing(ctx.DB(), uint(websiteID))
	if err != nil {
		ctx.Logger.Error("Failed to disable sharing", slog.Any("error", err), slog.Int("websiteID", websiteID))
		flash.SetFlash(ctx.Ctx, "error", "Failed to disable sharing")
	} else {
		flash.SetFlash(ctx.Ctx, "success", "Public sharing disabled")
	}

	return ctx.Redirect(fmt.Sprintf("/admin/websites/%d/dashboard", websiteID), fiber.StatusFound)
}
