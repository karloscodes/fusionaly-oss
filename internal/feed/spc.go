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
