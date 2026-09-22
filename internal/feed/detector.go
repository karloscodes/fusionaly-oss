package feed

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"time"

	"fusionaly/internal/pkg/referrers"

	"gorm.io/gorm"
)

// Detector runs anomaly detection for all websites
type Detector struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewDetector creates a new feed detector
func NewDetector(db *gorm.DB, logger *slog.Logger) *Detector {
	return &Detector{db: db, logger: logger}
}

// DetectAll runs all detectors for all websites
func (d *Detector) DetectAll() error {
	// Get all website IDs
	var websiteIDs []uint
	if err := d.db.Table("websites").Pluck("id", &websiteIDs).Error; err != nil {
		return fmt.Errorf("failed to get websites: %w", err)
	}

	d.logger.Info("Running feed detection", slog.Int("websiteCount", len(websiteIDs)))

	for _, websiteID := range websiteIDs {
		if err := d.DetectForWebsite(websiteID); err != nil {
			d.logger.Error("Failed to detect for website",
				slog.Uint64("websiteId", uint64(websiteID)),
				slog.Any("error", err))
			// Continue with other websites
		}
	}

	return nil
}

// DetectForWebsite runs all detectors for a single website
func (d *Detector) DetectForWebsite(websiteID uint) error {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1).Truncate(24 * time.Hour)

	// Traffic changes (compare yesterday vs the same weekday in past weeks)
	d.detectTrafficChanges(websiteID, yesterday)

	// New referrers (first-time sources with significant traffic)
	d.detectNewReferrers(websiteID, yesterday)

	// Goal spikes (unusual conversion activity)
	d.detectGoalSpikes(websiteID, yesterday)

	// Milestones (round number achievements)
	d.detectMilestones(websiteID)

	// Trending content (pages with unusual traffic)
	d.detectTrendingContent(websiteID, yesterday)

	// Page problems (high bounce on specific pages)
	d.detectPageProblems(websiteID, yesterday)

	// Monthly insights (run on 1st of month, or anytime for previous month)
	d.detectMonthlySummary(websiteID)
	d.detectDroppingPages(websiteID)
	d.detectBestSources(websiteID)

	return nil
}

// detectTrafficChanges uses SPC to detect statistically significant traffic changes
// against the same weekday in past weeks.
func (d *Detector) detectTrafficChanges(websiteID uint, yesterday time.Time) {
	// Get yesterday's visitors
	var yesterdayVisitors int64
	d.db.Table("site_stats").
		Where("website_id = ? AND DATE(hour) = DATE(?)", websiteID, yesterday).
		Select("COALESCE(SUM(visitors), 0)").Scan(&yesterdayVisitors)

	if yesterdayVisitors == 0 {
		return
	}

	baseline := BaselineFor(d.db, websiteID, Metric{Table: "site_stats", Column: "visitors"}, yesterday)
	if baseline.Total == 0 {
		return
	}
	typical := baseline.Typical

	// Calculate z-score and percent change for display
	zScore := baseline.ZScore(float64(yesterdayVisitors))
	percentChange := 0.0
	if typical > 0 {
		percentChange = (float64(yesterdayVisitors) - typical) / typical * 100
	}

	// Traffic spike: z-score >= 2 AND meaningful absolute volume.
	// The volume floor keeps low-traffic sites quiet: a jump from 2 to 6
	// visitors is statistically a "spike" but is not worth a feed item.
	if baseline.IsSpike(float64(yesterdayVisitors)) && yesterdayVisitors >= MinSpikeVisitors {
		desc := formatChange(yesterdayVisitors, int64(typical), "visitors")
		item := &FeedItem{
			WebsiteID:   websiteID,
			ItemType:    ItemTypeTrafficSpike,
			Title:       "Busy day",
			Description: desc,
			DetectedAt:  time.Now(),
			PeriodStart: yesterday,
			PeriodEnd:   yesterday.Add(24 * time.Hour),
		}
		item.SetMetadata(map[string]any{
			"visitors":      yesterdayVisitors,
			"avgVisitors":   int64(typical),
			"percentChange": percentChange,
			"zScore":        zScore,
		})
		if err := CreateItem(d.db, item); err != nil {
			d.logger.Error("Failed to create traffic spike item", slog.Any("error", err))
		}
	}

	// Traffic drop: only on real-traffic sites that fall meaningfully. A drop is a
	// "you lost real humans" signal, so we gate on absolute volume (the site must
	// average >= MinDropVisitors) AND magnitude (yesterday >= MinDropPercent below
	// the typical day). We deliberately do NOT use a z-score: SPC is scale-free and
	// flags an 11→1 dip on a tiny site as "significant" noise.
	if typical >= MinDropVisitors && percentChange <= -MinDropPercent {
		item := &FeedItem{
			WebsiteID:   websiteID,
			ItemType:    ItemTypeTrafficDrop,
			Title:       "Slow day",
			Description: fmt.Sprintf("%d visitors (vs %d avg).", yesterdayVisitors, int64(typical)),
			DetectedAt:  time.Now(),
			PeriodStart: yesterday,
			PeriodEnd:   yesterday.Add(24 * time.Hour),
		}
		item.SetMetadata(map[string]any{
			"visitors":      yesterdayVisitors,
			"avgVisitors":   int64(typical),
			"percentChange": percentChange,
			"zScore":        zScore,
		})
		if err := CreateItem(d.db, item); err != nil {
			d.logger.Error("Failed to create traffic drop item", slog.Any("error", err))
		}
	}
}

