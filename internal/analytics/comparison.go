package analytics

// ComparisonMetrics represents period-over-period percentage changes for key metrics
type ComparisonMetrics struct {
	VisitorsChange   *float64 `json:"visitors_change,omitempty"`
	ViewsChange      *float64 `json:"views_change,omitempty"`
	SessionsChange   *float64 `json:"sessions_change,omitempty"`
	BounceRateChange *float64 `json:"bounce_rate_change,omitempty"`
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

// CalculateComparisonMetrics computes period-over-period percentage changes
func CalculateComparisonMetrics(data ComparisonData) *ComparisonMetrics {
	comparison := &ComparisonMetrics{}

	// Helper function to calculate percentage change
	calculatePercentageChange := func(current, previous float64) *float64 {
		if previous > 0 {
			change := ((current - previous) / previous) * 100
			return &change
		}
		return nil
	}

	// Visitors change
	if data.PreviousVisitors > 0 {
		comparison.VisitorsChange = calculatePercentageChange(
			float64(data.CurrentVisitors),
			float64(data.PreviousVisitors),
		)
	}

	// Views change
	if data.PreviousViews > 0 {
		comparison.ViewsChange = calculatePercentageChange(
			float64(data.CurrentViews),
			float64(data.PreviousViews),
		)
	}

	// Sessions change
	if data.PreviousSessions > 0 {
		comparison.SessionsChange = calculatePercentageChange(
			float64(data.CurrentSessions),
			float64(data.PreviousSessions),
		)
	}

	// Bounce rate change
	if data.PreviousBounceRate > 0 {
		comparison.BounceRateChange = calculatePercentageChange(
			data.CurrentBounceRate,
			data.PreviousBounceRate,
		)
	}

	// Average time change
	if data.PreviousAvgTime > 0 {
		comparison.AvgTimeChange = calculatePercentageChange(
			data.CurrentAvgTime,
			data.PreviousAvgTime,
		)
	}

	// Revenue change
	if data.PreviousRevenue > 0 {
		comparison.RevenueChange = calculatePercentageChange(
			data.CurrentRevenue,
			data.PreviousRevenue,
		)
	}

	return comparison
}
