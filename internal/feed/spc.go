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

// ZScore calculates the z-score for a value given mean and stddev.
func ZScore(current, mean, stddev float64) float64 {
	if stddev == 0 {
		stddev = 1.0
	}
	return (current - mean) / stddev
}
