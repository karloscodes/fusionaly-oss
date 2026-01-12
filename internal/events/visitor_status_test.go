package events_test

import (
	"io"
	"net/url"
	"testing"
	"time"

	"fusionaly/internal/visitors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
	"gorm.io/gorm"

	"fusionaly/internal/events"
	"fusionaly/internal/testsupport"
)

func TestVisitorStatusFunctions(t *testing.T) {
	// Set up test database with clean slate
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Clean up existing events and create website directly
	testsupport.CleanTables(db, []string{"events", "ingested_events"})

	website := testsupport.CreateTestWebsite(db, "example.com")

	// Base time for all events
	baseTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

	// User signatures (generated using the helper for consistency)
	user1Input := &events.CollectEventInput{
		IPAddress: "1.1.1.1",
		UserAgent: "TestAgent1",
		RawUrl:    "https://example.com/page1",
	}
	user1Sig := visitors.BuildUniqueVisitorId("example.com", user1Input.IPAddress, user1Input.UserAgent, "test-key")

	user2Input := &events.CollectEventInput{
		IPAddress: "2.2.2.2",
		UserAgent: "TestAgent2",
		RawUrl:    "https://example.com/pageA",
	}
	user2Sig := visitors.BuildUniqueVisitorId("example.com", user2Input.IPAddress, user2Input.UserAgent, "test-key")

	// Test Cases - testing observable behavior via ProcessUnprocessedEvents

	t.Run("First page view is new visitor", func(t *testing.T) {
		testsupport.CleanTables(db, []string{"events", "ingested_events"})
		_, err := createIngestedEvent(db, website.ID, &events.CollectEventInput{
			IPAddress: user1Input.IPAddress, UserAgent: user1Input.UserAgent,
			RawUrl: "https://example.com/page1", EventType: events.EventTypePageView, Timestamp: baseTime,
		})
		require.NoError(t, err)

		result, err := events.ProcessUnprocessedEvents(dbManager, logger, 10)
		require.NoError(t, err)
		require.Len(t, result.ProcessingData, 1, "Expected 1 processed event data")
		assert.True(t, result.ProcessingData[0].IsNewVisitor, "First page view should be marked as new visitor")
		assert.Equal(t, user1Sig, result.ProcessingData[0].UserSignature)
	})

	t.Run("Second page view is not new visitor", func(t *testing.T) {
		testsupport.CleanTables(db, []string{"events", "ingested_events"})
		// First event - with an earlier timestamp
		firstEventTime := baseTime.Add(-10 * time.Minute)
		_, err := createIngestedEvent(db, website.ID, &events.CollectEventInput{
			IPAddress: user1Input.IPAddress, UserAgent: user1Input.UserAgent,
			RawUrl: "https://example.com/page1", EventType: events.EventTypePageView, Timestamp: firstEventTime,
		})
		require.NoError(t, err)
		_, err = events.ProcessUnprocessedEvents(dbManager, logger, 10) // Process first event
		require.NoError(t, err)

		// Second event - using the base time
		_, err = createIngestedEvent(db, website.ID, &events.CollectEventInput{
			IPAddress: user1Input.IPAddress, UserAgent: user1Input.UserAgent,
			RawUrl: "https://example.com/page2", EventType: events.EventTypePageView, Timestamp: baseTime,
		})
		require.NoError(t, err)

		result, err := events.ProcessUnprocessedEvents(dbManager, logger, 10) // Process second event
		require.NoError(t, err)
		require.Len(t, result.ProcessingData, 1, "Expected 1 processed event data for the second batch")
		assert.False(t, result.ProcessingData[0].IsNewVisitor, "Second page view should not be marked as new visitor")
		assert.Equal(t, user1Sig, result.ProcessingData[0].UserSignature)
	})

	t.Run("First custom event is new visitor", func(t *testing.T) {
		testsupport.CleanTables(db, []string{"events", "ingested_events"})
		_, err := createIngestedEvent(db, website.ID, &events.CollectEventInput{
			IPAddress: user1Input.IPAddress, UserAgent: user1Input.UserAgent,
			RawUrl: "https://example.com/action", EventType: events.EventTypeCustomEvent, CustomEventName: "click", Timestamp: baseTime,
		})
		require.NoError(t, err)

		result, err := events.ProcessUnprocessedEvents(dbManager, logger, 10)
		require.NoError(t, err)
		require.Len(t, result.ProcessingData, 1, "Expected 1 processed event data")
		assert.True(t, result.ProcessingData[0].IsNewVisitor, "First custom event should be marked as new visitor")
		assert.Equal(t, user1Sig, result.ProcessingData[0].UserSignature)
	})

	t.Run("Second custom event with same name is not new visitor", func(t *testing.T) {
		testsupport.CleanTables(db, []string{"events", "ingested_events"})
		// First event - with an earlier timestamp
		firstEventTime := baseTime.Add(-10 * time.Minute)
		_, err := createIngestedEvent(db, website.ID, &events.CollectEventInput{
			IPAddress: user1Input.IPAddress, UserAgent: user1Input.UserAgent,
			RawUrl: "https://example.com/action1", EventType: events.EventTypeCustomEvent, CustomEventName: "click", Timestamp: firstEventTime,
		})
		require.NoError(t, err)
		_, err = events.ProcessUnprocessedEvents(dbManager, logger, 10) // Process first event
		require.NoError(t, err)

		// Debug: Check if the first event was persisted
		var eventCount int64
		err = db.Model(&events.Event{}).
			Where("website_id = ? AND user_signature = ? AND event_type = ? AND custom_event_name = ? AND timestamp = ?",
				website.ID, user1Sig, events.EventTypeCustomEvent, "click", firstEventTime).
			Count(&eventCount).Error
		require.NoError(t, err, "Failed to query Event table")
		assert.Equal(t, int64(1), eventCount, "Expected first click event to be persisted in Event table")

		// Second event with same custom event name - using the base time
		_, err = createIngestedEvent(db, website.ID, &events.CollectEventInput{
			IPAddress: user1Input.IPAddress, UserAgent: user1Input.UserAgent,
			RawUrl: "https://example.com/action2", EventType: events.EventTypeCustomEvent, CustomEventName: "click", Timestamp: baseTime,
		})
		require.NoError(t, err)

		result, err := events.ProcessUnprocessedEvents(dbManager, logger, 10) // Process second event
		require.NoError(t, err)
		require.Len(t, result.ProcessingData, 1, "Expected 1 processed event data for the second batch")
		assert.False(t, result.ProcessingData[0].IsNewVisitor, "Second custom event with same name should not be marked as new visitor")
		assert.Equal(t, user1Sig, result.ProcessingData[0].UserSignature)
	})

	t.Run("Custom event with different name is new visitor", func(t *testing.T) {
		testsupport.CleanTables(db, []string{"events", "ingested_events"})
		// First event - with an earlier timestamp
		firstEventTime := baseTime.Add(-10 * time.Minute)
		_, err := createIngestedEvent(db, website.ID, &events.CollectEventInput{
			IPAddress: user1Input.IPAddress, UserAgent: user1Input.UserAgent,
			RawUrl: "https://example.com/action1", EventType: events.EventTypeCustomEvent, CustomEventName: "click1", Timestamp: firstEventTime,
		})
		require.NoError(t, err)
		_, err = events.ProcessUnprocessedEvents(dbManager, logger, 10) // Process first event
		require.NoError(t, err)

		// Second event with different custom event name - using the base time
		_, err = createIngestedEvent(db, website.ID, &events.CollectEventInput{
			IPAddress: user1Input.IPAddress, UserAgent: user1Input.UserAgent,
			RawUrl: "https://example.com/action2", EventType: events.EventTypeCustomEvent, CustomEventName: "click2", Timestamp: baseTime,
		})
		require.NoError(t, err)

		result, err := events.ProcessUnprocessedEvents(dbManager, logger, 10) // Process second event
		require.NoError(t, err)
		require.Len(t, result.ProcessingData, 1, "Expected 1 processed event data for the second batch")
		assert.True(t, result.ProcessingData[0].IsNewVisitor, "Custom event with different name should be marked as new visitor")
		assert.Equal(t, user1Sig, result.ProcessingData[0].UserSignature)
	})

	t.Run("First event for different user is new visitor", func(t *testing.T) {
		testsupport.CleanTables(db, []string{"events", "ingested_events"})
		// User 1 event - with an earlier timestamp
		firstEventTime := baseTime.Add(-10 * time.Minute)
		_, err := createIngestedEvent(db, website.ID, &events.CollectEventInput{
			IPAddress: user1Input.IPAddress, UserAgent: user1Input.UserAgent,
			RawUrl: "https://example.com/page1", EventType: events.EventTypePageView, Timestamp: firstEventTime,
		})
		require.NoError(t, err)
		_, err = events.ProcessUnprocessedEvents(dbManager, logger, 10)
		require.NoError(t, err)

		// User 2 event - using the base time
		_, err = createIngestedEvent(db, website.ID, &events.CollectEventInput{
			IPAddress: user2Input.IPAddress, UserAgent: user2Input.UserAgent,
			RawUrl: "https://example.com/pageA", EventType: events.EventTypePageView, Timestamp: baseTime,
		})
		require.NoError(t, err)

		result, err := events.ProcessUnprocessedEvents(dbManager, logger, 10)
		require.NoError(t, err)
		require.Len(t, result.ProcessingData, 1, "Expected 1 processed event data for the second batch")
		assert.True(t, result.ProcessingData[0].IsNewVisitor, "First event for second user should be marked as new visitor")
		assert.Equal(t, user2Sig, result.ProcessingData[0].UserSignature)
	})

	t.Run("Custom event after page view is new visitor for that event", func(t *testing.T) {
		testsupport.CleanTables(db, []string{"events", "ingested_events"})
		// First event: Page view - with an earlier timestamp
		firstEventTime := baseTime.Add(-10 * time.Minute)
		_, err := createIngestedEvent(db, website.ID, &events.CollectEventInput{
			IPAddress: user1Input.IPAddress, UserAgent: user1Input.UserAgent,
			RawUrl: "https://example.com/page1", EventType: events.EventTypePageView, Timestamp: firstEventTime,
		})
		require.NoError(t, err)
		_, err = events.ProcessUnprocessedEvents(dbManager, logger, 10) // Process page view
		require.NoError(t, err)

		// Second event: Custom event from the same user - using the base time
		_, err = createIngestedEvent(db, website.ID, &events.CollectEventInput{
			IPAddress: user1Input.IPAddress, UserAgent: user1Input.UserAgent,
			RawUrl: "https://example.com/action", EventType: events.EventTypeCustomEvent, CustomEventName: "click", Timestamp: baseTime,
		})
		require.NoError(t, err)

		result, err := events.ProcessUnprocessedEvents(dbManager, logger, 10) // Process custom event
		require.NoError(t, err)
		require.Len(t, result.ProcessingData, 1, "Expected 1 processed event data for the custom event batch")
		assert.True(t, result.ProcessingData[0].IsNewVisitor, "First custom event should be marked as new visitor for that event")
		assert.Equal(t, user1Sig, result.ProcessingData[0].UserSignature)
	})

	t.Run("Page view after custom event is not new visitor for website", func(t *testing.T) {
		testsupport.CleanTables(db, []string{"events", "ingested_events"})
		// First event: Custom event - with an earlier timestamp
		firstEventTime := baseTime.Add(-10 * time.Minute)
		_, err := createIngestedEvent(db, website.ID, &events.CollectEventInput{
			IPAddress: user1Input.IPAddress, UserAgent: user1Input.UserAgent,
			RawUrl: "https://example.com/action", EventType: events.EventTypeCustomEvent, CustomEventName: "click", Timestamp: firstEventTime,
		})
		require.NoError(t, err)
		_, err = events.ProcessUnprocessedEvents(dbManager, logger, 10) // Process custom event
		require.NoError(t, err)

		// Second event: Page view from the same user - using the base time
		_, err = createIngestedEvent(db, website.ID, &events.CollectEventInput{
			IPAddress: user1Input.IPAddress, UserAgent: user1Input.UserAgent,
			RawUrl: "https://example.com/page1", EventType: events.EventTypePageView, Timestamp: baseTime,
		})
		require.NoError(t, err)

		result, err := events.ProcessUnprocessedEvents(dbManager, logger, 10) // Process page view
		require.NoError(t, err)
		require.Len(t, result.ProcessingData, 1, "Expected 1 processed event data for the page view batch")
		assert.False(t, result.ProcessingData[0].IsNewVisitor, "Page view after custom event should not be marked as new visitor for website")
		assert.Equal(t, user1Sig, result.ProcessingData[0].UserSignature)
	})
}

