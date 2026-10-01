package analytics

import (
	"time"

	"gorm.io/gorm"
)

// DailyVisitorsForWebsites returns, for each website, its visitors per day in
// tz for the `days` full days before now (today is left out, it isn't over
// yet), oldest first. Days without traffic are 0. Home uses it for the site
// cards, so "yesterday" is the same day as on the dashboard.
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

	var rows []struct {
		WebsiteID uint
		Hour      time.Time
		Visitors  int64
	}
	err := db.Table("site_stats").
		Select("website_id, hour, visitors").
		Where("website_id IN ? AND hour >= ? AND hour < ?", websiteIDs, from.UTC(), today.UTC()).
		Scan(&rows).Error
	if err != nil {
		return series, err
	}

	for _, id := range websiteIDs {
		series[id] = make([]int64, days)
	}
	for _, r := range rows {
		bucket := r.Hour.In(tz)
		day := time.Date(bucket.Year(), bucket.Month(), bucket.Day(), 0, 0, 0, 0, tz)
		// Count calendar days, not 24-hour spans: a DST day is 23 or 25 hours.
		i := 0
		for d := from; d.Before(day); d = d.AddDate(0, 0, 1) {
			i++
		}
		if i >= 0 && i < days {
			series[r.WebsiteID][i] += r.Visitors
		}
	}
	return series, nil
}
