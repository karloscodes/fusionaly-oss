package analytics

// ComparisonMetrics represents period-over-period percentage changes for key metrics
type ComparisonMetrics struct {
	VisitorsChange   *float64 `json:"visitors_change,omitempty"`
	ViewsChange      *float64 `json:"views_change,omitempty"`
	SessionsChange   *float64 `json:"sessions_change,omitempty"`
	BounceRateChange *float64 `json:"bounce_rate_change,omitempty"` // percentage points, not percent
	AvgTimeChange    *float64 `json:"avg_time_change,omitempty"`
	RevenueChange    *float64 `json:"revenue_change,omitempty"`
}

// ComparisonData holds current and previous period metrics for comparison
type ComparisonData struct {
	CurrentVisitors    int64
	PreviousVisitors   int64
	CurrentViews       int64
	PreviousViews      int64
	CurrentSessions    int64
	PreviousSessions   int64
	CurrentBounceRate  float64
	PreviousBounceRate float64
	CurrentAvgTime     float64
	PreviousAvgTime    float64
	CurrentRevenue     float64
	PreviousRevenue    float64
}

// minComparable is the smallest previous count a change is shown for.
// Below it a change is noise: 1 visitor to 3 is not "+200%".
const minComparable = 10

// CalculateComparisonMetrics computes period-over-period percentage changes
func CalculateComparisonMetrics(data ComparisonData) *ComparisonMetrics {
	comparison := &ComparisonMetrics{}

	percentageChange := func(current, previous float64) *float64 {
		change := ((current - previous) / previous) * 100
		return &change
	}

	if data.PreviousVisitors >= minComparable {
		comparison.VisitorsChange = percentageChange(float64(data.CurrentVisitors), float64(data.PreviousVisitors))
	}
	if data.PreviousViews >= minComparable {
		comparison.ViewsChange = percentageChange(float64(data.CurrentViews), float64(data.PreviousViews))
	}
	if data.PreviousSessions >= minComparable {
		comparison.SessionsChange = percentageChange(float64(data.CurrentSessions), float64(data.PreviousSessions))
	}

	// Bounce rate change, in percentage points: 40% -> 46% is +6 points, not
	// +15%. A rate of 0% in a period with visits is still a real rate.
	if data.PreviousSessions >= minComparable && data.CurrentSessions > 0 {
		points := (data.CurrentBounceRate - data.PreviousBounceRate) * 100
		comparison.BounceRateChange = &points
	}

	// Average time is an average over visits: it needs enough of them.
	if data.PreviousSessions >= minComparable && data.PreviousAvgTime > 0 {
		comparison.AvgTimeChange = percentageChange(data.CurrentAvgTime, data.PreviousAvgTime)
	}

	if data.PreviousRevenue > 0 {
		comparison.RevenueChange = percentageChange(data.CurrentRevenue, data.PreviousRevenue)
	}

	return comparison
}