// Helper function to create an IngestedEvent directly in the database
func createIngestedEvent(tx *gorm.DB, websiteID uint, input *events.CollectEventInput) (*events.IngestedEvent, error) {
	// Parse URL
	parsedURL, err := url.Parse(input.RawUrl)
	if err != nil {
		return nil, err
	}

	hostname := parsedURL.Hostname()
	pathname := parsedURL.Path
	if pathname == "" {
		pathname = "/"
	}

	// Parse referrer URL
	referrerHostname := events.DirectOrUnknownReferrer
	referrerPathname := ""
	if input.ReferrerURL != "" {
		refURL, err := url.Parse(input.ReferrerURL)
		if err == nil {
			referrerHostname = refURL.Hostname()
			referrerPathname = refURL.Path
			if referrerPathname == "" {
				referrerPathname = "/"
			}
		}
	}

	// Create user signature using exported function
	userSignature := visitors.BuildUniqueVisitorId(hostname, input.IPAddress, input.UserAgent, "test-key")

	// Create IngestedEvent using exported type
	tempEvent := &events.IngestedEvent{
		WebsiteID:        websiteID,
		UserSignature:    userSignature,
		Hostname:         hostname,
		Pathname:         pathname,
		RawURL:           input.RawUrl,
		ReferrerHostname: referrerHostname,
		ReferrerPathname: referrerPathname,
		EventType:        input.EventType,
		CustomEventName:  input.CustomEventName,
		CustomEventMeta:  input.CustomEventMeta,
		Timestamp:        input.Timestamp,
		UserAgent:        input.UserAgent,
		Country:          "xx", // Test country code
		CreatedAt:        time.Now().UTC(),
		Processed:        0, // Unprocessed
	}

	err = tx.Create(tempEvent).Error
	if err != nil {
		return nil, err
	}

	return tempEvent, nil
}
