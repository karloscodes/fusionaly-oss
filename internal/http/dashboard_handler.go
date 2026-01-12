package http

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/pariz/gountries"
	"log/slog"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gorm.io/gorm"

	"fusionaly/internal/analytics"
	"fusionaly/internal/annotations"
	"fusionaly/internal/events"
	"fusionaly/internal/pkg/async"
	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/flash"
	"github.com/karloscodes/cartridge/inertia"
	"github.com/karloscodes/cartridge/structs"
	"fusionaly/internal/timeframe"
	websitesCtx "fusionaly/internal/websites"
)

type DashboardResponse struct {
	PageViews            []TimeSeriesPoint             `json:"page_views"`
	Visitors             []TimeSeriesPoint             `json:"visitors"`
	Sessions             []TimeSeriesPoint             `json:"sessions"`
	GoalConversions      []TimeSeriesPoint             `json:"goal_conversions"`
	Revenue              []TimeSeriesPoint             `json:"revenue"`
	TopURLs              []analytics.MetricCountResult `json:"top_urls"`
	TopCountries         []analytics.MetricCountResult `json:"top_countries"`
	TopDevices           []analytics.MetricCountResult `json:"top_devices"`
	TopReferrers         []analytics.MetricCountResult `json:"top_referrers"`
	TopBrowsers          []analytics.MetricCountResult `json:"top_browsers"`
	TopCustomEvents      []analytics.MetricCountResult `json:"top_custom_events"`
	EventConversionRates map[string]float64            `json:"event_conversion_rates"`
	TopOperatingSystems  []analytics.MetricCountResult `json:"top_operating_systems"`
	EventRevenueTotals   map[string]float64            `json:"event_revenue_totals"`
	BounceRate           float64                       `json:"bounce_rate"`
	VisitsDuration       float64                       `json:"visits_duration"`
	RevenuePerVisitor    float64                       `json:"revenue_per_visitor"`
	TopEntryPages        []analytics.MetricCountResult `json:"top_entry_pages"`
	TopExitPages         []analytics.MetricCountResult `json:"top_exit_pages"`
	TopUTMMediums        []analytics.MetricCountResult `json:"top_utm_mediums"`
	TopUTMSources        []analytics.MetricCountResult `json:"top_utm_sources"`
	TopUTMCampaigns      []analytics.MetricCountResult `json:"top_utm_campaigns"`
	TopUTMTerms          []analytics.MetricCountResult `json:"top_utm_terms"`
	TopUTMContents       []analytics.MetricCountResult `json:"top_utm_contents"`
	TopRefParams         []analytics.MetricCountResult `json:"top_ref_params"`
	BucketSize           string                        `json:"bucket_size"`
	TotalVisitors        int64                         `json:"total_visitors"`
	TotalViews           int64                         `json:"total_views"`
	TotalSessions        int64                         `json:"total_sessions"`
	TotalEntryCount      int64                         `json:"total_entry_count"`
	TotalExitCount       int64                         `json:"total_exit_count"`
	TotalCustomEvents    int64                         `json:"total_custom_events"`
	RevenueMetrics       *analytics.RevenueMetrics     `json:"revenue_metrics"`
	TopRevenueEvents     []analytics.MetricCountResult `json:"top_revenue_events"`
	ConversionGoals      []string                     `json:"conversion_goals"`
	Insights             []interface{}                `json:"insights"`
	Comparison           *analytics.ComparisonMetrics `json:"comparison,omitempty"`
	UserFlow             []analytics.UserFlowLink      `json:"user_flow"`
	Annotations          []annotations.Annotation      `json:"annotations"`
}

type TimeSeriesPoint struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

