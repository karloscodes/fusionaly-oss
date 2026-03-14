package analytics

import (
	"context"
	"fmt"
	"log/slog"

	"fusionaly/internal/pkg/async"
	"fusionaly/internal/settings"
	"fusionaly/internal/timeframe"

	"gorm.io/gorm"
)

// DashboardMetrics contains all metrics displayed on the analytics dashboard.
type DashboardMetrics struct {
	PageViews            []TimeSeriesPoint    `json:"page_views"`
	Visitors             []TimeSeriesPoint    `json:"visitors"`
	Sessions             []TimeSeriesPoint    `json:"sessions"`
	GoalConversions      []TimeSeriesPoint    `json:"goal_conversions"`
	Revenue              []TimeSeriesPoint    `json:"revenue"`
	TopURLs              []MetricCountResult  `json:"top_urls"`
	TopCountries         []MetricCountResult  `json:"top_countries"`
	TopDevices           []MetricCountResult  `json:"top_devices"`
	TopReferrers         []MetricCountResult  `json:"top_referrers"`
	TopBrowsers          []MetricCountResult  `json:"top_browsers"`
	TopCustomEvents      []MetricCountResult  `json:"top_custom_events"`
	EventConversionRates map[string]float64   `json:"event_conversion_rates"`
	TopOperatingSystems  []MetricCountResult  `json:"top_operating_systems"`
	EventRevenueTotals   map[string]float64   `json:"event_revenue_totals"`
	BounceRate           float64              `json:"bounce_rate"`
	VisitsDuration       float64              `json:"visits_duration"`
	RevenuePerVisitor    float64              `json:"revenue_per_visitor"`
	TopEntryPages        []MetricCountResult  `json:"top_entry_pages"`
	TopExitPages         []MetricCountResult  `json:"top_exit_pages"`
	TopUTMMediums        []MetricCountResult  `json:"top_utm_mediums"`
	TopUTMSources        []MetricCountResult  `json:"top_utm_sources"`
	TopUTMCampaigns      []MetricCountResult  `json:"top_utm_campaigns"`
	TopUTMTerms          []MetricCountResult  `json:"top_utm_terms"`
	TopUTMContents       []MetricCountResult  `json:"top_utm_contents"`
	TopRefParams         []MetricCountResult  `json:"top_ref_params"`
	BucketSize           string               `json:"bucket_size"`
	TotalVisitors        int64                `json:"total_visitors"`
	TotalViews           int64                `json:"total_views"`
	TotalSessions        int64                `json:"total_sessions"`
	TotalEntryCount      int64                `json:"total_entry_count"`
	TotalExitCount       int64                `json:"total_exit_count"`
	TotalCustomEvents    int64                `json:"total_custom_events"`
	RevenueMetrics       *RevenueMetrics      `json:"revenue_metrics"`
	TopRevenueEvents     []MetricCountResult  `json:"top_revenue_events"`
	ConversionGoals      []string             `json:"conversion_goals"`
	Insights             []interface{}        `json:"insights"`
	Comparison           *ComparisonMetrics   `json:"comparison,omitempty"`
	UserFlow             []UserFlowLink       `json:"user_flow"`
}

