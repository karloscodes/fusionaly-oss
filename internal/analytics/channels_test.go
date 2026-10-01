package analytics_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/analytics"
	"fusionaly/internal/events"
	"fusionaly/internal/testsupport"
	"fusionaly/internal/timeframe"
)

func TestChannelOf(t *testing.T) {
	t.Run("with no source", func(t *testing.T) {
		assert.Equal(t, "Direct", analytics.ChannelOf(events.DirectOrUnknownReferrer, ""))
		assert.Equal(t, "Direct", analytics.ChannelOf("", events.EmptyUTMAttr))
	})

	t.Run("with a paid utm_medium", func(t *testing.T) {
		for _, medium := range []string{"cpc", "PPC", "paid", "paid_social", "paidsearch", "display", "cpm", "ads"} {
			assert.Equal(t, "Paid", analytics.ChannelOf("google", medium), medium)
		}
	})

	t.Run("with an email utm_medium", func(t *testing.T) {
		assert.Equal(t, "Email", analytics.ChannelOf("weekly", "email"))
		assert.Equal(t, "Email", analytics.ChannelOf("weekly", "Newsletter"))
	})

	t.Run("with another utm_medium", func(t *testing.T) {
		assert.Equal(t, "Social", analytics.ChannelOf("twitter", "social"))
		assert.Equal(t, "Referral", analytics.ChannelOf("partner", "adsense-review"))
	})

	t.Run("with a known source", func(t *testing.T) {
		assert.Equal(t, "Search", analytics.ChannelOf("www.google.com", ""))
		assert.Equal(t, "AI", analytics.ChannelOf("chatgpt.com", ""))
		assert.Equal(t, "Social", analytics.ChannelOf("t.co", ""))
		assert.Equal(t, "Email", analytics.ChannelOf("mail.google.com", ""))
		assert.Equal(t, "Referral", analytics.ChannelOf("news.ycombinator.com", ""))
	})

	t.Run("with an unknown source", func(t *testing.T) {
		assert.Equal(t, "Referral", analytics.ChannelOf("myblog.io", ""))
	})
}

func TestGetTopChannelsInTimeFrame(t *testing.T) {
	dbManager, logger, website := testsupport.SetupTestDBManagerWithWebsite(t, "example.com")
	db := dbManager.GetConnection()
	at := time.Date(2024, 7, 1, 10, 0, 0, 0, time.UTC)
	visits := []struct{ url, referrer string }{
		{"https://example.com/", ""},
		{"https://example.com/", "https://www.google.com/"},
		{"https://example.com/?utm_source=google&utm_medium=cpc", ""},
		{"https://example.com/?utm_source=google&utm_medium=cpc", "https://www.google.com/"},
		{"https://example.com/?utm_source=facebook&utm_medium=paid_social", ""},
		{"https://example.com/", "https://chatgpt.com/"},
		{"https://example.com/", "https://t.co/abc"},
		{"https://example.com/?utm_source=twitter", ""},
		{"https://example.com/?utm_source=weekly&utm_medium=email", ""},
		{"https://example.com/", "https://mail.google.com/"},
		{"https://example.com/", "https://myblog.io/post"},
		{"https://example.com/b", "https://example.com/a"},
	}
	for i, visit := range visits {
		input := testsupport.CreateTestEventInput(fmt.Sprintf("203.0.113.%d", i+1), "Mozilla/5.0 (test)",
			events.EventTypePageView, at, visit.url, visit.referrer, "", "")
		input.ReceivedAt = at
		require.NoError(t, events.CollectEvent(dbManager, logger, input))
	}
	require.NoError(t, testsupport.ProcessAllTestEvents(dbManager, logger))
	timeFrame, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
		FromTime:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
		ToTime:        time.Date(2024, 7, 2, 0, 0, 0, 0, time.UTC),
		TimeFrameSize: timeframe.DailyTimeFrame,
	}, time.UTC)
	require.NoError(t, err)

	results, err := analytics.GetTopChannelsInTimeFrame(db, analytics.NewWebsiteScopedQueryParams(timeFrame, int(website.ID)))

	require.NoError(t, err)
	assert.Equal(t, []analytics.MetricCountResult{
		{Name: "Paid", Count: 3},
		{Name: "Direct", Count: 2},
		{Name: "Email", Count: 2},
		{Name: "Social", Count: 2},
		{Name: "AI", Count: 1},
		{Name: "Referral", Count: 1},
		{Name: "Search", Count: 1},
	}, results)
}