func fetchMetrics(db *gorm.DB, timeFrame *timeframe.TimeFrame, websiteId int, logger *slog.Logger) (*DashboardResponse, error) {
	// Create a WebsiteScopedQueryParams to pass to all metrics functions
	queryParams := analytics.NewWebsiteScopedQueryParams(timeFrame, websiteId)

	// Conversion goals - Pro feature
	conversionGoals := []string{}

	// Consolidated async tasks including both current and comparison metrics
	tasks := []async.Task{
		{
			Name: "pageViews",
			Execute: func() (interface{}, error) {
				stats, err := analytics.AggregatedPageViewsInTimeFrame(db, queryParams)
				if err != nil {
					logger.Error("Error fetching page views", slog.Any("error", err))
					return []timeframe.DateStat{}, err
				}
				return convertToTimeSeries(stats), nil
			},
		},
		{
			Name: "visitors",
			Execute: func() (interface{}, error) {
				stats, err := analytics.AggregatedVisitorsInTimeFrame(db, queryParams)
				if err != nil {
					logger.Error("Error fetching visitors", slog.Any("error", err))
					return []timeframe.DateStat{}, err
				}
				return convertToTimeSeries(stats), nil
			},
		},
		{
			Name: "sessions",
			Execute: func() (interface{}, error) {
				stats, err := analytics.AggregatedSessionsInTimeFrame(db, queryParams)
				if err != nil {
					logger.Error("Error fetching sessions", slog.Any("error", err))
					return []timeframe.DateStat{}, err
				}
				return convertToTimeSeries(stats), nil
			},
		},
		{
			Name: "revenue",
			Execute: func() (interface{}, error) {
				stats, err := analytics.AggregatedRevenueInTimeFrame(db, queryParams)
				if err != nil {
					logger.Error("Error fetching revenue", slog.Any("error", err))
					return []timeframe.DateStat{}, err
				}
				return convertToTimeSeries(stats), nil
			},
		},
		{
			Name: "topUrls",
			Execute: func() (interface{}, error) {
				return analytics.GetTopURLsInTimeFrame(db, queryParams)
			},
		},
		{
			Name: "topCountries",
			Execute: func() (interface{}, error) {
				stats, err := analytics.GetTopCountriesInTimeFrame(db, queryParams)
				if err != nil {
					return nil, err
				}
				return convertCountryStats(stats), nil
			},
		},
		{
			Name: "topDevices",
			Execute: func() (interface{}, error) {
				stats, err := analytics.GetTopDeviceTypesInTimeFrame(db, queryParams)
				if err != nil {
					return nil, err
				}
				return convertDeviceStats(stats), nil
			},
		},
		{
			Name: "topReferrers",
			Execute: func() (interface{}, error) {
				stats, err := analytics.GetTopReferrersInTimeFrame(db, queryParams)
				if err != nil {
					return nil, err
				}
				return convertReferrerStats(stats), nil
			},
		},
		{
			Name: "topBrowsers",
			Execute: func() (interface{}, error) {
				stats, err := analytics.GetTopBrowsersInTimeFrame(db, queryParams)
				if err != nil {
					return nil, err
				}
				return convertBrowserStats(stats), nil
			},
		},
		{
			Name: "topCustomEvents",
			Execute: func() (interface{}, error) {
				return analytics.GetTopCustomEventsInTimeFrame(db, queryParams)
			},
		},
		{
			Name: "eventRevenueTotals",
			Execute: func() (interface{}, error) {
				return analytics.GetEventRevenueTotals(db, queryParams)
			},
		},
		{
			Name: "topOperatingSystems",
			Execute: func() (interface{}, error) {
				stats, err := analytics.GetTopOsInTimeFrame(db, queryParams)
				if err != nil {
					return nil, err
				}
				return convertOSStats(stats), nil
			},
		},
		{
			Name: "bounceRate",
			Execute: func() (interface{}, error) {
				return analytics.GetBounceRateInTimeFrame(db, queryParams)
			},
		},
		{
			Name: "visitsDuration",
			Execute: func() (interface{}, error) {
				return analytics.GetVisitDurationInTimeFrame(db, queryParams)
			},
		},
		{
			Name: "revenuePerVisitor",
			Execute: func() (interface{}, error) {
				return analytics.GetRevenuePerVisitor(db, queryParams)
			},
		},
		{
			Name: "topEntryPages",
			Execute: func() (interface{}, error) {
				return analytics.GetTopEntryPagesInTimeFrame(db, queryParams)
			},
		},
		{
			Name: "topExitPages",
			Execute: func() (interface{}, error) {
				return analytics.GetTopExitPagesInTimeFrame(db, queryParams)
			},
		},
		{
			Name: "topUTMMediums",
			Execute: func() (interface{}, error) {
				return analytics.GetTopUTMMediumsInTimeFrame(db, queryParams)
			},
		},
		{
			Name: "topUTMSources",
			Execute: func() (interface{}, error) {
				return analytics.GetTopUTMSourcesInTimeFrame(db, queryParams)
			},
		},
		{
			Name: "topUTMCampaigns",
			Execute: func() (interface{}, error) {
				return analytics.GetTopUTMCampaignsInTimeFrame(db, queryParams)
			},
		},
		{
			Name: "topUTMTerms",
			Execute: func() (interface{}, error) {
				return analytics.GetTopUTMTermsInTimeFrame(db, queryParams)
			},
		},
		{
			Name: "topUTMContents",
			Execute: func() (interface{}, error) {
				return analytics.GetTopUTMContentsInTimeFrame(db, queryParams)
			},
		},
		{
			Name: "topRefParams",
			Execute: func() (interface{}, error) {
				return analytics.GetTopQueryParamValuesInTimeFrame(db, queryParams, "ref")
			},
		},
		{
			Name: "totalVisitors",
			Execute: func() (interface{}, error) {
				return analytics.GetTotalVisitorsInTimeFrame(db, queryParams)
			},
		},
		{
			Name: "totalViews",
			Execute: func() (interface{}, error) {
				return analytics.GetTotalPageViewsInTimeFrame(db, queryParams)
			},
		},
		{
			Name: "totalSessions",
			Execute: func() (interface{}, error) {
				return analytics.GetTotalSessionsInTimeFrame(db, queryParams)
			},
		},
		{
			Name: "totalEntryCount",
			Execute: func() (interface{}, error) {
				return analytics.GetTotalEntryCountInTimeFrame(db, queryParams)
			},
		},
		{
			Name: "totalExitCount",
			Execute: func() (interface{}, error) {
				return analytics.GetTotalExitCountInTimeFrame(db, queryParams)
			},
		},
		{
			Name: "totalCustomEvents",
			Execute: func() (interface{}, error) {
				return analytics.GetTotalCustomEventsInTimeFrame(db, queryParams)
			},
		},
		{
			Name: "revenueMetrics",
			Execute: func() (interface{}, error) {
				return analytics.GetRevenueMetrics(db, queryParams)
			},
		},
		{
			Name: "topRevenueEvents",
			Execute: func() (interface{}, error) {
				return analytics.GetTopRevenueEvents(db, queryParams)
			},
		},
		{
			Name: "conversionGoals",
			Execute: func() (interface{}, error) {
				return conversionGoals, nil
			},
		},
	}

	pool := async.NewPool(12)
	results := pool.Execute(context.Background(), tasks)

	// Check for errors first
	for name, result := range results {
		if result.Err != nil {
			return nil, fmt.Errorf("error fetching %s: %w", name, result.Err)
		}
	}

	// Build response directly
	resp := &DashboardResponse{
		PageViews:            results["pageViews"].Data.([]TimeSeriesPoint),
		Visitors:             results["visitors"].Data.([]TimeSeriesPoint),
		Sessions:             results["sessions"].Data.([]TimeSeriesPoint),
		GoalConversions:      results["revenue"].Data.([]TimeSeriesPoint),
		Revenue:              results["revenue"].Data.([]TimeSeriesPoint),
		TopURLs:              ensureNonNil(getMetricResultsOrEmpty(results, "topUrls")),
		TopCountries:         ensureNonNil(getMetricResultsOrEmpty(results, "topCountries")),
		TopDevices:           ensureNonNil(getMetricResultsOrEmpty(results, "topDevices")),
		TopReferrers:         ensureNonNil(getMetricResultsOrEmpty(results, "topReferrers")),
		TopBrowsers:          ensureNonNil(getMetricResultsOrEmpty(results, "topBrowsers")),
		TopCustomEvents:      ensureNonNil(getMetricResultsOrEmpty(results, "topCustomEvents")),
		EventConversionRates: map[string]float64{},
		TopOperatingSystems:  ensureNonNil(getMetricResultsOrEmpty(results, "topOperatingSystems")),
		EventRevenueTotals:   getRevenueTotalsOrEmpty(results, "eventRevenueTotals"),
		BounceRate:           results["bounceRate"].Data.(float64),
		VisitsDuration:       results["visitsDuration"].Data.(float64),
		RevenuePerVisitor:    results["revenuePerVisitor"].Data.(float64),
		TopEntryPages:        ensureNonNil(getMetricResultsOrEmpty(results, "topEntryPages")),
		TopExitPages:         ensureNonNil(getMetricResultsOrEmpty(results, "topExitPages")),
		TopUTMMediums:        ensureNonNil(getMetricResultsOrEmpty(results, "topUTMMediums")),
		TopUTMSources:        ensureNonNil(getMetricResultsOrEmpty(results, "topUTMSources")),
		TopUTMCampaigns:      ensureNonNil(getMetricResultsOrEmpty(results, "topUTMCampaigns")),
		TopUTMTerms:          ensureNonNil(getMetricResultsOrEmpty(results, "topUTMTerms")),
		TopUTMContents:       ensureNonNil(getMetricResultsOrEmpty(results, "topUTMContents")),
		TopRefParams:         ensureNonNil(getMetricResultsOrEmpty(results, "topRefParams")),
		BucketSize:           string(timeFrame.BucketSize),
		TotalVisitors:        results["totalVisitors"].Data.(int64),
		TotalViews:           results["totalViews"].Data.(int64),
		TotalSessions:        results["totalSessions"].Data.(int64),
		TotalEntryCount:      results["totalEntryCount"].Data.(int64),
		TotalExitCount:       results["totalExitCount"].Data.(int64),
		TotalCustomEvents:    results["totalCustomEvents"].Data.(int64),
		RevenueMetrics:       results["revenueMetrics"].Data.(*analytics.RevenueMetrics),
		TopRevenueEvents:     ensureNonNil(getMetricResultsOrEmpty(results, "topRevenueEvents")),
		ConversionGoals:      results["conversionGoals"].Data.([]string),
		Insights:             []interface{}{}, // Pro feature - AI insights
		UserFlow:             []analytics.UserFlowLink{}, // Deferred - loaded separately
	}

	// Comparison is deferred - set to nil initially
	resp.Comparison = nil
	resp.EventConversionRates = buildEventConversionRates(resp)

	return resp, nil
}