// TimeSeriesPoint represents a single data point in a time series chart.
type TimeSeriesPoint struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// FetchDashboardMetrics loads all dashboard metrics in parallel for the given timeframe and website.
func FetchDashboardMetrics(db *gorm.DB, tf *timeframe.TimeFrame, websiteId int, logger *slog.Logger) (*DashboardMetrics, error) {
	queryParams := NewWebsiteScopedQueryParams(tf, websiteId)

	conversionGoals, err := settings.GetWebsiteGoals(db, uint(websiteId))
	if err != nil {
		logger.Error("Error fetching conversion goals", slog.Any("error", err))
		conversionGoals = []string{}
	}

	tasks := []async.Task{
		timeSeriesTask("pageViews", func() ([]timeframe.DateStat, error) { return AggregatedPageViewsInTimeFrame(db, queryParams) }, logger),
		timeSeriesTask("visitors", func() ([]timeframe.DateStat, error) { return AggregatedVisitorsInTimeFrame(db, queryParams) }, logger),
		timeSeriesTask("sessions", func() ([]timeframe.DateStat, error) { return AggregatedSessionsInTimeFrame(db, queryParams) }, logger),
		timeSeriesTask("revenue", func() ([]timeframe.DateStat, error) { return AggregatedRevenueInTimeFrame(db, queryParams) }, logger),
		formattedMetricTask("topCountries", func() ([]MetricCountResult, error) { return GetTopCountriesInTimeFrame(db, queryParams) }, FormatCountryStats),
		formattedMetricTask("topDevices", func() ([]MetricCountResult, error) { return GetTopDeviceTypesInTimeFrame(db, queryParams) }, FormatDeviceStats),
		formattedMetricTask("topReferrers", func() ([]MetricCountResult, error) { return GetTopReferrersInTimeFrame(db, queryParams) }, FormatReferrerStats),
		formattedMetricTask("topBrowsers", func() ([]MetricCountResult, error) { return GetTopBrowsersInTimeFrame(db, queryParams) }, FormatBrowserStats),
		formattedMetricTask("topOperatingSystems", func() ([]MetricCountResult, error) { return GetTopOsInTimeFrame(db, queryParams) }, FormatOSStats),
		passthroughTask("topUrls", func() (interface{}, error) { return GetTopURLsInTimeFrame(db, queryParams) }),
		passthroughTask("topCustomEvents", func() (interface{}, error) { return GetTopCustomEventsInTimeFrame(db, queryParams) }),
		passthroughTask("eventRevenueTotals", func() (interface{}, error) { return GetEventRevenueTotals(db, queryParams) }),
		passthroughTask("bounceRate", func() (interface{}, error) { return GetBounceRateInTimeFrame(db, queryParams) }),
		passthroughTask("visitsDuration", func() (interface{}, error) { return GetVisitDurationInTimeFrame(db, queryParams) }),
		passthroughTask("revenuePerVisitor", func() (interface{}, error) { return GetRevenuePerVisitor(db, queryParams) }),
		passthroughTask("topEntryPages", func() (interface{}, error) { return GetTopEntryPagesInTimeFrame(db, queryParams) }),
		passthroughTask("topExitPages", func() (interface{}, error) { return GetTopExitPagesInTimeFrame(db, queryParams) }),
		passthroughTask("topUTMMediums", func() (interface{}, error) { return GetTopUTMMediumsInTimeFrame(db, queryParams) }),
		passthroughTask("topUTMSources", func() (interface{}, error) { return GetTopUTMSourcesInTimeFrame(db, queryParams) }),
		passthroughTask("topUTMCampaigns", func() (interface{}, error) { return GetTopUTMCampaignsInTimeFrame(db, queryParams) }),
		passthroughTask("topUTMTerms", func() (interface{}, error) { return GetTopUTMTermsInTimeFrame(db, queryParams) }),
		passthroughTask("topUTMContents", func() (interface{}, error) { return GetTopUTMContentsInTimeFrame(db, queryParams) }),
		passthroughTask("topRefParams", func() (interface{}, error) { return GetTopQueryParamValuesInTimeFrame(db, queryParams, "ref") }),
		passthroughTask("totalVisitors", func() (interface{}, error) { return GetTotalVisitorsInTimeFrame(db, queryParams) }),
		passthroughTask("totalViews", func() (interface{}, error) { return GetTotalPageViewsInTimeFrame(db, queryParams) }),
		passthroughTask("totalSessions", func() (interface{}, error) { return GetTotalSessionsInTimeFrame(db, queryParams) }),
		passthroughTask("totalEntryCount", func() (interface{}, error) { return GetTotalEntryCountInTimeFrame(db, queryParams) }),
		passthroughTask("totalExitCount", func() (interface{}, error) { return GetTotalExitCountInTimeFrame(db, queryParams) }),
		passthroughTask("totalCustomEvents", func() (interface{}, error) { return GetTotalCustomEventsInTimeFrame(db, queryParams) }),
		passthroughTask("revenueMetrics", func() (interface{}, error) { return GetRevenueMetrics(db, queryParams) }),
		passthroughTask("topRevenueEvents", func() (interface{}, error) { return GetTopRevenueEvents(db, queryParams) }),
		{Name: "conversionGoals", Execute: func() (interface{}, error) { return conversionGoals, nil }},
	}

	pool := async.NewPool(12)
	results := pool.Execute(context.Background(), tasks)

	for name, result := range results {
		if result.Err != nil {
			return nil, fmt.Errorf("error fetching %s: %w", name, result.Err)
		}
	}

	resp := &DashboardMetrics{
		PageViews:            results["pageViews"].Data.([]TimeSeriesPoint),
		Visitors:             results["visitors"].Data.([]TimeSeriesPoint),
		Sessions:             results["sessions"].Data.([]TimeSeriesPoint),
		GoalConversions:      results["revenue"].Data.([]TimeSeriesPoint),
		Revenue:              results["revenue"].Data.([]TimeSeriesPoint),
		TopURLs:              ensureNonNil(metricResultsOrEmpty(results, "topUrls")),
		TopCountries:         ensureNonNil(metricResultsOrEmpty(results, "topCountries")),
		TopDevices:           ensureNonNil(metricResultsOrEmpty(results, "topDevices")),
		TopReferrers:         ensureNonNil(metricResultsOrEmpty(results, "topReferrers")),
		TopBrowsers:          ensureNonNil(metricResultsOrEmpty(results, "topBrowsers")),
		TopCustomEvents:      ensureNonNil(metricResultsOrEmpty(results, "topCustomEvents")),
		EventConversionRates: map[string]float64{},
		TopOperatingSystems:  ensureNonNil(metricResultsOrEmpty(results, "topOperatingSystems")),
		EventRevenueTotals:   revenueTotalsOrEmpty(results, "eventRevenueTotals"),
		BounceRate:           results["bounceRate"].Data.(float64),
		VisitsDuration:       results["visitsDuration"].Data.(float64),
		RevenuePerVisitor:    results["revenuePerVisitor"].Data.(float64),
		TopEntryPages:        ensureNonNil(metricResultsOrEmpty(results, "topEntryPages")),
		TopExitPages:         ensureNonNil(metricResultsOrEmpty(results, "topExitPages")),
		TopUTMMediums:        ensureNonNil(metricResultsOrEmpty(results, "topUTMMediums")),
		TopUTMSources:        ensureNonNil(metricResultsOrEmpty(results, "topUTMSources")),
		TopUTMCampaigns:      ensureNonNil(metricResultsOrEmpty(results, "topUTMCampaigns")),
		TopUTMTerms:          ensureNonNil(metricResultsOrEmpty(results, "topUTMTerms")),
		TopUTMContents:       ensureNonNil(metricResultsOrEmpty(results, "topUTMContents")),
		TopRefParams:         ensureNonNil(metricResultsOrEmpty(results, "topRefParams")),
		BucketSize:           string(tf.BucketSize),
		TotalVisitors:        results["totalVisitors"].Data.(int64),
		TotalViews:           results["totalViews"].Data.(int64),
		TotalSessions:        results["totalSessions"].Data.(int64),
		TotalEntryCount:      results["totalEntryCount"].Data.(int64),
		TotalExitCount:       results["totalExitCount"].Data.(int64),
		TotalCustomEvents:    results["totalCustomEvents"].Data.(int64),
		RevenueMetrics:       results["revenueMetrics"].Data.(*RevenueMetrics),
		TopRevenueEvents:     ensureNonNil(metricResultsOrEmpty(results, "topRevenueEvents")),
		ConversionGoals:      results["conversionGoals"].Data.([]string),
		Insights:             []interface{}{},
		UserFlow:             []UserFlowLink{},
	}

	resp.EventConversionRates = buildEventConversionRates(resp)

	return resp, nil
}

