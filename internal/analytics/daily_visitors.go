package analytics

import (
	"time"

	"gorm.io/gorm"
)

// DailyVisitorsForWebsites returns, for each website, its visitors per UTC day
// for the `days` full days before now (today is left out, it isn't over yet),
// oldest first. Days without traffic are 0. Home uses it for the site cards.
func DailyVisitorsForWebsites(db *gorm.DB, websiteIDs []uint, days int, now time.Time) (map[uint][]int64, error) {
	series := make(map[uint][]int64, len(websiteIDs))
	if len(websiteIDs) == 0 || days <= 0 {
		return series, nil
	}

	today := now.UTC().Truncate(24 * time.Hour)
	from := today.AddDate(0, 0, -days)

	var rows []struct {
		WebsiteID uint
		Day       string
		Visitors  int64
	}
	err := db.Table("site_stats").
		Select("website_id, DATE(hour) AS day, SUM(visitors) AS visitors").
		Where("website_id IN ? AND hour >= ? AND hour < ?", websiteIDs, from, today).
		Group("website_id, DATE(hour)").
		Scan(&rows).Error
	if err != nil {
		return series, err
	}

	for _, id := range websiteIDs {
		series[id] = make([]int64, days)
	}
	for _, r := range rows {
		day, err := time.Parse("2006-01-02", r.Day)
		if err != nil {
			continue
		}
		i := int(day.Sub(from).Hours() / 24)
		if i >= 0 && i < days {
			series[r.WebsiteID][i] += r.Visitors
		}
	}
	return series, nil
}