// fetchComparisonMetrics fetches comparison period metrics for deferred loading
func fetchComparisonMetrics(db *gorm.DB, timeFrame *timeframe.TimeFrame, websiteId int, currentMetrics *DashboardResponse, logger *slog.Logger) *analytics.ComparisonMetrics {
	// Calculate comparison period (same duration, ending when current period starts)
	duration := timeFrame.To.Sub(timeFrame.From)
	comparisonFrom := timeFrame.From.Add(-duration)
	comparisonTo := timeFrame.From

	comparisonTimeFrame := &timeframe.TimeFrame{
		From:       comparisonFrom,
		To:         comparisonTo,
		BucketSize: timeFrame.BucketSize,
	}
	comparisonQueryParams := analytics.NewWebsiteScopedQueryParams(comparisonTimeFrame, websiteId)

	// Fetch comparison metrics in parallel
	tasks := []async.Task{
		{
			Name: "comparisonVisitors",
			Execute: func() (interface{}, error) {
				return analytics.GetTotalVisitorsInTimeFrame(db, comparisonQueryParams)
			},
		},
		{
			Name: "comparisonViews",
			Execute: func() (interface{}, error) {
				return analytics.GetTotalPageViewsInTimeFrame(db, comparisonQueryParams)
			},
		},
		{
			Name: "comparisonSessions",
			Execute: func() (interface{}, error) {
				return analytics.GetTotalSessionsInTimeFrame(db, comparisonQueryParams)
			},
		},
		{
			Name: "comparisonBounceRate",
			Execute: func() (interface{}, error) {
				return analytics.GetBounceRateInTimeFrame(db, comparisonQueryParams)
			},
		},
		{
			Name: "comparisonVisitsDuration",
			Execute: func() (interface{}, error) {
				return analytics.GetVisitDurationInTimeFrame(db, comparisonQueryParams)
			},
		},
		{
			Name: "comparisonRevenueMetrics",
			Execute: func() (interface{}, error) {
				return analytics.GetRevenueMetrics(db, comparisonQueryParams)
			},
		},
	}

	pool := async.NewPool(6)
	results := pool.Execute(context.Background(), tasks)

	// Build comparison data
	comparisonData := analytics.ComparisonData{
		CurrentVisitors:   currentMetrics.TotalVisitors,
		CurrentViews:      currentMetrics.TotalViews,
		CurrentSessions:   currentMetrics.TotalSessions,
		CurrentBounceRate: currentMetrics.BounceRate,
		CurrentAvgTime:    currentMetrics.VisitsDuration,
	}

	if prevVisitors, ok := results["comparisonVisitors"].Data.(int64); ok {
		comparisonData.PreviousVisitors = prevVisitors
	}
	if prevViews, ok := results["comparisonViews"].Data.(int64); ok {
		comparisonData.PreviousViews = prevViews
	}
	if prevSessions, ok := results["comparisonSessions"].Data.(int64); ok {
		comparisonData.PreviousSessions = prevSessions
	}
	if prevBounceRate, ok := results["comparisonBounceRate"].Data.(float64); ok {
		comparisonData.PreviousBounceRate = prevBounceRate
	}
	if prevAvgTime, ok := results["comparisonVisitsDuration"].Data.(float64); ok {
		comparisonData.PreviousAvgTime = prevAvgTime
	}
	if currentMetrics.RevenueMetrics != nil {
		comparisonData.CurrentRevenue = currentMetrics.RevenueMetrics.TotalRevenue
	}
	if prevRevenueMetrics, ok := results["comparisonRevenueMetrics"].Data.(*analytics.RevenueMetrics); ok && prevRevenueMetrics != nil {
		comparisonData.PreviousRevenue = prevRevenueMetrics.TotalRevenue
	}

	return analytics.CalculateComparisonMetrics(comparisonData)
}