// detectNewReferrers finds first-time traffic sources with significant traffic
func (d *Detector) detectNewReferrers(websiteID uint, yesterday time.Time) {
	// Get referrers from yesterday with >10 visitors
	type refRow struct {
		Hostname string
		Visitors int64
	}
	var yesterdayRefs []refRow

	d.db.Table("ref_stats").
		Select("hostname, SUM(visitors_count) as visitors").
		Where("website_id = ? AND DATE(hour) = DATE(?) AND hostname != '' AND hostname != '(direct)' AND hostname != '__direct_or_unknown__'", websiteID, yesterday).
		Group("hostname").
		Having("visitors >= ?", MinReferrerVisitors).
		Order("visitors DESC").
		Limit(5).
		Scan(&yesterdayRefs)

	for _, ref := range yesterdayRefs {
		// Check if this referrer appeared before yesterday
		weekBefore := yesterday.AddDate(0, 0, -7)
		var historicalVisitors int64
		d.db.Table("ref_stats").
			Where("website_id = ? AND hostname = ? AND DATE(hour) >= DATE(?) AND DATE(hour) < DATE(?)",
				websiteID, ref.Hostname, weekBefore, yesterday).
			Select("COALESCE(SUM(visitors_count), 0)").Scan(&historicalVisitors)

		// If no historical traffic, this is a new referrer
		if historicalVisitors == 0 {
			friendlyName := friendlySourceName(ref.Hostname)
			item := &FeedItem{
				WebsiteID:   websiteID,
				ItemType:    ItemTypeNewReferrer,
				Title:       friendlyName,
				Description: fmt.Sprintf("%d visitors from %s.", ref.Visitors, friendlyName),
				DetectedAt:  time.Now(),
				PeriodStart: yesterday,
				PeriodEnd:   yesterday.Add(24 * time.Hour),
			}
			item.SetMetadata(map[string]any{
				"hostname":     ref.Hostname,
				"friendlyName": friendlyName,
				"visitors":     ref.Visitors,
			})
			if err := CreateItem(d.db, item); err != nil {
				d.logger.Error("Failed to create new referrer item", slog.Any("error", err))
			}
		}
	}
}

