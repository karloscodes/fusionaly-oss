package http

import (
	"net/url"
	"time"

	"github.com/gofiber/fiber/v2"
	"log/slog"

	"fusionaly/internal/analytics"
	"fusionaly/internal/annotations"
	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/flash"
	"github.com/karloscodes/cartridge/inertia"
	"github.com/karloscodes/cartridge/structs"
	"fusionaly/internal/timeframe"
	websitesCtx "fusionaly/internal/websites"

	"gorm.io/gorm"
)

// DashboardPropsExtender is a function that can modify dashboard props before rendering.
// Used by Pro to inject additional props like insights.
type DashboardPropsExtender func(ctx *cartridge.Context, websiteID int, props map[string]interface{})

// WebsiteDashboardAction handles the dashboard for a specific website at /admin/websites/:id
func WebsiteDashboardAction(ctx *cartridge.Context) error {
	return WebsiteDashboardActionWithExtension(ctx, "Dashboard", nil)
}

// WebsiteDashboardActionWithExtension renders the dashboard with optional props extender.
// Pro uses this to render custom components and inject additional props like insights.
func WebsiteDashboardActionWithExtension(ctx *cartridge.Context, component string, extender DashboardPropsExtender) error {
	websiteId, err := ctx.ParamsInt("id")
	if err != nil {
		ctx.Logger.Error("Invalid website ID in URL", slog.Any("error", err))
		return ctx.Redirect("/admin/websites", fiber.StatusFound)
	}

	db := ctx.DB()

	website, err := websitesCtx.GetWebsiteByID(db, uint(websiteId))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.Logger.Warn("Website not found", slog.Int("websiteId", websiteId))
			flash.SetFlash(ctx.Ctx, "error", "Website not found")
			return ctx.Redirect("/admin/websites", fiber.StatusFound)
		}
		ctx.Logger.Error("Failed to get website", slog.Any("error", err))
		return ctx.Redirect("/admin/websites", fiber.StatusFound)
	}

	timeZone := ctx.Cookies("_tz")
	if timeZone != "" {
		if decodedTZ, err := url.QueryUnescape(timeZone); err == nil {
			timeZone = decodedTZ
		}
	}

	if timeZone == "" {
		return ctx.Status(fiber.StatusBadRequest).SendString("Your cookies have issues, we can't continue")
	}

	ctx.Logger.Info("Website Dashboard accessed",
		slog.Int("websiteId", websiteId),
		slog.String("domain", website.Domain),
		slog.String("timeZone", timeZone),
		slog.String("fromDate", ctx.Query("from")),
		slog.String("toDate", ctx.Query("to")))

	parser := timeframe.NewTimeFrameParser()

	firstEvent, err := analytics.GetFirstPageView(db, websiteId)
	firstEventDate := time.Now().UTC().Add(-time.Hour * 24 * 365 * 5)

	if err != nil {
		ctx.Logger.Warn("Error fetching first event date", slog.Any("error", err))
	}
	if firstEvent != nil {
		firstEventDate = firstEvent.Timestamp
	}

	timeFrame, err := parser.ParseTimeFrame(timeframe.TimeFrameParserParams{
		FromDate:            ctx.Query("from"),
		ToDate:              ctx.Query("to"),
		Tz:                  timeZone,
		AllTimeFirstEventAt: firstEventDate,
	})
	if err != nil {
		ctx.Logger.Error("Error parsing time frame", slog.Any("error", err))
		return ctx.Status(fiber.StatusBadRequest).SendString("Invalid date range")
	}

	metrics, err := analytics.FetchDashboardMetrics(db, timeFrame, websiteId, ctx.Logger)
	if err != nil {
		ctx.Logger.Error("Error fetching metrics", slog.Any("error", err))
		return ctx.Status(fiber.StatusInternalServerError).SendString("Error fetching metrics")
	}

	websitesData, err := websitesCtx.GetWebsitesForSelector(db)
	if err != nil {
		ctx.Logger.Error("Failed to fetch websites for selector", slog.Any("error", err))
		websitesData = []map[string]interface{}{}
	}

	annotationsList, err := annotations.GetAnnotationsForTimeframe(db, uint(websiteId), timeFrame.From, timeFrame.To)
	if err != nil {
		ctx.Logger.Error("Failed to fetch annotations", slog.Any("error", err))
		annotationsList = []annotations.Annotation{}
	}

	props := structs.Map(metrics)
	props["current_website_id"] = websiteId
	props["website_domain"] = website.Domain
	props["websites"] = websitesData
	props["annotations"] = annotationsList
	props["share_token"] = website.ShareToken
	props["insights"] = []interface{}{}

	props["comparison"] = inertia.Defer(func() interface{} {
		return analytics.FetchComparisonMetrics(db, timeFrame, websiteId, metrics, ctx.Logger)
	})

	queryParams := analytics.NewWebsiteScopedQueryParams(timeFrame, websiteId)
	props["user_flow"] = inertia.Defer(func() interface{} {
		flowData, err := analytics.GetUserFlowData(db, queryParams, 5)
		if err != nil {
			ctx.Logger.Error("Error fetching deferred user flow data", slog.Any("error", err))
			return []analytics.UserFlowLink{}
		}
		return flowData
	})

	if extender != nil {
		extender(ctx, websiteId, props)
	}

	return inertia.RenderPage(ctx.Ctx, component, props)
}