func buildEventConversionRates(resp *DashboardResponse) map[string]float64 {
	rates := make(map[string]float64, len(resp.TopCustomEvents))
	totalVisitors := resp.TotalVisitors
	if totalVisitors <= 0 {
		return rates
	}

	denominator := float64(totalVisitors)
	for _, event := range resp.TopCustomEvents {
		if event.Count <= 0 {
			continue
		}
		rates[event.Name] = float64(event.Count) / denominator * 100
	}

	return rates
}

func convertToTimeSeries(stats []timeframe.DateStat) []TimeSeriesPoint {
	result := make([]TimeSeriesPoint, len(stats))
	for i, stat := range stats {
		result[i] = TimeSeriesPoint{
			Date:  stat.Date,
			Count: int(stat.Count), // Convert int64 to int for response
		}
	}
	return result
}

func convertCountryStats(items []analytics.MetricCountResult) []analytics.MetricCountResult {
	caser := cases.Upper(language.AmericanEnglish)
	countries := gountries.New()

	if len(items) == 0 {
		return []analytics.MetricCountResult{}
	}

	result := make([]analytics.MetricCountResult, len(items))
	for i, item := range items {
		if item.Name == events.UnknownCountry {
			item.Name = "Unknown"
			result[i] = analytics.MetricCountResult{
				Name:  item.Name,
				Count: item.Count,
			}
		} else {
			countryName, err := countries.FindCountryByAlpha(item.Name)
			if err != nil {
				result[i] = analytics.MetricCountResult{
					Name:  caser.String(item.Name),
					Count: item.Count,
				}
			} else {
				result[i] = analytics.MetricCountResult{
					Name:  countryName.Name.Common,
					Count: item.Count,
				}
			}
		}
	}
	return result
}

