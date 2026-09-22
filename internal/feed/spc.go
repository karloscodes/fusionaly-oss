package feed

import (
	"math"
	"slices"
	"time"

	"gorm.io/gorm"
)

// SPC Configuration Constants
const (
	// BaselineWeeks is how many past same-weekday days form a baseline.
	// Comparing a Monday with past Mondays absorbs weekly patterns (B2B sites
	// peak on weekdays, blogs on weekends).
	BaselineWeeks = 8

	// MinBaselineWeeks is how many same-weekday samples we need before we trust
	// their mean and stddev. Younger sites fall back to the last 7 days.
	MinBaselineWeeks = 3

	// SpikeSigma is the z-score a day must reach to count as a spike (~95%).
	SpikeSigma = 2.0

	// ColdStartVariance is the assumed coefficient of variation (stddev/mean)
	// while a site has fewer than MinBaselineWeeks weeks of history. Day-to-day
	// analytics traffic routinely swings ±40-50% just from sampling noise. At the old 0.25 a z=2 alert
	// fired on an ordinary +50% day — pure noise. At 0.45, a day must be about
	// +90% (nearly double) before it counts as a spike, and about -90% before
	// it counts as a drop. Combined with the absolute volume floors below, this
	// keeps quiet sites quiet while still catching genuine surges.
	ColdStartVariance = 0.45

	// --- Absolute noise floors (the "stay quiet on small sites" guarantee) ---
	// SPC is scale-free: on a 2-visitor/day site a jump to 5 is a 2.5-sigma
	// "spike" even though nothing real happened. These floors require a minimum
	// absolute volume before a detector is allowed to emit, independent of any
	// z-score. They are the primary lever that keeps low-traffic sites silent.

	// Floors are calibrated for small sites (solo devs, indie hackers): a tiny
	// site getting a handful of signups or a new referrer IS news. They are set
	// just high enough to kill pure noise (a 2→5 visitor "spike"), not to mute
	// the moments the feed exists to surface.

	// MinSpikeVisitors is the minimum visitors a day must have before it can be
	// reported as a traffic spike. Below this, a "spike" is just a quiet site.
	MinSpikeVisitors = 10

	// MinDropVisitors is the minimum a site must AVERAGE on a given weekday before
	// a quiet day is worth flagging as a drop. A drop is a "you lost real humans"
	// signal — a source dried up, a campaign ended, tracking broke. On a small site
	// a quiet day is just variance, never news, so the floor keeps small sites
	// silent. At 500/day a flagged drop means losing ~150+ real visitors.
	MinDropVisitors = 500

	// MinDropPercent is how far below the typical day yesterday must fall before a
	// drop is flagged. We use a plain magnitude rule (not a z-score) because SPC is
	// scale-free: on a tiny site an 11→1 dip is "statistically significant" yet
	// meaningless. A 30% fall on a real-traffic site is a genuine signal.
	MinDropPercent = 30.0

	// MinGoalConversions is the minimum conversions before a goal spike is
	// reported. One or two conversions is never a story; three on a small site is.
	MinGoalConversions = 3

	// MinReferrerVisitors is the minimum visitors a brand-new source must send
	// before it earns a feed item.
	MinReferrerVisitors = 3

	// MinTrendingVisitors is the minimum visitors a page needs in a day before
	// it can be flagged as trending or newly popular.
	MinTrendingVisitors = 8

	// MinDroppingPageVisitors is the minimum prior-month visitors a page needs
	// before a month-over-month drop is worth surfacing.
	MinDroppingPageVisitors = 20
)

const dateLayout = "2006-01-02"

// Metric is one daily count in a stats table, such as visitors in site_stats
// or conversions of one goal in event_stats.
type Metric struct {
	Table  string // "site_stats", "event_stats", "page_stats"
	Column string // column to sum per day
	Filter string // optional extra condition, e.g. "event_name = ?"
	Args   []any
}

// Baseline is the typical value of a metric on one weekday.
type Baseline struct {
	Typical float64 // median of past days
	Spread  float64 // robust stddev estimate (scaled MAD)
	Total   float64 // sum over the whole lookback window; 0 means no history
}

