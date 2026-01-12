package analytics_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"fusionaly/internal/analytics"
	"fusionaly/internal/timeframe"
	"fusionaly/internal/testsupport"
)

func TestGetTopUTMMediumsInTimeFrame(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()

	// Seed UTMStat directly as we are testing the retrieval from aggregate tables
	testData := []analytics.UTMStat{
		{WebsiteID: 1, UTMMedium: "social", VisitorsCount: 1, Hour: time.Date(2024, 7, 1, 10, 0, 0, 0, time.UTC)},
		{WebsiteID: 1, UTMMedium: "social", VisitorsCount: 1, Hour: time.Date(2024, 7, 1, 11, 0, 0, 0, time.UTC)},
		{WebsiteID: 1, UTMMedium: "email", VisitorsCount: 1, Hour: time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC)},
	}

	db.CreateInBatches(testData, len(testData))

	timeFrame, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
		FromTime:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
		ToTime:        time.Date(2024, 7, 2, 0, 0, 0, 0, time.UTC),
		TimeFrameSize: timeframe.DailyTimeFrame,
	}, time.Local)
	assert.NoError(t, err)

	params := analytics.NewWebsiteScopedQueryParams(timeFrame, 1)
	results, err := analytics.GetTopUTMMediumsInTimeFrame(db, params)
	assert.NoError(t, err)
	assert.Equal(t, []analytics.MetricCountResult{
		{Name: "social", Count: 2},
		{Name: "email", Count: 1},
	}, results)
}

func TestGetTopUTMSourcesInTimeFrame(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()

	testData := []analytics.UTMStat{
		{WebsiteID: 1, UTMSource: "google", VisitorsCount: 1, Hour: time.Date(2024, 7, 1, 10, 0, 0, 0, time.UTC)},
		{WebsiteID: 1, UTMSource: "google", VisitorsCount: 1, Hour: time.Date(2024, 7, 1, 11, 0, 0, 0, time.UTC)},
		{WebsiteID: 1, UTMSource: "facebook", VisitorsCount: 1, Hour: time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC)},
	}

	db.CreateInBatches(testData, len(testData))

	timeFrame, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
		FromTime:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
		ToTime:        time.Date(2024, 7, 2, 0, 0, 0, 0, time.UTC),
		TimeFrameSize: timeframe.DailyTimeFrame,
	}, time.Local)
	assert.NoError(t, err)

	params := analytics.NewWebsiteScopedQueryParams(timeFrame, 1)
	results, err := analytics.GetTopUTMSourcesInTimeFrame(db, params)
	assert.NoError(t, err)
	assert.Equal(t, []analytics.MetricCountResult{
		{Name: "google", Count: 2},
		{Name: "facebook", Count: 1},
	}, results)
}

func TestGetTopUTMCampaignsInTimeFrame(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()

	testData := []analytics.UTMStat{
		{WebsiteID: 1, UTMCampaign: "summer_sale", VisitorsCount: 1, Hour: time.Date(2024, 7, 1, 10, 0, 0, 0, time.UTC)},
		{WebsiteID: 1, UTMCampaign: "summer_sale", VisitorsCount: 1, Hour: time.Date(2024, 7, 1, 11, 0, 0, 0, time.UTC)},
		{WebsiteID: 1, UTMCampaign: "winter_sale", VisitorsCount: 1, Hour: time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC)},
	}

	db.CreateInBatches(testData, len(testData))

	timeFrame, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
		FromTime:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
		ToTime:        time.Date(2024, 7, 2, 0, 0, 0, 0, time.UTC),
		TimeFrameSize: timeframe.DailyTimeFrame,
	}, time.Local)
	assert.NoError(t, err)

	params := analytics.NewWebsiteScopedQueryParams(timeFrame, 1)
	results, err := analytics.GetTopUTMCampaignsInTimeFrame(db, params)
	assert.NoError(t, err)
	assert.Equal(t, []analytics.MetricCountResult{
		{Name: "summer_sale", Count: 2},
		{Name: "winter_sale", Count: 1},
	}, results)
}

func TestGetTopUTMTermsInTimeFrame(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()

	testData := []analytics.UTMStat{
		{WebsiteID: 1, UTMTerm: "shoes", VisitorsCount: 1, Hour: time.Date(2024, 7, 1, 10, 0, 0, 0, time.UTC)},
		{WebsiteID: 1, UTMTerm: "shoes", VisitorsCount: 1, Hour: time.Date(2024, 7, 1, 11, 0, 0, 0, time.UTC)},
		{WebsiteID: 1, UTMTerm: "bags", VisitorsCount: 1, Hour: time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC)},
	}

	db.CreateInBatches(testData, len(testData))

	timeFrame, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
		FromTime:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
		ToTime:        time.Date(2024, 7, 2, 0, 0, 0, 0, time.UTC),
		TimeFrameSize: timeframe.DailyTimeFrame,
	}, time.Local)
	assert.NoError(t, err)

	params := analytics.NewWebsiteScopedQueryParams(timeFrame, 1)
	results, err := analytics.GetTopUTMTermsInTimeFrame(db, params)
	assert.NoError(t, err)
	assert.Equal(t, []analytics.MetricCountResult{
		{Name: "shoes", Count: 2},
		{Name: "bags", Count: 1},
	}, results)
}

func TestGetTopUTMContentsInTimeFrame(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()

	testData := []analytics.UTMStat{
		{WebsiteID: 1, UTMContent: "banner1", VisitorsCount: 1, Hour: time.Date(2024, 7, 1, 10, 0, 0, 0, time.UTC)},
		{WebsiteID: 1, UTMContent: "banner1", VisitorsCount: 1, Hour: time.Date(2024, 7, 1, 11, 0, 0, 0, time.UTC)},
		{WebsiteID: 1, UTMContent: "banner2", VisitorsCount: 1, Hour: time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC)},
	}

	db.CreateInBatches(testData, len(testData))

	timeFrame, err := timeframe.NewTimeFrame(timeframe.TimeFrameParams{
		FromTime:      time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
		ToTime:        time.Date(2024, 7, 2, 0, 0, 0, 0, time.UTC),
		TimeFrameSize: timeframe.DailyTimeFrame,
	}, time.Local)
	assert.NoError(t, err)

	params := analytics.NewWebsiteScopedQueryParams(timeFrame, 1)
	results, err := analytics.GetTopUTMContentsInTimeFrame(db, params)
	assert.NoError(t, err)
	assert.Equal(t, []analytics.MetricCountResult{
		{Name: "banner1", Count: 2},
		{Name: "banner2", Count: 1},
	}, results)
}