func convertDeviceStats(items []analytics.MetricCountResult) []analytics.MetricCountResult {
	caser := cases.Title(language.AmericanEnglish)

	if len(items) == 0 {
		return []analytics.MetricCountResult{}
	}

	result := make([]analytics.MetricCountResult, len(items))
	for i, item := range items {
		name := item.Name
		if name == events.UnknownDevice {
			name = "Unknown"
		}
		result[i] = analytics.MetricCountResult{
			Name:  caser.String(name),
			Count: item.Count,
		}
	}
	return result
}

// convertReferrerStats converts referrer statistics, handling internal constants
func convertReferrerStats(items []analytics.MetricCountResult) []analytics.MetricCountResult {
	if len(items) == 0 {
		return []analytics.MetricCountResult{}
	}

	result := make([]analytics.MetricCountResult, len(items))
	for i, item := range items {
		name := item.Name
		// Convert the internal constant to a human-readable format
		if name == events.DirectOrUnknownReferrer {
			name = "Direct / Unknown"
		}
		result[i] = analytics.MetricCountResult{
			Name:  name,
			Count: item.Count,
		}
	}
	return result
}

func convertOSStats(items []analytics.MetricCountResult) []analytics.MetricCountResult {
	caser := cases.Title(language.AmericanEnglish)

	if len(items) == 0 {
		return []analytics.MetricCountResult{}
	}

	result := make([]analytics.MetricCountResult, len(items))
	for i, item := range items {
		name := item.Name

		if name == events.UnknownOS {
			name = "Unknown"
		} else {
			// Special handling for iOS and iPadOS to maintain correct capitalization
			nameLower := strings.ToLower(strings.TrimSpace(name))

			switch nameLower {
			case "ios", "iphone os":
				name = "iOS"
			case "ipados":
				name = "iPadOS"
			case "macos", "mac os", "mac os x", "darwin":
				name = "macOS"
			default:
				name = caser.String(name)
			}
		}
		result[i] = analytics.MetricCountResult{
			Name:  name,
			Count: item.Count,
		}
	}
	return result
}

