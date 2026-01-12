package analytics_test

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"

	"fusionaly/internal/analytics"
	"fusionaly/internal/events"
	"fusionaly/internal/timeframe"
	"fusionaly/internal/testsupport"
)

func TestGetUserFlowDataFromAggregates(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()

	// Create some flow transition stats
	now := time.Now().UTC().Truncate(time.Hour)
	flowStats := []analytics.FlowTransitionStat{
		{
			WebsiteID:    1,
			StepPosition: 1,
			SourcePage:   "example.com/home",
			TargetPage:   "example.com/products",
			Transitions:  10,
			Hour:         now,
		},
		{
			WebsiteID:    1,
			StepPosition: 1,
			SourcePage:   "example.com/home",
			TargetPage:   "example.com/about",
			Transitions:  5,
			Hour:         now,
		},
		{
			WebsiteID:    1,
			StepPosition: 2,
			SourcePage:   "example.com/products",
			TargetPage:   "example.com/cart",
			Transitions:  3,
			Hour:         now,
		},
	}
	db.CreateInBatches(flowStats, len(flowStats))

	// Query the flow data
	params := analytics.WebsiteScopedQueryParams{
		WebsiteID: 1,
		TimeFrame: &timeframe.TimeFrame{
			From: now.Add(-time.Hour),
			To:   now.Add(time.Hour),
		},
	}

	results, err := analytics.GetUserFlowData(db, params, 5)
	require.NoError(t, err)
	assert.Len(t, results, 3)

	// Verify the results are properly formatted with step prefixes
	assert.Equal(t, "step1:example.com/home", results[0].Source)
	assert.Equal(t, "step2:example.com/products", results[0].Target)
	assert.Equal(t, int64(10), results[0].Value)
}

func TestGetUserFlowDataFallbackToEvents(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()

	// Clean up any existing data
	testsupport.CleanAllTables(db)

	// Create events but no flow_transition_stats (to test fallback)
	now := time.Now().UTC()
	evts := []events.Event{
		{
			WebsiteID:     1,
			UserSignature: "user1",
			Hostname:      "example.com",
			Pathname:      "/home",
			EventType:     events.EventTypePageView,
			Timestamp:     now,
		},
		{
			WebsiteID:     1,
			UserSignature: "user1",
			Hostname:      "example.com",
			Pathname:      "/products",
			EventType:     events.EventTypePageView,
			Timestamp:     now.Add(time.Minute),
		},
	}
	db.CreateInBatches(evts, len(evts))

	// Query should fallback to events since flow_transition_stats is empty
	params := analytics.WebsiteScopedQueryParams{
		WebsiteID: 1,
		TimeFrame: &timeframe.TimeFrame{
			From: now.Add(-time.Hour),
			To:   now.Add(time.Hour),
		},
	}

	results, err := analytics.GetUserFlowData(db, params, 5)
	require.NoError(t, err)
	// Should have one transition from /home to /products
	assert.Len(t, results, 1)
	assert.Equal(t, "step1:example.com/home", results[0].Source)
	assert.Equal(t, "step2:example.com/products", results[0].Target)
}

func TestComputeFlowTransitionsForHour(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Clean up any existing data
	testsupport.CleanAllTables(db)

	// Create events for testing
	now := time.Now().UTC().Truncate(time.Hour)
	evts := []events.Event{
		{
			WebsiteID:     1,
			UserSignature: "user1",
			Hostname:      "example.com",
			Pathname:      "/page-a",
			EventType:     events.EventTypePageView,
			Timestamp:     now.Add(time.Minute),
		},
		{
			WebsiteID:     1,
			UserSignature: "user1",
			Hostname:      "example.com",
			Pathname:      "/page-b",
			EventType:     events.EventTypePageView,
			Timestamp:     now.Add(2 * time.Minute),
		},
		{
			WebsiteID:     1,
			UserSignature: "user2",
			Hostname:      "example.com",
			Pathname:      "/page-a",
			EventType:     events.EventTypePageView,
			Timestamp:     now.Add(3 * time.Minute),
		},
		{
			WebsiteID:     1,
			UserSignature: "user2",
			Hostname:      "example.com",
			Pathname:      "/page-b",
			EventType:     events.EventTypePageView,
			Timestamp:     now.Add(4 * time.Minute),
		},
	}
	db.CreateInBatches(evts, len(evts))

	// Compute flow transitions
	err := events.ComputeFlowTransitionsForHour(db, logger, now, 5)
	require.NoError(t, err)

	// Verify flow_transition_stats was populated
	var stats []analytics.FlowTransitionStat
	db.Where("website_id = ?", 1).Find(&stats)

	require.Len(t, stats, 1)
	assert.Equal(t, "example.com/page-a", stats[0].SourcePage)
	assert.Equal(t, "example.com/page-b", stats[0].TargetPage)
	assert.Equal(t, 2, stats[0].Transitions) // 2 users made this transition
}
