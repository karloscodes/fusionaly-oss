package analytics

import (
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"

	"fusionaly/internal/timeframe"
)

// Visitors come from site_stats. A visitor is a distinct signature with a
// page view. The signature rotates at midnight UTC, so one person is one
// visitor per UTC day at most.
//
// The rules:
//   - a day counts the distinct visitors of the viewer's local day
//   - a range of several days adds the days ("unique per day, summed")
//   - an hour counts the distinct visitors with a page view in that hour,
//     so a person active at 9:00 and 15:00 counts in both hours
//
// Each bucket of site_stats counts a visitor in three columns: visitors at
// their first page view of the UTC day, last_visitors at their last, and
// hour_visitors at their first page view of each UTC hour. A local day holds
// the end of one UTC day and the start of the next. Its visitors are the
// visitors of the first UTC day whose last page view falls in the local day,
// plus the visitors of the second whose first page view does.

type visitorBucket struct {
	Hour         time.Time
	Visitors     int64
	LastVisitors int64
}

// localDayVisitors returns the visitors of each local day of the range, by
// day (YYYY-MM-DD).
func localDayVisitors(db *gorm.DB, websiteID int, tf *timeframe.TimeFrame) (map[string]int64, error) {
	var buckets []visitorBucket
	err := db.Table("site_stats").
		Select("hour, visitors, last_visitors").
		Where("website_id = ? AND hour BETWEEN ? AND ?", websiteID, tf.From.UTC(), tf.To.UTC()).
		Scan(&buckets).Error
	if err != nil {
		return nil, fmt.Errorf("error fetching visitors: %w", err)
	}

	tz := tf.Tz
	if tz == nil {
		tz = time.UTC
	}
	days := map[string]int64{}
	for _, b := range buckets {
		local := b.Hour.In(tz)
		days[local.Format(time.DateOnly)] += visitorsOfBucket(b, local)
	}
	return days, nil
}

// visitorsOfBucket returns what a bucket adds to the visitors of its local
// day: its last_visitors when it falls before the UTC midnight inside the
// local day, else its visitors.
func visitorsOfBucket(b visitorBucket, local time.Time) int64 {
	dayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location()).UTC()
	startsAtUTCMidnight := dayStart.Equal(dayStart.Truncate(24 * time.Hour))
	beforeUTCMidnight := b.Hour.UTC().Truncate(24 * time.Hour).Equal(dayStart.Truncate(24 * time.Hour))
	if !startsAtUTCMidnight && beforeUTCMidnight {
		return b.LastVisitors
	}
	return b.Visitors
}

// GetTotalVisitorsInTimeFrame returns the visitors of the range: the sum of
// the distinct visitors of each local day.
func GetTotalVisitorsInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) (int64, error) {
	days, err := localDayVisitors(db, params.WebsiteID, params.TimeFrame)
	if err != nil {
		return 0, fmt.Errorf("error calculating total visitors: %w", err)
	}
	var total int64
	for _, visitors := range days {
		total += visitors
	}
	return total, nil
}

// AggregatedVisitorsInTimeFrame returns visitor counts aggregated over a time frame
func AggregatedVisitorsInTimeFrame(db *gorm.DB, params WebsiteScopedQueryParams) ([]timeframe.DateStat, error) {
	result, err := aggregatedVisitorsInTimeFrameRaw(db, params)
	if err != nil {
		return nil, err
	}

	// Build consistent time series with all points including zeros
	return params.TimeFrame.BuildTimeSeriesPoints(result), nil
}

// aggregatedVisitorsInTimeFrameRaw counts distinct visitors per UTC hour for
// hour buckets. Larger buckets add the visitors of their local days.
func aggregatedVisitorsInTimeFrameRaw(db *gorm.DB, params WebsiteScopedQueryParams) ([]timeframe.DateStat, error) {
	tf := params.TimeFrame

	if tf.BucketSize == timeframe.TimeFrameBucketSizeHour {
		hour, err := tf.GroupByExpressionFor("hour")
		if err != nil {
			return nil, err
		}
		var results []timeframe.DateStat
		err = db.Raw(fmt.Sprintf(`
			SELECT %s AS date, SUM(hour_visitors) AS count
			FROM site_stats
			WHERE website_id = ? AND hour BETWEEN ? AND ?
			GROUP BY date
			ORDER BY date ASC
		`, hour), params.WebsiteID, tf.From.UTC(), tf.To.UTC()).Scan(&results).Error
		if err != nil {
			return nil, fmt.Errorf("error fetching aggregated visitors: %w", err)
		}
		return results, nil
	}

	days, err := localDayVisitors(db, params.WebsiteID, tf)
	if err != nil {
		return nil, fmt.Errorf("error fetching aggregated visitors: %w", err)
	}
	counts := map[string]int{}
	for day, visitors := range days {
		key, err := bucketOfLocalDay(day, tf.BucketSize)
		if err != nil {
			return nil, err
		}
		counts[key] += int(visitors)
	}

	results := make([]timeframe.DateStat, 0, len(counts))
	for date, count := range counts {
		results = append(results, timeframe.DateStat{Date: date, Count: count})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Date < results[j].Date })
	return results, nil
}

// bucketOfLocalDay returns the series bucket of a local day (YYYY-MM-DD), in
// the format of timeframe.GroupByExpressionFor.
func bucketOfLocalDay(day string, size timeframe.TimeFrameBucketSize) (string, error) {
	d, err := time.Parse(time.DateOnly, day)
	if err != nil {
		return "", fmt.Errorf("bad local day %q: %w", day, err)
	}
	switch size {
	case timeframe.TimeFrameBucketSizeDay:
		return day, nil
	case timeframe.TimeFrameBucketSizeWeek:
		monday := d.AddDate(0, 0, -((int(d.Weekday()) + 6) % 7))
		return monday.Format(time.DateOnly), nil
	case timeframe.TimeFrameBucketSizeMonth:
		return d.Format("2006-01"), nil
	case timeframe.TimeFrameBucketSizeYear:
		return d.Format("2006"), nil
	default:
		return "", fmt.Errorf("unsupported time frame bucket size: %v", size)
	}
}
