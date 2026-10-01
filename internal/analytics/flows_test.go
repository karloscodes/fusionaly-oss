package analytics_test

import (
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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

// flowDay is a fixed UTC day, so no test depends on the wall clock.
var flowDay = time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)

func at(clock string) time.Time {
	d, err := time.ParseDuration(clock)
	if err != nil {
		panic(err)
	}
	return flowDay.Add(d)
}

func pageView(user, path string, ts time.Time) events.Event {
	return events.Event{WebsiteID: 1, UserSignature: user, Hostname: "example.com", Pathname: path, EventType: events.EventTypePageView, Timestamp: ts}
}

func customEvent(user string, ts time.Time) events.Event {
	return events.Event{WebsiteID: 1, UserSignature: user, Hostname: "example.com", Pathname: "/", EventType: events.EventTypeCustomEvent, CustomEventName: "signup", Timestamp: ts}
}

func setupFlowDB(t *testing.T, evts ...events.Event) *gorm.DB {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	testsupport.CleanAllTables(db)
	if len(evts) > 0 {
		require.NoError(t, db.Create(&evts).Error)
	}
	return db
}

func flowLinks(t *testing.T, db *gorm.DB, from, to time.Time) []string {
	params := analytics.WebsiteScopedQueryParams{WebsiteID: 1, TimeFrame: &timeframe.TimeFrame{From: from, To: to}}
	results, err := analytics.GetUserFlowDataFromEvents(db, params, 5)
	require.NoError(t, err)
	links := make([]string, 0, len(results))
	for _, r := range results {
		links = append(links, fmt.Sprintf("%s -> %s = %d", r.Source, r.Target, r.Value))
	}
	return links
}

func storedFlows(t *testing.T, db *gorm.DB) []string {
	var stats []analytics.FlowTransitionStat
	require.NoError(t, db.Order("hour, step_position, source_page, target_page").Find(&stats).Error)
	rows := make([]string, 0, len(stats))
	for _, s := range stats {
		rows = append(rows, fmt.Sprintf("%s step%d %s -> %s = %d", s.Hour.UTC().Format("15:04"), s.StepPosition, s.SourcePage, s.TargetPage, s.Transitions))
	}
	return rows
}

func TestUserFlowsFollowVisits(t *testing.T) {
	t.Run("a visit that crosses an hour boundary keeps its move", func(t *testing.T) {
		db := setupFlowDB(t,
			pageView("u1", "/home", at("11h59m")),
			pageView("u1", "/products", at("12h01m")),
		)

		links := flowLinks(t, db, at("0h"), at("24h"))

		assert.Equal(t, []string{"step1:example.com/home -> step2:example.com/products = 1"}, links)
	})

	t.Run("steps count from the start of the visit, also before the range", func(t *testing.T) {
		db := setupFlowDB(t,
			pageView("u1", "/home", at("10h50m")),
			pageView("u1", "/products", at("10h55m")),
			pageView("u1", "/cart", at("11h05m")),
		)

		links := flowLinks(t, db, at("11h"), at("12h"))

		assert.Equal(t, []string{"step2:example.com/products -> step3:example.com/cart = 1"}, links)
	})

	t.Run("a gap longer than the session timeout starts a new visit", func(t *testing.T) {
		db := setupFlowDB(t,
			pageView("u1", "/home", at("10h00m")),
			pageView("u1", "/products", at("10h31m")),
		)

		links := flowLinks(t, db, at("0h"), at("24h"))

		assert.Empty(t, links)
	})

	t.Run("a custom event keeps the visit going", func(t *testing.T) {
		db := setupFlowDB(t,
			pageView("u1", "/home", at("10h00m")),
			customEvent("u1", at("10h25m")),
			pageView("u1", "/products", at("10h50m")),
		)

		links := flowLinks(t, db, at("0h"), at("24h"))

		assert.Equal(t, []string{"step1:example.com/home -> step2:example.com/products = 1"}, links)
	})

	t.Run("a reload is not a step", func(t *testing.T) {
		db := setupFlowDB(t,
			pageView("u1", "/home", at("10h00m")),
			pageView("u1", "/home", at("10h01m")),
			pageView("u1", "/products", at("10h02m")),
		)

		links := flowLinks(t, db, at("0h"), at("24h"))

		assert.Equal(t, []string{"step1:example.com/home -> step2:example.com/products = 1"}, links)
	})

	t.Run("other websites do not count", func(t *testing.T) {
		other := pageView("u1", "/products", at("10h01m"))
		other.WebsiteID = 2
		db := setupFlowDB(t,
			pageView("u1", "/home", at("10h00m")),
			other,
		)

		links := flowLinks(t, db, at("0h"), at("24h"))

		assert.Empty(t, links)
	})
}

func TestComputeFlowTransitionsForHour(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("stores each move in the hour of its target page view", func(t *testing.T) {
		db := setupFlowDB(t,
			pageView("u1", "/a", at("11h59m")),
			pageView("u1", "/b", at("12h01m")),
			pageView("u2", "/a", at("12h10m")),
			pageView("u2", "/b", at("12h11m")),
		)

		require.NoError(t, events.ComputeFlowTransitionsForHour(db, logger, at("11h"), 5))
		require.NoError(t, events.ComputeFlowTransitionsForHour(db, logger, at("12h"), 5))

		assert.Equal(t, []string{"12:00 step1 example.com/a -> example.com/b = 2"}, storedFlows(t, db))
	})

	t.Run("a recompute replaces the hour's rows", func(t *testing.T) {
		db := setupFlowDB(t,
			pageView("u1", "/a", at("12h00m")),
			pageView("u1", "/b", at("12h01m")),
		)
		require.NoError(t, events.ComputeFlowTransitionsForHour(db, logger, at("12h"), 5))
		require.NoError(t, db.Where("pathname = ?", "/b").Delete(&events.Event{}).Error)

		require.NoError(t, events.ComputeFlowTransitionsForHour(db, logger, at("12h"), 5))
		require.NoError(t, events.ComputeFlowTransitionsForHour(db, logger, at("12h"), 5))

		assert.Empty(t, storedFlows(t, db))
	})

	t.Run("stored flows match the flows computed from events", func(t *testing.T) {
		db := setupFlowDB(t,
			pageView("u1", "/a", at("09h50m")),
			pageView("u1", "/b", at("10h05m")),
			pageView("u1", "/c", at("10h20m")),
			pageView("u2", "/a", at("10h40m")),
			pageView("u2", "/c", at("11h10m")),
		)
		for h := 0; h < 24; h++ {
			require.NoError(t, events.ComputeFlowTransitionsForHour(db, logger, flowDay.Add(time.Duration(h)*time.Hour), 5))
		}
		params := analytics.WebsiteScopedQueryParams{WebsiteID: 1, TimeFrame: &timeframe.TimeFrame{From: at("0h"), To: at("24h")}}

		stored, err := analytics.GetUserFlowData(db, params, 5)
		require.NoError(t, err)

		computed, err := analytics.GetUserFlowDataFromEvents(db, params, 5)
		require.NoError(t, err)
		assert.ElementsMatch(t, computed, stored)
		assert.Len(t, stored, 3)
	})
}