// detectGoalSpikes uses SPC to find statistically significant goal conversion spikes.
// Only tracks events that are configured as goals for this website.
func (d *Detector) detectGoalSpikes(websiteID uint, yesterday time.Time) {
	// Get configured goals for this website
	configuredGoals := d.getConfiguredGoals(websiteID)
	if len(configuredGoals) == 0 {
		return // No goals configured, nothing to track
	}

	// Get yesterday's conversions for configured goals only
	type goalRow struct {
		EventName string
		Count     int64
	}
	var yesterdayGoals []goalRow

	d.db.Table("event_stats").
		Select("event_name, SUM(visitors_count) as count").
		Where("website_id = ? AND DATE(hour) = DATE(?) AND event_name IN ?", websiteID, yesterday, configuredGoals).
		Group("event_name").
		Having("count >= ?", MinGoalConversions).
		Scan(&yesterdayGoals)

	for _, goal := range yesterdayGoals {
		metric := Metric{Table: "event_stats", Column: "visitors_count", Filter: "event_name = ?", Args: []any{goal.EventName}}
		baseline := BaselineFor(d.db, websiteID, metric, yesterday)
		if baseline.Total == 0 {
			continue // First conversions ever: no history to compare against
		}
		typical := baseline.Typical

		if baseline.IsSpike(float64(goal.Count)) {
			zScore := baseline.ZScore(float64(goal.Count))
			desc := formatGoalChange(goal.EventName, goal.Count, int64(typical))
			item := &FeedItem{
				WebsiteID:   websiteID,
				ItemType:    ItemTypeGoalHit,
				Title:       "Conversions up",
				Description: desc,
				DetectedAt:  time.Now(),
				PeriodStart: yesterday,
				PeriodEnd:   yesterday.Add(24 * time.Hour),
			}
			item.SetMetadata(map[string]any{
				"goalName":       goal.EventName,
				"conversions":    goal.Count,
				"avgConversions": typical,
				"zScore":         zScore,
			})
			if err := CreateItem(d.db, item); err != nil {
				d.logger.Error("Failed to create goal spike item", slog.Any("error", err))
			}
		}
	}
}

// detectMilestones checks for round number achievements
func (d *Detector) detectMilestones(websiteID uint) {
	// Get total visitors for this website
	var totalVisitors int64
	d.db.Table("site_stats").
		Where("website_id = ?", websiteID).
		Select("COALESCE(SUM(visitors), 0)").Scan(&totalVisitors)

	// Check milestones: 1k, 5k, 10k, 25k, 50k, 100k, 250k, 500k, 1M, etc.
	milestones := []int64{1000, 5000, 10000, 25000, 50000, 100000, 250000, 500000, 1000000}

	for _, milestone := range milestones {
		if totalVisitors >= milestone {
			// Check if we already have this milestone recorded
			var existing FeedItem
			result := d.db.Where(
				"website_id = ? AND item_type = ? AND metadata LIKE ?",
				websiteID, ItemTypeMilestone, fmt.Sprintf(`%%"milestone":%d%%`, milestone),
			).First(&existing)

			if result.Error != nil {
				// Milestone not recorded yet, check if we crossed it recently
				// (within last 7 days based on visitor accumulation rate)
				weekAgo := time.Now().AddDate(0, 0, -7)
				var weekAgoVisitors int64
				d.db.Table("site_stats").
					Where("website_id = ? AND hour < ?", websiteID, weekAgo).
					Select("COALESCE(SUM(visitors), 0)").Scan(&weekAgoVisitors)

				// If we crossed this milestone in the last 7 days
				if weekAgoVisitors < milestone && totalVisitors >= milestone {
					item := &FeedItem{
						WebsiteID:   websiteID,
						ItemType:    ItemTypeMilestone,
						Title:       "Milestone",
						Description: fmt.Sprintf("%s total visitors.", formatNumber(milestone)),
						DetectedAt:  time.Now(),
						PeriodStart: weekAgo,
						PeriodEnd:   time.Now(),
					}
					item.SetMetadata(map[string]any{
						"milestone":     milestone,
						"totalVisitors": totalVisitors,
					})
					if err := CreateItem(d.db, item); err != nil {
						d.logger.Error("Failed to create milestone item", slog.Any("error", err))
					}
					break // Only report one milestone at a time
				}
			}
		}
	}
}