// FetchComparisonMetrics loads comparison period metrics for deferred rendering.
func FetchComparisonMetrics(db *gorm.DB, tf *timeframe.TimeFrame, websiteId int, currentMetrics *DashboardMetrics, logger *slog.Logger) *ComparisonMetrics {
	duration := tf.To.Sub(tf.From)
	comparisonFrom := tf.From.Add(-duration)
	comparisonTo := tf.From

	comparisonTF := &timeframe.TimeFrame{
		From:       comparisonFrom,
		To:         comparisonTo,
		BucketSize: tf.BucketSize,
	}
	comparisonParams := NewWebsiteScopedQueryParams(comparisonTF, websiteId)

	tasks := []async.Task{
		passthroughTask("comparisonVisitors", func() (interface{}, error) { return GetTotalVisitorsInTimeFrame(db, comparisonParams) }),
		passthroughTask("comparisonViews", func() (interface{}, error) { return GetTotalPageViewsInTimeFrame(db, comparisonParams) }),
		passthroughTask("comparisonSessions", func() (interface{}, error) { return GetTotalSessionsInTimeFrame(db, comparisonParams) }),
		passthroughTask("comparisonBounceRate", func() (interface{}, error) { return GetBounceRateInTimeFrame(db, comparisonParams) }),
		passthroughTask("comparisonVisitsDuration", func() (interface{}, error) { return GetVisitDurationInTimeFrame(db, comparisonParams) }),
		passthroughTask("comparisonRevenueMetrics", func() (interface{}, error) { return GetRevenueMetrics(db, comparisonParams) }),
	}

	pool := async.NewPool(6)
	results := pool.Execute(context.Background(), tasks)

	data := ComparisonData{
		CurrentVisitors:   currentMetrics.TotalVisitors,
		CurrentViews:      currentMetrics.TotalViews,
		CurrentSessions:   currentMetrics.TotalSessions,
		CurrentBounceRate: currentMetrics.BounceRate,
		CurrentAvgTime:    currentMetrics.VisitsDuration,
	}

	if v, ok := results["comparisonVisitors"].Data.(int64); ok {
		data.PreviousVisitors = v
	}
	if v, ok := results["comparisonViews"].Data.(int64); ok {
		data.PreviousViews = v
	}
	if v, ok := results["comparisonSessions"].Data.(int64); ok {
		data.PreviousSessions = v
	}
	if v, ok := results["comparisonBounceRate"].Data.(float64); ok {
		data.PreviousBounceRate = v
	}
	if v, ok := results["comparisonVisitsDuration"].Data.(float64); ok {
		data.PreviousAvgTime = v
	}
	if currentMetrics.RevenueMetrics != nil {
		data.CurrentRevenue = currentMetrics.RevenueMetrics.TotalRevenue
	}
	if v, ok := results["comparisonRevenueMetrics"].Data.(*RevenueMetrics); ok && v != nil {
		data.PreviousRevenue = v.TotalRevenue
	}

	return CalculateComparisonMetrics(data)
}

