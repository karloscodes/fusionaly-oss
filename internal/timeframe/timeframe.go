package timeframe

import (
	"fmt"
	"strings"
	"time"
)

type DateStat struct {
	Date  string
	Count int
}

// TimeFrame structs
type TimeFrameBucketSize string

const (
	TimeFrameBucketSizeYear  TimeFrameBucketSize = "year"
	TimeFrameBucketSizeMonth TimeFrameBucketSize = "month"
	TimeFrameBucketSizeWeek  TimeFrameBucketSize = "week"
	TimeFrameBucketSizeDay   TimeFrameBucketSize = "day"
	TimeFrameBucketSizeHour  TimeFrameBucketSize = "hour"
)

// TimeFrameRangeLabel represents the available time range options
type TimeFrameRangeLabel string

const (
	TimeFrameRangeLabelToday        TimeFrameRangeLabel = "today"
	TimeFrameRangeLabelYesterday    TimeFrameRangeLabel = "yesterday"
	TimeFrameRangeLabelLast7Days    TimeFrameRangeLabel = "last_7_days"
	TimeFrameRangeLabelLast30Days   TimeFrameRangeLabel = "last_30_days"
	TimeFrameRangeLabelMonthToDate  TimeFrameRangeLabel = "month_to_date"
	TimeFrameRangeLabelLastMonth    TimeFrameRangeLabel = "last_month"
	TimeFrameRangeLabelYearToDate   TimeFrameRangeLabel = "year_to_date"
	TimeFrameRangeLabelLast12Months TimeFrameRangeLabel = "last_12_months"
	TimeFrameRangeLabelAllTime      TimeFrameRangeLabel = "all_time"
	TimeFrameRangeLabelCustom       TimeFrameRangeLabel = "custom"
)

type TimeProvider interface {
	Now(loc *time.Location) time.Time
}

// DefaultTimeProvider is the default implementation that uses the system clock
// without any stabilization delay, as we now use TimeWindowBuffer directly
type DefaultTimeProvider struct{}

// Now returns the current time without any adjustments
// This is simpler than the previous approach with StabilizationDelay
func (p *DefaultTimeProvider) Now(loc *time.Location) time.Time {
	return time.Now().In(loc)
}

type TimeFrameSize struct {
	DBFormat   string
	BucketSize TimeFrameBucketSize
}

type TimeFrameParams struct {
	FromTime      time.Time
	ToTime        time.Time
	TimeFrameSize TimeFrameSize
}

// TimeFrame represents a period between two points in time
type TimeFrame struct {
	From       time.Time
	To         time.Time
	Label      TimeFrameRangeLabel
	BucketSize TimeFrameBucketSize
	dbFormat   string // private field for internal use
	Tz         *time.Location
}

type DatePointsOfReference struct {
	SQLiteBucketTimeFormat string
	UserFacingTimeFormat   string
}

// Predefined TimeFrameSizes
var (
	HourlyTimeFrame  = TimeFrameSize{DBFormat: "%Y-%m-%d %H:00:00", BucketSize: TimeFrameBucketSizeHour}
	DailyTimeFrame   = TimeFrameSize{DBFormat: "%Y-%m-%d", BucketSize: TimeFrameBucketSizeDay}
	WeeklyTimeFrame  = TimeFrameSize{DBFormat: "%Y-%m-%d", BucketSize: TimeFrameBucketSizeWeek}
	MonthlyTimeFrame = TimeFrameSize{DBFormat: "%Y-%m-01", BucketSize: TimeFrameBucketSizeMonth}
	YearlyTimeFrame  = TimeFrameSize{DBFormat: "%Y", BucketSize: TimeFrameBucketSizeYear}
)

func NewTimeFrame(params TimeFrameParams, tz *time.Location) (*TimeFrame, error) {
	if params.FromTime.After(params.ToTime) {
		return nil, fmt.Errorf("fromTime must be before toTime")
	}
	return &TimeFrame{
		From:       params.FromTime,
		To:         params.ToTime,
		BucketSize: params.TimeFrameSize.BucketSize,
		dbFormat:   params.TimeFrameSize.DBFormat,
		Tz:         tz,
	}, nil
}