// ZScore returns how many spreads current sits above the typical value.
func (b Baseline) ZScore(current float64) float64 {
	return (current - b.Typical) / b.Spread
}

// IsSpike reports whether current is a statistically significant rise.
func (b Baseline) IsSpike(current float64) bool {
	return b.ZScore(current) >= SpikeSigma
}

// BaselineFor computes the baseline for day from the stats tables. It compares
// day with the same weekday over the last BaselineWeeks weeks. Sites younger
// than MinBaselineWeeks weeks fall back to the last 7 days with an assumed
// ColdStartVariance. Nothing is stored: the stats tables are the history.
//
// It uses the median and MAD, not the mean and stddev, so a viral day in the
// window cannot hide the next spike. Up to 3 of 8 samples can be outliers.
func BaselineFor(db *gorm.DB, websiteID uint, m Metric, day time.Time) Baseline {
	from := day.AddDate(0, 0, -7*BaselineWeeks)
	totals := dailyTotals(db, websiteID, m, from, day)

	var total float64
	for _, v := range totals {
		total += v
	}

	siteStart := firstTrackedDay(db, websiteID)
	var sameWeekday []float64
	for w := 1; w <= BaselineWeeks; w++ {
		d := day.AddDate(0, 0, -7*w).Format(dateLayout)
		if d < siteStart {
			break
		}
		sameWeekday = append(sameWeekday, totals[d])
	}

	if len(sameWeekday) >= MinBaselineWeeks {
		typical := median(sameWeekday)
		return Baseline{Typical: typical, Spread: noiseFloor(typical, mad(sameWeekday, typical)), Total: total}
	}

	// Cold start. The last 7 days mix weekdays and weekends, so their spread
	// measures the weekly pattern, not noise: assume ColdStartVariance instead.
	var lastWeek []float64
	for i := 1; i <= 7; i++ {
		d := day.AddDate(0, 0, -i).Format(dateLayout)
		if d < siteStart {
			break
		}
		lastWeek = append(lastWeek, totals[d])
	}
	typical := median(lastWeek)
	return Baseline{Typical: typical, Spread: noiseFloor(typical, typical*ColdStartVariance), Total: total}
}

// noiseFloor stops a steady metric from producing a near-zero spread, which
// would turn any small bump into a spike. Counts vary by at least
// sqrt(typical) (Poisson noise); the floor of 1 covers metrics that are
// usually zero, where MAD is 0.
func noiseFloor(typical, spread float64) float64 {
	return math.Max(spread, math.Sqrt(math.Max(typical, 1)))
}

// median returns the middle value, or 0 for no values.
func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

// mad returns the median absolute deviation, scaled by 1.4826 so it matches
// the stddev on normally distributed data.
func mad(values []float64, center float64) float64 {
	deviations := make([]float64, len(values))
	for i, v := range values {
		deviations[i] = math.Abs(v - center)
	}
	return median(deviations) * 1.4826
}

// dailyTotals returns the metric's total per day in [from, to), keyed by date.
// Days without rows are absent, and callers read them as zero.
func dailyTotals(db *gorm.DB, websiteID uint, m Metric, from, to time.Time) map[string]float64 {
	type row struct {
		Day   string
		Total float64
	}
	var rows []row

	q := db.Table(m.Table).
		Select("DATE(hour) AS day, COALESCE(SUM("+m.Column+"), 0) AS total").
		Where("website_id = ? AND DATE(hour) >= ? AND DATE(hour) < ?",
			websiteID, from.Format(dateLayout), to.Format(dateLayout))
	if m.Filter != "" {
		q = q.Where(m.Filter, m.Args...)
	}
	q.Group("DATE(hour)").Scan(&rows)

	totals := make(map[string]float64, len(rows))
	for _, r := range rows {
		totals[r.Day] = r.Total
	}
	return totals
}

// firstTrackedDay returns the first date the site received traffic. Weeks
// before it are not zero-traffic weeks, so they stay out of the baseline.
func firstTrackedDay(db *gorm.DB, websiteID uint) string {
	var first *string
	db.Table("site_stats").
		Where("website_id = ?", websiteID).
		Select("MIN(DATE(hour))").Scan(&first)
	if first == nil {
		return "9999-12-31"
	}
	return *first
}