func convertBrowserStats(items []analytics.MetricCountResult) []analytics.MetricCountResult {
	caser := cases.Title(language.AmericanEnglish)

	if len(items) == 0 {
		return []analytics.MetricCountResult{}
	}

	result := make([]analytics.MetricCountResult, len(items))
	for i, item := range items {
		name := item.Name
		if name == events.UnknownBrowser {
			name = "Unknown"
		}
		result[i] = analytics.MetricCountResult{
			Name:  caser.String(name),
			Count: item.Count,
		}
	}
	return result
}

func getMetricResultsOrEmpty(results map[string]async.Result, name string) []analytics.MetricCountResult {
	if result, exists := results[name]; exists {
		if result.Data != nil {
			return result.Data.([]analytics.MetricCountResult)
		}
	}
	return []analytics.MetricCountResult{}
}

func getRevenueTotalsOrEmpty(results map[string]async.Result, name string) map[string]float64 {
	if result, exists := results[name]; exists {
		if result.Data != nil {
			if totals, ok := result.Data.(map[string]float64); ok {
				return totals
			}
		}
	}
	return map[string]float64{}
}

func ensureNonNil(items []analytics.MetricCountResult) []analytics.MetricCountResult {
	if items == nil {
		return []analytics.MetricCountResult{}
	}
	return items
}