func NewAutoTimeFrameFromClientTimezone(fromTime, toTime time.Time, tz *time.Location) (*TimeFrame, error) {
	fromUTC := fromTime.UTC()
	toUTC := toTime.UTC()

	timeFrameSize := GetAppropriateTimeFrameSize(fromUTC, toUTC)

	// For completed periods, apply bucket truncation respecting timezone boundaries
	// This prevents issues where UTC truncation crosses user timezone day boundaries
	toTruncated := TruncateToBucketInTimezone(toTime, timeFrameSize.BucketSize, tz)

	switch timeFrameSize.BucketSize {
	case TimeFrameBucketSizeYear:
		toTruncated = toTruncated.AddDate(1, 0, 0).Add(-1 * time.Second)
	case TimeFrameBucketSizeMonth:
		toTruncated = toTruncated.AddDate(0, 1, 0).Add(-1 * time.Second)
	case TimeFrameBucketSizeWeek:
		toTruncated = toTruncated.AddDate(0, 0, 7).Add(-1 * time.Second)
	case TimeFrameBucketSizeDay:
		toTruncated = toTruncated.AddDate(0, 0, 1).Add(-1 * time.Second)
	case TimeFrameBucketSizeHour:
		toTruncated = toTruncated.Add(time.Hour).Add(-1 * time.Second)
	}

	return NewTimeFrame(TimeFrameParams{
		FromTime:      fromUTC,
		ToTime:        toTruncated.UTC(), // Convert back to UTC for internal storage
		TimeFrameSize: timeFrameSize,
	}, tz)
}

func GetAppropriateTimeFrameSize(fromTime, toTime time.Time) TimeFrameSize {
	days := toTime.Sub(fromTime).Hours() / 24

	switch {
	case days >= 5*365:
		return YearlyTimeFrame
	case days >= 3*30:
		return MonthlyTimeFrame
	case days >= 30:
		return DailyTimeFrame
	case days >= 2:
		return DailyTimeFrame
	default:
		return HourlyTimeFrame
	}
}

func (tf *TimeFrame) FormatDate(t time.Time) string {
	return t.Format(tf.sqliteToGoFormat())
}

func (tf *TimeFrame) sqliteToGoFormat() string {
	format := tf.dbFormat
	format = strings.ReplaceAll(format, "%Y", "2006")
	format = strings.ReplaceAll(format, "%m", "01")
	format = strings.ReplaceAll(format, "%d", "02")
	format = strings.ReplaceAll(format, "%H", "15")
	format = strings.ReplaceAll(format, "%W", "02") // Week of year
	return format
}

func (tf *TimeFrame) Duration() time.Duration {
	return tf.To.Sub(tf.From)
}

func (tf *TimeFrame) Validate() error {
	if tf.From.After(tf.To) {
		return fmt.Errorf("fromTime must be before toTime")
	}
	return nil
}

func GetTimeFrameSize(bucketSize TimeFrameBucketSize) (TimeFrameSize, error) {
	switch bucketSize {
	case TimeFrameBucketSizeHour:
		return HourlyTimeFrame, nil
	case TimeFrameBucketSizeDay:
		return DailyTimeFrame, nil
	case TimeFrameBucketSizeWeek:
		return WeeklyTimeFrame, nil
	case TimeFrameBucketSizeMonth:
		return MonthlyTimeFrame, nil
	case TimeFrameBucketSizeYear:
		return YearlyTimeFrame, nil
	default:
		return TimeFrameSize{}, fmt.Errorf("unknown bucket size: %s", bucketSize)
	}
}

func (tf *TimeFrame) GetDBFormat() string {
	switch tf.BucketSize {
	case TimeFrameBucketSizeHour:
		return "%Y-%m-%d %H:00:00"
	case TimeFrameBucketSizeDay:
		return "%Y-%m-%d"
	case TimeFrameBucketSizeWeek:
		return "%Y-%m-%d"
	case TimeFrameBucketSizeMonth:
		return "%Y-%m-01"
	case TimeFrameBucketSizeYear:
		return "%Y"
	default:
		return "%Y-%m-%d"
	}
}