// Task builder helpers

func timeSeriesTask(name string, fetch func() ([]timeframe.DateStat, error), logger *slog.Logger) async.Task {
	return async.Task{
		Name: name,
		Execute: func() (interface{}, error) {
			stats, err := fetch()
			if err != nil {
				logger.Error("Error fetching "+name, slog.Any("error", err))
				return []timeframe.DateStat{}, err
			}
			return convertToTimeSeries(stats), nil
		},
	}
}

func formattedMetricTask(name string, fetch func() ([]MetricCountResult, error), format func([]MetricCountResult) []MetricCountResult) async.Task {
	return async.Task{
		Name: name,
		Execute: func() (interface{}, error) {
			stats, err := fetch()
			if err != nil {
				return nil, err
			}
			return format(stats), nil
		},
	}
}

func passthroughTask(name string, execute func() (interface{}, error)) async.Task {
	return async.Task{Name: name, Execute: execute}
}

// Internal helpers

func convertToTimeSeries(stats []timeframe.DateStat) []TimeSeriesPoint {
	result := make([]TimeSeriesPoint, len(stats))
	for i, stat := range stats {
		result[i] = TimeSeriesPoint{Date: stat.Date, Count: int(stat.Count)}
	}
	return result
}

func buildEventConversionRates(resp *DashboardMetrics) map[string]float64 {
	rates := make(map[string]float64, len(resp.TopCustomEvents))
	if resp.TotalVisitors <= 0 {
		return rates
	}

	denominator := float64(resp.TotalVisitors)
	for _, event := range resp.TopCustomEvents {
		if event.Count <= 0 {
			continue
		}
		rates[event.Name] = float64(event.Count) / denominator * 100
	}
	return rates
}

func metricResultsOrEmpty(results map[string]async.Result, name string) []MetricCountResult {
	if result, exists := results[name]; exists && result.Data != nil {
		return result.Data.([]MetricCountResult)
	}
	return []MetricCountResult{}
}

func revenueTotalsOrEmpty(results map[string]async.Result, name string) map[string]float64 {
	if result, exists := results[name]; exists && result.Data != nil {
		if totals, ok := result.Data.(map[string]float64); ok {
			return totals
		}
	}
	return map[string]float64{}
}

func ensureNonNil(items []MetricCountResult) []MetricCountResult {
	if items == nil {
		return []MetricCountResult{}
	}
	return items
}