// detectTrendingContent uses SPC to find pages with statistically significant traffic spikes.
func (d *Detector) detectTrendingContent(websiteID uint, yesterday time.Time) {
	// Get yesterday's top pages by visitors
	type pageRow struct {
		Pathname string
		Visitors int64
	}
	var yesterdayPages []pageRow

	d.db.Table("page_stats").
		Select("pathname, SUM(visitors_count) as visitors").
		Where("website_id = ? AND DATE(hour) = DATE(?) AND pathname != '/'", websiteID, yesterday).
		Group("pathname").
		Having("visitors >= ?", MinTrendingVisitors). // Absolute floor: ignore low-volume pages
		Order("visitors DESC").
		Limit(10).
		Scan(&yesterdayPages)

	for _, page := range yesterdayPages {
		metric := Metric{Table: "page_stats", Column: "visitors_count", Filter: "pathname = ?", Args: []any{page.Pathname}}
		baseline := BaselineFor(d.db, websiteID, metric, yesterday)
		typical := baseline.Typical

		isNew := baseline.Total == 0
		isTrending := !isNew && baseline.IsSpike(float64(page.Visitors))

		if isNew || isTrending {
			pageName := friendlyPageName(page.Pathname)
			var title, description string
			if isNew {
				title = "New page"
				description = fmt.Sprintf("%s: %d visitors.", pageName, page.Visitors)
			} else {
				title = "Popular page"
				description = formatPageChange(pageName, page.Visitors, int64(typical))
			}

			zScore := baseline.ZScore(float64(page.Visitors))
			item := &FeedItem{
				WebsiteID:   websiteID,
				ItemType:    ItemTypeTrendingContent,
				Title:       title,
				Description: description,
				DetectedAt:  time.Now(),
				PeriodStart: yesterday,
				PeriodEnd:   yesterday.Add(24 * time.Hour),
			}
			item.SetMetadata(map[string]any{
				"pathname":    page.Pathname,
				"visitors":    page.Visitors,
				"avgVisitors": int64(typical),
				"isNew":       isNew,
				"zScore":      zScore,
			})
			if err := CreateItem(d.db, item); err != nil {
				d.logger.Error("Failed to create trending content item", slog.Any("error", err))
			}
		}
	}
}

// detectPageProblems finds pages with unusually high bounce rates
// NOTE: Currently disabled - page_stats schema doesn't have bounce_count column
func (d *Detector) detectPageProblems(websiteID uint, yesterday time.Time) {
	// Schema doesn't support bounce rate detection yet
	// Would need bounce_count column in page_stats table
}

// friendlySourceName returns a display-friendly name for a referrer hostname.
// Translates internal constants like __direct_or_unknown__ to human-readable text.
func friendlySourceName(hostname string) string {
	if hostname == "__direct_or_unknown__" || hostname == "(direct)" {
		return "Direct / Unknown"
	}
	return referrers.FriendlyName(hostname)
}

// friendlyPageName returns a display-friendly name for a page path.
// Translates bare "/" to "Homepage".
func friendlyPageName(pathname string) string {
	if pathname == "/" || pathname == "" {
		return "Homepage"
	}
	return pathname
}

// formatNumber formats a number with K/M suffix
func formatNumber(n int64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%dM", n/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%dK", n/1000)
	}
	return fmt.Sprintf("%d", n)
}

// formatChange formats a value vs average comparison.
// Uses multipliers (3x normal) for big swings, plain comparison for smaller changes.
func formatChange(current, avg int64, unit string) string {
	if avg == 0 {
		return fmt.Sprintf("%d %s.", current, unit)
	}
	ratio := float64(current) / float64(avg)
	if ratio >= 2.5 {
		return fmt.Sprintf("%d %s (%.0fx normal).", current, unit, ratio)
	}
	return fmt.Sprintf("%d %s (vs %d avg).", current, unit, avg)
}

// formatGoalChange formats goal/conversion changes with the goal name.
func formatGoalChange(goalName string, current, avg int64) string {
	if avg == 0 {
		return fmt.Sprintf("%s: %d conversions.", goalName, current)
	}
	ratio := float64(current) / float64(avg)
	if ratio >= 2.5 {
		return fmt.Sprintf("%s: %d conversions (%.0fx normal).", goalName, current, ratio)
	}
	return fmt.Sprintf("%s: %d conversions (vs %d avg).", goalName, current, avg)
}

// formatPageChange formats page traffic changes.
func formatPageChange(pathname string, current, avg int64) string {
	if avg == 0 {
		return fmt.Sprintf("%s: %d visitors.", pathname, current)
	}
	ratio := float64(current) / float64(avg)
	if ratio >= 2.5 {
		return fmt.Sprintf("%s: %d visitors (%.0fx normal).", pathname, current, ratio)
	}
	return fmt.Sprintf("%s: %d visitors (vs %d avg).", pathname, current, avg)
}

// getConfiguredGoals returns the list of event names configured as goals for a website.
// Reads from the website_goals setting in the database.
func (d *Detector) getConfiguredGoals(websiteID uint) []string {
	var settingValue string
	err := d.db.Table("settings").
		Where("key = ?", "website_goals").
		Pluck("value", &settingValue).Error
	if err != nil || settingValue == "" {
		return nil
	}

	// Parse JSON: {"goals": {"1": ["signup", "purchase"], "2": ["subscribe"]}}
	type goalsConfig struct {
		Goals map[string][]string `json:"goals"`
	}
	var config goalsConfig
	if err := json.Unmarshal([]byte(settingValue), &config); err != nil {
		return nil
	}

	websiteIDStr := fmt.Sprintf("%d", websiteID)
	return config.Goals[websiteIDStr]
}