// GetSQLiteGroupByExpression returns the SQLite expression to use for grouping events based on the time frame's bucket size.
func (tf *TimeFrame) GetSQLiteGroupByExpression() (string, error) {
	switch tf.BucketSize {
	case TimeFrameBucketSizeHour:
		// Use consistent format YYYY-MM-DD HH (to match existing tests)
		return "strftime('%Y-%m-%d %H', hour)", nil
	case TimeFrameBucketSizeDay:
		// Use consistent format YYYY-MM-DD
		return "strftime('%Y-%m-%d', hour)", nil
	case TimeFrameBucketSizeWeek:
		// Use consistent format YYYY-MM-DD for week start
		return "date(hour, 'start of day', '-' || ((strftime('%w', hour) + 6) % 7) || ' days')", nil
	case TimeFrameBucketSizeMonth:
		// Use consistent format YYYY-MM
		return "strftime('%Y-%m', hour)", nil
	case TimeFrameBucketSizeYear:
		// Use consistent format YYYY
		return "strftime('%Y', hour)", nil
	default:
		return "", fmt.Errorf("unsupported time frame bucket size: %v", tf.BucketSize)
	}
}

func (tf *TimeFrame) GenerateDateTimePointsReference() []DatePointsOfReference {
	datePoints := []DatePointsOfReference{}

	// Timezone handling: All date points are generated with UTC midnight times
	// to ensure consistent display across timezones. The frontend handles
	// any necessary timezone conversions for display purposes.

	// Start from UTC times directly - no timezone conversion needed
	currentTime := tf.From
	endTime := tf.To

	// IMPORTANT: For daily/monthly/yearly buckets, we need to work in the USER's timezone
	// because tf.From might be in a different day when expressed in UTC
	// Example: CET user requesting Dec 1 → From = Nov 30 23:00 UTC (Dec 1 00:00 CET)

	// Get timezone, fallback to UTC
	tz := tf.Tz
	if tz == nil {
		tz = time.UTC
	}

	// For non-hourly buckets, start from the date in user timezone
	if tf.BucketSize != TimeFrameBucketSizeHour {
		// Convert UTC time to user timezone to get the correct starting date
		localTime := currentTime.In(tz)
		// Create midnight UTC for that DATE (not midnight in user timezone!)
		// This ensures "Dec 1" always displays as "2025-12-01T00:00:00Z" regardless of timezone
		currentTime = time.Date(localTime.Year(), localTime.Month(), localTime.Day(), 0, 0, 0, 0, time.UTC)

		// DON'T adjust endTime - it's already correct from the parser
		// Adjusting it can create extra buckets when crossing timezone boundaries
	} else {
		// For hourly buckets, truncate to hour boundary in UTC
		currentTime = truncateToBucket(currentTime, tf.BucketSize)
	}

	// Set a reasonable maximum number of points to prevent infinite loops
	maxPoints := 1000
	pointCount := 0

	for {
		// Safety check to prevent infinite loops
		if pointCount >= maxPoints {
			break
		}

		// Stop if we've gone past the end time
		// For daily/monthly/yearly buckets, compare bucket boundaries, not exact times
		// This ensures we include a bucket for the day/month/year containing endTime
		shouldStop := false
		switch tf.BucketSize {
		case TimeFrameBucketSizeDay:
			// Include bucket if currentTime is on or before the day containing endTime
			currentDay := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 0, 0, 0, 0, time.UTC)
			endDay := time.Date(endTime.Year(), endTime.Month(), endTime.Day(), 0, 0, 0, 0, time.UTC)
			shouldStop = currentDay.After(endDay)
		case TimeFrameBucketSizeMonth:
			// Include bucket if currentTime is on or before the month containing endTime
			currentMonth := time.Date(currentTime.Year(), currentTime.Month(), 1, 0, 0, 0, 0, time.UTC)
			endMonth := time.Date(endTime.Year(), endTime.Month(), 1, 0, 0, 0, 0, time.UTC)
			shouldStop = currentMonth.After(endMonth)
		case TimeFrameBucketSizeYear:
			// Include bucket if currentTime is on or before the year containing endTime
			shouldStop = currentTime.Year() > endTime.Year()
		default:
			// For hour and week buckets, use exact time comparison
			shouldStop = currentTime.After(endTime)
		}

		if shouldStop {
			break
		}

		// Format the time according to bucket size
		// IMPORTANT: currentTime is already normalized to represent the display date
		// For daily+ buckets, it's the date at midnight UTC
		// For hourly buckets, it's the hour boundary in UTC
		var sqliteBucketFormat string
		var displayTime time.Time

		switch tf.BucketSize {
		case TimeFrameBucketSizeYear:
			sqliteBucketFormat = currentTime.Format("2006")
			// currentTime is already Jan 1 midnight UTC
			displayTime = currentTime
		case TimeFrameBucketSizeMonth:
			sqliteBucketFormat = currentTime.Format("2006-01")
			// currentTime is already 1st of month midnight UTC
			displayTime = currentTime
		case TimeFrameBucketSizeWeek:
			sqliteBucketFormat = currentTime.Format("2006-01-02")
			// currentTime is already the week start date at midnight UTC
			displayTime = currentTime
		case TimeFrameBucketSizeDay:
			sqliteBucketFormat = currentTime.Format("2006-01-02")
			// currentTime is already the date at midnight UTC
			displayTime = currentTime
		case TimeFrameBucketSizeHour:
			// Match the expected format in the tests: YYYY-MM-DD HH
			sqliteBucketFormat = currentTime.Format("2006-01-02 15")
			// currentTime is already at hour boundary
			displayTime = currentTime
		}

		// Return dates using displayTime which represents the bucket in a timezone-safe way
		datePoints = append(datePoints, DatePointsOfReference{
			SQLiteBucketTimeFormat: sqliteBucketFormat,
			UserFacingTimeFormat:   displayTime.Format(time.RFC3339),
		})

		// Increment the time for the next bucket
		switch tf.BucketSize {
		case TimeFrameBucketSizeYear:
			currentTime = currentTime.AddDate(1, 0, 0)
		case TimeFrameBucketSizeMonth:
			currentTime = currentTime.AddDate(0, 1, 0)
		case TimeFrameBucketSizeWeek:
			currentTime = currentTime.AddDate(0, 0, 7)
		case TimeFrameBucketSizeDay:
			currentTime = currentTime.AddDate(0, 0, 1)
		case TimeFrameBucketSizeHour:
			currentTime = currentTime.Add(time.Hour)
		}

		pointCount++
	}

	return datePoints
}

