package timeframe

import (
	"fmt"
	"time"
)

// TimeWindowBuffer defines a unified approach to ensure we capture all relevant data points
// when creating time frames. This single 5-minute buffer:
//  1. Compensates for clock synchronization issues between servers
//  2. Accounts for network latency when events are recorded
//  3. Ensures events that are slightly delayed in processing are still included
//  4. Prevents data loss at time frame boundaries
//
// This replaces the previous dual approach of DataStabilizationDelay and end-time buffer.
const TimeWindowBuffer = 5 * time.Minute

type TimeFrameParserParams struct {
	FromDate            string
	ToDate              string
	Tz                  string
	AllTimeFirstEventAt time.Time
}

type TimeFrameParser struct {
	timeProvider TimeProvider
}

func NewTimeFrameParser(timeProvider ...TimeProvider) *TimeFrameParser {
	var provider TimeProvider = &DefaultTimeProvider{}
	if len(timeProvider) > 0 && timeProvider[0] != nil {
		provider = timeProvider[0]
	}

	return &TimeFrameParser{
		timeProvider: provider,
	}
}

func (p *TimeFrameParser) ParseTimeFrame(params TimeFrameParserParams) (*TimeFrame, error) {
	// Load the timezone once at the top
	loc, err := time.LoadLocation(params.Tz)
	if err != nil {
		return nil, fmt.Errorf("error loading timezone: %w", err)
	}

	// Parse from/to dates - always use custom date range parsing
	// No more range parameter support - everything is based on explicit from/to dates
	from, to, err := p.parseCustomDateRange(params)
	if err != nil {
		return nil, err
	}

	// Create TimeFrame - convert user timezone dates to UTC for internal storage
	return NewAutoTimeFrameFromClientTimezone(from, to, loc)
}

func (p *TimeFrameParser) parseCustomDateRange(params TimeFrameParserParams) (time.Time, time.Time, error) {
	loc, err := time.LoadLocation(params.Tz)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("error loading timezone: %w", err)
	}
	now := p.timeProvider.Now(loc)

	// Option 1: Provide sensible defaults when no dates are provided
	// Default to last 7 days if no dates are specified
	defaultFrom := now.Truncate(24*time.Hour).AddDate(0, 0, -30)
	defaultTo := now

	from, err := p.parseDateWithDefault(params.FromDate, defaultFrom, loc, false)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid 'from' date: %w", err)
	}

	to, err := p.parseDateWithDefault(params.ToDate, defaultTo, loc, true)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid 'to' date: %w", err)
	}

	return from, to, nil
}

func (p *TimeFrameParser) parseDateWithDefault(dateStr string, defaultDate time.Time, loc *time.Location, isEndDate bool) (time.Time, error) {
	if dateStr == "" {
		return defaultDate, nil
	}

	date, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		return time.Time{}, err
	}

	// Use stable now for today comparison
	now := p.timeProvider.Now(loc)
	isToday := date.Year() == now.Year() && date.Month() == now.Month() && date.Day() == now.Day()

	if isToday && isEndDate {
		// For "today" end dates, we want to include current data but prevent spilling into tomorrow
		// The key insight: if this will be used with daily/weekly/monthly buckets, we should not
		// cross the day boundary in the user's timezone to prevent future dates from appearing

		// Calculate end of the requested date in user's timezone
		endOfRequestedDate := time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 59, 999999999, loc)

		// Apply buffer but ensure we don't go beyond the requested date boundary
		bufferedTime := now.Add(TimeWindowBuffer)
		if bufferedTime.After(endOfRequestedDate) {
			// Clamp to end of the requested date to prevent spillover to next day
			return endOfRequestedDate, nil
		}
		return bufferedTime, nil
	}

	// Ensure we don't exceed stable now
	if isEndDate {
		endOfDay := time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 59, 999999999, loc)
		if endOfDay.After(now) {
			// Apply TimeWindowBuffer for dates in the future, but respect date boundaries
			bufferedTime := now.Add(TimeWindowBuffer)
			if bufferedTime.After(endOfDay) {
				return endOfDay, nil
			}
			return bufferedTime, nil
		}
		return endOfDay, nil
	}

	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc), nil
}