// detectMonthlySummary creates a monthly recap on the 1st of each month
func (d *Detector) detectMonthlySummary(websiteID uint) {
	now := time.Now()

	// Calculate previous month boundaries
	firstOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	firstOfLastMonth := firstOfThisMonth.AddDate(0, -1, 0)
	firstOfTwoMonthsAgo := firstOfThisMonth.AddDate(0, -2, 0)

	// Get last month's total visitors
	var lastMonthVisitors int64
	d.db.Table("site_stats").
		Where("website_id = ? AND hour >= ? AND hour < ?", websiteID, firstOfLastMonth, firstOfThisMonth).
		Select("COALESCE(SUM(visitors), 0)").Scan(&lastMonthVisitors)

	// Skip if zero traffic
	if lastMonthVisitors < 1 {
		return
	}

	// Get previous month's visitors for comparison
	var prevMonthVisitors int64
	d.db.Table("site_stats").
		Where("website_id = ? AND hour >= ? AND hour < ?", websiteID, firstOfTwoMonthsAgo, firstOfLastMonth).
		Select("COALESCE(SUM(visitors), 0)").Scan(&prevMonthVisitors)

	// Get top 5 pages
	type pageRow struct {
		Pathname string
		Visitors int64
	}
	var topPages []pageRow
	d.db.Table("page_stats").
		Select("pathname, SUM(visitors_count) as visitors").
		Where("website_id = ? AND hour >= ? AND hour < ?", websiteID, firstOfLastMonth, firstOfThisMonth).
		Group("pathname").
		Order("visitors DESC").
		Limit(5).
		Scan(&topPages)

	// Get top 5 sources
	type refRow struct {
		Hostname string
		Visitors int64
	}
	var topSources []refRow
	d.db.Table("ref_stats").
		Select("hostname, SUM(visitors_count) as visitors").
		Where("website_id = ? AND hour >= ? AND hour < ? AND hostname != '' AND hostname != '(direct)' AND hostname != '__direct_or_unknown__'", websiteID, firstOfLastMonth, firstOfThisMonth).
		Group("hostname").
		Order("visitors DESC").
		Limit(5).
		Scan(&topSources)

	// Build description with actual insight
	monthName := firstOfLastMonth.Format("January 2006")

	// Get the highlight: top page or top source
	var highlight string
	if len(topPages) > 0 && topPages[0].Pathname != "/" && topPages[0].Pathname != "" {
		highlight = fmt.Sprintf("%s was your top page.", topPages[0].Pathname)
	} else if len(topSources) > 0 {
		friendlySource := friendlySourceName(topSources[0].Hostname)
		highlight = fmt.Sprintf("%s sent the most traffic.", friendlySource)
	}

	var description string
	if prevMonthVisitors > 0 {
		percentChange := (float64(lastMonthVisitors) - float64(prevMonthVisitors)) / float64(prevMonthVisitors) * 100
		if percentChange >= 0 {
			description = fmt.Sprintf("%s visitors, up %.0f%%.", formatNumber(lastMonthVisitors), percentChange)
		} else {
			description = fmt.Sprintf("%s visitors, down %.0f%%.", formatNumber(lastMonthVisitors), math.Abs(percentChange))
		}
		if highlight != "" {
			description += " " + highlight
		}
	} else {
		description = fmt.Sprintf("%s visitors.", formatNumber(lastMonthVisitors))
		if highlight != "" {
			description += " " + highlight
		}
	}

	// Convert to metadata format
	pagesData := make([]map[string]any, len(topPages))
	for i, p := range topPages {
		pagesData[i] = map[string]any{"pathname": p.Pathname, "visitors": p.Visitors}
	}
	sourcesData := make([]map[string]any, len(topSources))
	for i, s := range topSources {
		sourcesData[i] = map[string]any{"hostname": s.Hostname, "visitors": s.Visitors}
	}

	item := &FeedItem{
		WebsiteID:   websiteID,
		ItemType:    ItemTypeMonthlySummary,
		Title:       monthName,
		Description: description,
		DetectedAt:  time.Now(),
		PeriodStart: firstOfLastMonth,
		PeriodEnd:   firstOfThisMonth,
	}
	item.SetMetadata(map[string]any{
		"visitors":          lastMonthVisitors,
		"prevMonthVisitors": prevMonthVisitors,
		"topPages":          pagesData,
		"topSources":        sourcesData,
	})

	if err := CreateItem(d.db, item); err != nil {
		d.logger.Error("Failed to create monthly summary item", slog.Any("error", err))
	}
}