// TruncateToBucketInTimezone truncates a time to the appropriate bucket boundary in the given timezone
func TruncateToBucketInTimezone(t time.Time, bucketSize TimeFrameBucketSize, loc *time.Location) time.Time {
	// Ensure we're working in the correct timezone
	localTime := t.In(loc)
	year, month, day := localTime.Year(), localTime.Month(), localTime.Day()

	switch bucketSize {
	case TimeFrameBucketSizeYear:
		return time.Date(year, 1, 1, 0, 0, 0, 0, loc)
	case TimeFrameBucketSizeMonth:
		return time.Date(year, month, 1, 0, 0, 0, 0, loc)
	case TimeFrameBucketSizeWeek:
		weekday := int(localTime.Weekday())
		if weekday == 0 { // Sunday
			weekday = 7
		}
		daysToSubtract := weekday - 1
		return time.Date(year, month, day-daysToSubtract, 0, 0, 0, 0, loc)
	case TimeFrameBucketSizeDay:
		return time.Date(year, month, day, 0, 0, 0, 0, loc)
	case TimeFrameBucketSizeHour:
		return time.Date(year, month, day, localTime.Hour(), 0, 0, 0, loc)
	default:
		return localTime
	}
}

func (tf *TimeFrame) BuildTimeSeriesPoints(groupedResults []DateStat) []DateStat {
	dateLabels := tf.GenerateDateTimePointsReference()
	results := make([]DateStat, len(dateLabels))

	// Create maps for date lookup - normalize formats based on bucket size
	resultsMap := make(map[string]int, len(groupedResults))

	// Process all database results and add to lookup map with normalized keys
	for _, result := range groupedResults {
		// Normalize the date format based on bucket size for consistent matching
		normalizedKey := tf.normalizeDBDateFormat(result.Date)
		resultsMap[normalizedKey] = result.Count
	}

	// Build the final time series with all expected points
	for i, datePoint := range dateLabels {
		// The SQLiteBucketTimeFormat matches the database format used in queries
		normalizedKey := tf.normalizeDBDateFormat(datePoint.SQLiteBucketTimeFormat)
		count := 0

		// Look up the count for this point
		if val, exists := resultsMap[normalizedKey]; exists {
			count = val
		}

		// Add the point with the user-facing time format
		results[i] = DateStat{
			Date:  datePoint.UserFacingTimeFormat,
			Count: count,
		}
	}

	return results
}

