package feed

import "time"

// SPC Configuration Constants
const (
	// Baseline learning
	BaselineSmoothingFactor = 0.1 // EMA weight for new data (10%)
	MinSamplesForBaseline   = 20  // Samples needed before trusting baseline

	// Defaults during cold start - calibrated for analytics
	// With mean=100 and stddev=25:
	//   - 50% increase (150) → z=2 → warning
	//   - 50% decrease (50) → z=-2 → warning
	// This matches the original hardcoded thresholds while enabling learning
	DefaultMean   = 100.0 // Default visitor count during cold start
	DefaultStdDev = 25.0  // 25% of mean - matches original 50% spike/drop thresholds

	// Control Chart thresholds (z-scores)
	WarningSigma  = 2.0 // 95% confidence - spike/drop detection
	CriticalSigma = 3.0 // 99.7% confidence - severe anomaly

	// ColdStartVariance is the assumed coefficient of variation (stddev/mean)
	// before a learned baseline exists. Day-to-day analytics traffic routinely
	// swings ±40-50% just from sampling noise. At the old 0.25 a z=2 alert
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

	// MinSpikeVisitors is the minimum visitors a day must have before it can be
	// reported as a traffic spike. Below this, a "spike" is just a quiet site.
	MinSpikeVisitors = 30

	// MinDropVisitors is the minimum average a site must normally see before a
	// quiet day is worth flagging as a drop. A site that averages a handful of
	// visitors has nothing meaningful to "drop".
	MinDropVisitors = 30

	// MinGoalConversions is the minimum conversions before a goal spike is
	// reported. One or two conversions is never a story.
	MinGoalConversions = 10

	// MinReferrerVisitors is the minimum visitors a brand-new source must send
	// before it earns a feed item.
	MinReferrerVisitors = 10

	// MinTrendingVisitors is the minimum visitors a page needs in a day before
	// it can be flagged as trending or newly popular.
	MinTrendingVisitors = 20

	// MinDroppingPageVisitors is the minimum prior-month visitors a page needs
	// before a month-over-month drop is worth surfacing.
	MinDroppingPageVisitors = 50
)

// HourOfWeek returns 0-167 for the current hour-of-week.
// Monday 00:00 = 0, Sunday 23:00 = 167.
// This allows baselines to adapt to weekly patterns (e.g., weekday vs weekend traffic).
func HourOfWeek(t time.Time) int {
	weekday := int(t.Weekday())
	// Convert Sunday=0 to Monday=0 based week
	if weekday == 0 {
		weekday = 6 // Sunday becomes 6
	} else {
		weekday-- // Mon=0, Tue=1, etc.
	}
	return weekday*24 + t.Hour()
}

// SPCIsSpike checks if current value is a statistical spike using z-score.
// Returns (isSpike, severity) where severity is "warning", "critical", or "".
func SPCIsSpike(current, mean, stddev float64) (bool, string) {
	if stddev == 0 {
		stddev = 1.0 // Prevent division by zero
	}

	zScore := (current - mean) / stddev

	if zScore >= CriticalSigma {
		return true, "critical"
	}
	if zScore >= WarningSigma {
		return true, "warning"
	}
	return false, ""
}

// SPCIsDrop checks if current value is a statistical drop using z-score.
// Returns (isDrop, severity) where severity is "warning", "critical", or "".
func SPCIsDrop(current, mean, stddev float64) (bool, string) {
	if stddev == 0 {
		stddev = 1.0 // Prevent division by zero
	}

	zScore := (current - mean) / stddev

	if zScore <= -CriticalSigma {
		return true, "critical"
	}
	if zScore <= -WarningSigma {
		return true, "warning"
	}
	return false, ""
}

// ZScore calculates the z-score for a value given mean and stddev.
func ZScore(current, mean, stddev float64) float64 {
	if stddev == 0 {
		stddev = 1.0
	}
	return (current - mean) / stddev
}