// WebsiteDashboardAction handles the dashboard for a specific website at /admin/websites/:id
func WebsiteDashboardAction(ctx *cartridge.Context) error {
	// Get website ID from URL params
	websiteId, err := ctx.ParamsInt("id")
	if err != nil {
		ctx.Logger.Error("Invalid website ID in URL", slog.Any("error", err))
		return ctx.Redirect("/admin/websites", fiber.StatusFound)
	}

	db := ctx.DB()

	// Get website to verify it exists and get domain
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

	// Parse timezone from cookie
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

	// Create time frame parser
	parser := timeframe.NewTimeFrameParser()

	firstEvent, err := analytics.GetFirstPageView(db, websiteId)
	firstEventDate := time.Now().UTC().Add(-time.Hour * 24 * 365 * 5)

	if err != nil {
		ctx.Logger.Warn("Error fetching first event date", slog.Any("error", err))
	}
	if firstEvent != nil {
		firstEventDate = firstEvent.Timestamp
	}

	// Parse time frame with parameters from request
	params := timeframe.TimeFrameParserParams{
		FromDate:            ctx.Query("from"),
		ToDate:              ctx.Query("to"),
		Tz:                  timeZone,
		AllTimeFirstEventAt: firstEventDate,
	}

	timeFrame, err := parser.ParseTimeFrame(params)
	if err != nil {
		ctx.Logger.Error("Error parsing time frame", slog.Any("error", err))
		return ctx.Status(fiber.StatusBadRequest).SendString("Invalid date range")
	}

	// Fetch metrics for the determined time frame and website
	metrics, err := fetchMetrics(db, timeFrame, websiteId, ctx.Logger)
	if err != nil {
		ctx.Logger.Error("Error fetching metrics", slog.Any("error", err))
		return ctx.Status(fiber.StatusInternalServerError).SendString("Error fetching metrics")
	}

	// Fetch websites for the selector
	websitesData, err := websitesCtx.GetWebsitesForSelector(db)
	if err != nil {
		ctx.Logger.Error("Failed to fetch websites for selector", slog.Any("error", err))
		websitesData = []map[string]interface{}{} // Set to empty array on error
	}

	// Fetch annotations for this website and timeframe
	annotationsList, err := annotations.GetAnnotationsForTimeframe(db, uint(websiteId), timeFrame.From, timeFrame.To)
	if err != nil {
		ctx.Logger.Error("Failed to fetch annotations", slog.Any("error", err))
		annotationsList = []annotations.Annotation{} // Set to empty array on error
	}

	// Prepare props with metrics data (csrfToken and flash auto-injected by cartridgeinertia.RenderPage)
	props := structs.Map(metrics)
	props["current_website_id"] = websiteId
	props["website_domain"] = website.Domain
	props["websites"] = websitesData
	props["annotations"] = annotationsList

	// Insights are a Pro feature - return empty array for OSS
	props["insights"] = []interface{}{}

	props["comparison"] = inertia.Defer(func() interface{} {
		return fetchComparisonMetrics(db, timeFrame, websiteId, metrics, ctx.Logger)
	})

	// Create query params for user flow (needs to be in closure scope)
	queryParams := analytics.NewWebsiteScopedQueryParams(timeFrame, websiteId)
	props["user_flow"] = inertia.Defer(func() interface{} {
		flowData, err := analytics.GetUserFlowData(db, queryParams, 5)
		if err != nil {
			ctx.Logger.Error("Error fetching deferred user flow data", slog.Any("error", err))
			return []analytics.UserFlowLink{}
		}
		return flowData
	})

	return inertia.RenderPage(ctx.Ctx, "Dashboard", props)
}

