package analytics

import (
	"time"

	"gorm.io/gorm"

	"fusionaly/internal/timeframe"
)

// DailyVisitorsForWebsites returns, for each website, its visitors per day in
// tz for the `days` full days before now (today is left out, it isn't over
// yet), oldest first. Days without traffic are 0. Home uses it for the site
// cards. It counts like the dashboard: the distinct visitors of each local
// day (see visitors.go).
func DailyVisitorsForWebsites(db *gorm.DB, websiteIDs []uint, days int, now time.Time, tz *time.Location) (map[uint][]int64, error) {
	series := make(map[uint][]int64, len(websiteIDs))
	if len(websiteIDs) == 0 || days <= 0 {
		return series, nil
	}
	if tz == nil {
		tz = time.UTC
	}

	local := now.In(tz)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, tz)
	from := today.AddDate(0, 0, -days)
	tf, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
		FromTime: from.UTC(), ToTime: today.Add(-time.Nanosecond).UTC(), TimeFrameSize: timeframe.DailyTimeFrame,
	}, tz)
	if err != nil {
		return series, err
	}

	// Index the days once: a DST day is 23 or 25 hours, so count calendar days.
	dayIndex := make(map[string]int, days)
	for i := 0; i < days; i++ {
		dayIndex[from.AddDate(0, 0, i).Format(time.DateOnly)] = i
	}

	for _, id := range websiteIDs {
		series[id] = make([]int64, days)
		visitors, err := localDayVisitors(db, int(id), tf)
		if err != nil {
			return series, err
		}
		for day, count := range visitors {
			if i, ok := dayIndex[day]; ok {
				series[id][i] = count
			}
		}
	}
	return series, nil
}