// detectDroppingPages finds pages with significant traffic drops month-over-month
func (d *Detector) detectDroppingPages(websiteID uint) {
	now := time.Now()

	// Calculate month boundaries
	firstOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	firstOfLastMonth := firstOfThisMonth.AddDate(0, -1, 0)
	firstOfTwoMonthsAgo := firstOfThisMonth.AddDate(0, -2, 0)

	// Get pages from two months ago with >= 50 visitors
	type pageRow struct {
		Pathname     string
		PrevVisitors int64
	}
	var prevMonthPages []pageRow
	d.db.Table("page_stats").
		Select("pathname, SUM(visitors_count) as prev_visitors").
		Where("website_id = ? AND hour >= ? AND hour < ?", websiteID, firstOfTwoMonthsAgo, firstOfLastMonth).
		Group("pathname").
		Having("prev_visitors >= ?", MinDroppingPageVisitors).
		Scan(&prevMonthPages)

	if len(prevMonthPages) == 0 {
		return
	}

	// Check each page for drops
	type droppingPage struct {
		Pathname      string
		PrevVisitors  int64
		CurrVisitors  int64
		PercentChange float64
	}
	var droppingPages []droppingPage

	for _, page := range prevMonthPages {
		var currVisitors int64
		d.db.Table("page_stats").
			Where("website_id = ? AND pathname = ? AND hour >= ? AND hour < ?", websiteID, page.Pathname, firstOfLastMonth, firstOfThisMonth).
			Select("COALESCE(SUM(visitors_count), 0)").Scan(&currVisitors)

		percentChange := (float64(currVisitors) - float64(page.PrevVisitors)) / float64(page.PrevVisitors) * 100

		// 30%+ drop
		if percentChange <= -30 {
			droppingPages = append(droppingPages, droppingPage{
				Pathname:      page.Pathname,
				PrevVisitors:  page.PrevVisitors,
				CurrVisitors:  currVisitors,
				PercentChange: percentChange,
			})
		}
	}

	if len(droppingPages) == 0 {
		return
	}

	// Sort by biggest drop and take top 5
	// Simple bubble sort for small slice
	for i := 0; i < len(droppingPages)-1; i++ {
		for j := i + 1; j < len(droppingPages); j++ {
			if droppingPages[j].PercentChange < droppingPages[i].PercentChange {
				droppingPages[i], droppingPages[j] = droppingPages[j], droppingPages[i]
			}
		}
	}
	if len(droppingPages) > 5 {
		droppingPages = droppingPages[:5]
	}

	// Build description that names names
	monthName := firstOfLastMonth.Format("January")
	topDrop := droppingPages[0]
	topDropName := friendlyPageName(topDrop.Pathname)

	var description string
	if len(droppingPages) == 1 {
		description = fmt.Sprintf("%s dropped %.0f%% in %s.", topDropName, math.Abs(topDrop.PercentChange), monthName)
	} else {
		description = fmt.Sprintf("%s and %d other pages lost traffic in %s.", topDropName, len(droppingPages)-1, monthName)
	}

	// Convert to metadata
	pagesData := make([]map[string]any, len(droppingPages))
	for i, p := range droppingPages {
		pagesData[i] = map[string]any{
			"pathname":      p.Pathname,
			"prevVisitors":  p.PrevVisitors,
			"currVisitors":  p.CurrVisitors,
			"percentChange": p.PercentChange,
		}
	}

	item := &FeedItem{
		WebsiteID:   websiteID,
		ItemType:    ItemTypeDroppingPages,
		Title:       "Traffic shifts",
		Description: description,
		DetectedAt:  time.Now(),
		PeriodStart: firstOfLastMonth,
		PeriodEnd:   firstOfThisMonth,
	}
	item.SetMetadata(map[string]any{
		"pages": pagesData,
	})

	if err := CreateItem(d.db, item); err != nil {
		d.logger.Error("Failed to create dropping pages item", slog.Any("error", err))
	}
}