// normalizeDBDateFormat standardizes date formats for consistent lookups
func (tf *TimeFrame) normalizeDBDateFormat(dateStr string) string {
	switch tf.BucketSize {
	case TimeFrameBucketSizeHour:
		// For hourly data, we standardize to YYYY-MM-DD HH format
		if len(dateStr) >= 13 {
			return dateStr[:13] // Keep YYYY-MM-DD HH
		}
	case TimeFrameBucketSizeDay, TimeFrameBucketSizeWeek:
		// For daily/weekly data, we keep only YYYY-MM-DD
		if len(dateStr) >= 10 {
			return dateStr[:10]
		}
	case TimeFrameBucketSizeMonth:
		// For monthly data, we keep only YYYY-MM
		if len(dateStr) >= 7 {
			return dateStr[:7]
		}
	case TimeFrameBucketSizeYear:
		// For yearly data, we keep only YYYY
		if len(dateStr) >= 4 {
			return dateStr[:4]
		}
	}
	// If we can't normalize, return as is
	return dateStr
}

func (tf *TimeFrame) GetSQLiteFormat() string {
	switch tf.BucketSize {
	case TimeFrameBucketSizeHour:
		return "2006-01-02 15:00:00"
	case TimeFrameBucketSizeDay:
		return "2006-01-02"
	case TimeFrameBucketSizeWeek:
		return "2006-01-02"
	case TimeFrameBucketSizeMonth:
		return "2006-01-01"
	case TimeFrameBucketSizeYear:
		return "2006"
	default:
		return "2006-01-02"
	}
}

func (tf *TimeFrame) GetUserFormat() string {
	switch tf.BucketSize {
	case TimeFrameBucketSizeHour:
		return "2006-01-02T15:00:00Z"
	case TimeFrameBucketSizeDay:
		return "2006-01-02"
	case TimeFrameBucketSizeWeek:
		return "2006-01-02"
	case TimeFrameBucketSizeMonth:
		return "Jan 2006"
	case TimeFrameBucketSizeYear:
		return "2006"
	default:
		return "2006-01-02"
	}
}

func (tf *TimeFrame) GenerateHumanReadableDateTimeLabels() []string {
	datePoints := tf.GenerateDateTimePointsReference()
	labels := make([]string, len(datePoints))

	for i, point := range datePoints {
		// Parse the time in UTC
		t, err := time.Parse(time.RFC3339, point.UserFacingTimeFormat)
		if err != nil {
			labels[i] = point.UserFacingTimeFormat
			continue
		}
		// Keep the time in UTC for the label format
		labels[i] = t.Format(tf.GetUserFormat())
	}

	return labels
}

func (tf *TimeFrame) CalculateTrend(points []DateStat) float64 {
	if len(points) < 2 {
		return 0
	}

	var sumX, sumY, sumXY, sumXX float64
	n := float64(len(points))

	for i, point := range points {
		x := float64(i)
		y := float64(point.Count)

		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumXX - sumX*sumX)
	return slope
}

func truncateToBucket(t time.Time, bucketSize TimeFrameBucketSize) time.Time {
	utc := t.UTC()
	year, month, day := utc.Year(), utc.Month(), utc.Day()

	switch bucketSize {
	case TimeFrameBucketSizeYear:
		return time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	case TimeFrameBucketSizeMonth:
		return time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	case TimeFrameBucketSizeWeek:
		weekday := int(utc.Weekday())
		if weekday == 0 { // Sunday
			weekday = 7
		}
		daysToSubtract := weekday - 1
		return time.Date(year, month, day-daysToSubtract, 0, 0, 0, 0, time.UTC)
	case TimeFrameBucketSizeDay:
		return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	case TimeFrameBucketSizeHour:
		return time.Date(year, month, day, utc.Hour(), 0, 0, 0, time.UTC)
	default:
		return utc
	}
}