// detectBestSources finds referrers with highest engagement (2+ page views)
func (d *Detector) detectBestSources(websiteID uint) {
	now := time.Now()

	// Calculate month boundaries
	firstOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	firstOfLastMonth := firstOfThisMonth.AddDate(0, -1, 0)

	// Get sessions with referrer info and page count
	// We need to count sessions per referrer and sessions with 2+ pages per referrer
	// This requires session-level data. Let's check if we have session stats or need to approximate.

	// Approximation: Use ref_stats visitors vs page_views ratio as engagement proxy
	// Engaged = page_views > visitors (meaning return visits or multi-page sessions)
	type refRow struct {
		Hostname  string
		Visitors  int64
		PageViews int64
	}
	var sources []refRow
	d.db.Table("ref_stats").
		Select("hostname, SUM(visitors_count) as visitors, SUM(page_views_count) as page_views").
		Where("website_id = ? AND hour >= ? AND hour < ? AND hostname != '' AND hostname != '(direct)' AND hostname != '__direct_or_unknown__'", websiteID, firstOfLastMonth, firstOfThisMonth).
		Group("hostname").
		Having("visitors >= 2").
		Order("visitors DESC").
		Limit(20). // Get top 20 to filter by engagement
		Scan(&sources)

	if len(sources) == 0 {
		return
	}

	// Calculate pages per visit (the real metric, no fake "engagement rate")
	type sourceWithDepth struct {
		Hostname      string
		Visitors      int64
		PageViews     int64
		PagesPerVisit float64
	}
	var deepSources []sourceWithDepth

	for _, s := range sources {
		if s.Visitors == 0 {
			continue
		}
		pagesPerVisit := float64(s.PageViews) / float64(s.Visitors)

		// Only include sources where visitors read significantly more than average
		// 3.0+ pages per visit indicates genuinely engaged readers
		if pagesPerVisit >= 3.0 {
			deepSources = append(deepSources, sourceWithDepth{
				Hostname:      s.Hostname,
				Visitors:      s.Visitors,
				PageViews:     s.PageViews,
				PagesPerVisit: pagesPerVisit,
			})
		}
	}

	// Sort by pages per visit (highest first)
	for i := 0; i < len(deepSources)-1; i++ {
		for j := i + 1; j < len(deepSources); j++ {
			if deepSources[j].PagesPerVisit > deepSources[i].PagesPerVisit {
				deepSources[i], deepSources[j] = deepSources[j], deepSources[i]
			}
		}
	}

	// Take top 5
	if len(deepSources) > 5 {
		deepSources = deepSources[:5]
	}

	// Only create if there's meaningful data
	if len(deepSources) == 0 {
		return
	}

	// Build description with real numbers
	topSource := friendlySourceName(deepSources[0].Hostname)
	topPagesPerVisit := deepSources[0].PagesPerVisit
	monthName := firstOfLastMonth.Format("January")

	var description string
	if len(deepSources) == 1 {
		description = fmt.Sprintf("%s visitors explored deeply in %s, averaging %d pages each.",
			topSource, monthName, int(topPagesPerVisit))
	} else {
		secondSource := friendlySourceName(deepSources[1].Hostname)
		description = fmt.Sprintf("%s and %s visitors explored deeply in %s.",
			topSource, secondSource, monthName)
	}

	// Convert to metadata
	sourcesData := make([]map[string]any, len(deepSources))
	for i, s := range deepSources {
		friendlyName := friendlySourceName(s.Hostname)
		sourcesData[i] = map[string]any{
			"hostname":      s.Hostname,
			"friendlyName":  friendlyName,
			"visitors":      s.Visitors,
			"pageViews":     s.PageViews,
			"pagesPerVisit": s.PagesPerVisit,
		}
	}

	item := &FeedItem{
		WebsiteID:   websiteID,
		ItemType:    ItemTypeBestSources,
		Title:       "Engaged readers",
		Description: description,
		DetectedAt:  time.Now(),
		PeriodStart: firstOfLastMonth,
		PeriodEnd:   firstOfThisMonth,
	}
	item.SetMetadata(map[string]any{
		"sources": sourcesData,
	})

	if err := CreateItem(d.db, item); err != nil {
		d.logger.Error("Failed to create best sources item", slog.Any("error", err))
	}
}
