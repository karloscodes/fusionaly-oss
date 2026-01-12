package events_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"fusionaly/internal/events"
	"fusionaly/internal/settings"
	"fusionaly/internal/visitors"
	"fusionaly/internal/websites"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"fusionaly/internal/config"
	"fusionaly/internal/testsupport"
)

// TestProcessUnprocessedEvents tests the ProcessUnprocessedEvents function
func TestProcessUnprocessedEvents(t *testing.T) {
	// Run only the first test case for debugging
	testCases := []struct {
		name             string
		setupEvents      func(t *testing.T, db *gorm.DB, website websites.Website)
		expectEventCount int
		validate         func(t *testing.T, db *gorm.DB, result *events.EventProcessingResult)
	}{
		{
			name: "Single page view event",
			setupEvents: func(t *testing.T, db *gorm.DB, website websites.Website) {
				// Create a single unprocessed event directly in the database
				now := time.Now().UTC()
				event := events.IngestedEvent{
					WebsiteID:        website.ID,
					UserSignature:    "test-user-signature",
					Hostname:         website.Domain,
					Pathname:         "/page1",
					RawURL:           "https://" + website.Domain + "/page1",
					ReferrerHostname: "google.com",
					ReferrerPathname: "/search",
					EventType:        events.EventTypePageView,
					Timestamp:        now,
					UserAgent:        "Mozilla/5.0 (test)",
					Country:          "US",
					CreatedAt:        now,
					Processed:        0,
				}

				err := db.Create(&event).Error
				if err != nil {
					t.Fatalf("Failed to create test event: %v", err)
				}

				// Verify the event was created successfully
				var count int64
				err = db.Model(&events.IngestedEvent{}).Count(&count).Error
				if err != nil {
					t.Fatalf("Failed to count events: %v", err)
				}
				if count != 1 {
					t.Fatalf("Should have 1 event in the database, got %d", count)
				}
				t.Logf("Successfully created test event")
			},
			expectEventCount: 1,
			validate: func(t *testing.T, db *gorm.DB, result *events.EventProcessingResult) {
				// Only validate if we got a result
				if result == nil {
					t.Fatal("Result is nil")
					return
				}

				t.Logf("Validating result with %d processed events", len(result.ProcessedEvents))
				if len(result.ProcessedEvents) != 1 {
					t.Errorf("Expected 1 processed event, got %d", len(result.ProcessedEvents))
					return
				}

				// Further validation only if we have events
				if len(result.ProcessedEvents) > 0 {
					t.Logf("Event type: %v", result.ProcessedEvents[0].EventType)
					t.Logf("Referrer hostname: %s", result.ProcessedEvents[0].ReferrerHostname)
				}
			},
		},
		{
			name: "Multiple events",
			setupEvents: func(t *testing.T, db *gorm.DB, website websites.Website) {
				// Create three unprocessed events directly in the database
				now := time.Now().UTC()

				testEvents := []events.IngestedEvent{
					{
						WebsiteID:        website.ID,
						UserSignature:    "user1",
						Hostname:         website.Domain,
						Pathname:         "/landing",
						RawURL:           "https://" + website.Domain + "/landing",
						ReferrerHostname: "twitter.com",
						ReferrerPathname: "/post",
						EventType:        events.EventTypePageView,
						Timestamp:        now,
						UserAgent:        "Mozilla/5.0 (test)",
						Country:          "US",
						CreatedAt:        now,
						Processed:        0,
					},
					{
						WebsiteID:        website.ID,
						UserSignature:    "user2",
						Hostname:         website.Domain,
						Pathname:         "/signup",
						RawURL:           "https://" + website.Domain + "/signup",
						ReferrerHostname: "facebook.com",
						ReferrerPathname: "/ad",
						EventType:        events.EventTypePageView,
						Timestamp:        now.Add(5 * time.Minute),
						UserAgent:        "Mozilla/5.0 (test)",
						Country:          "UK",
						CreatedAt:        now,
						Processed:        0,
					},
					{
						WebsiteID:        website.ID,
						UserSignature:    "user3",
						Hostname:         website.Domain,
						Pathname:         "/checkout",
						RawURL:           "https://" + website.Domain + "/checkout",
						ReferrerHostname: website.Domain,
						ReferrerPathname: "/products",
						EventType:        events.EventTypeCustomEvent,
						CustomEventName:  "checkout_started",
						CustomEventMeta:  `{"value":"49.99"}`,
						Timestamp:        now.Add(10 * time.Minute),
						UserAgent:        "Mozilla/5.0 (test)",
						Country:          "CA",
						CreatedAt:        now,
						Processed:        0,
					},
				}

				// Create events one by one to avoid any transaction issues
				for _, event := range testEvents {
					err := db.Create(&event).Error
					require.NoError(t, err, "Failed to create test event")
				}

				// Verify all events were created
				var count int64
				err := db.Model(&events.IngestedEvent{}).Count(&count).Error
				require.NoError(t, err)
				require.Equal(t, int64(3), count, "Should have 3 events in the database")
			},
			expectEventCount: 3,
			validate: func(t *testing.T, db *gorm.DB, result *events.EventProcessingResult) {
				// Check processed events result
				require.Equal(t, 3, len(result.ProcessedEvents), "Should have 3 processed events")

				// Verify events in the events table
				var dbEvents []events.Event
				err := db.Find(&dbEvents).Error
				require.NoError(t, err)
				require.Equal(t, 3, len(dbEvents), "Should have 3 events in the events table")

				// Check that all events are now marked as processed
				var unprocessedCount int64
				err = db.Model(&events.IngestedEvent{}).Where("processed = ?", 0).Count(&unprocessedCount).Error
				require.NoError(t, err)
				assert.Equal(t, int64(0), unprocessedCount, "Should have 0 unprocessed events")

				// Verify custom event data
				var customEvent events.Event
				err = db.Where("event_type = ?", events.EventTypeCustomEvent).First(&customEvent).Error
				require.NoError(t, err)
				assert.Equal(t, "checkout_started", customEvent.CustomEventName)
				assert.Contains(t, customEvent.CustomEventMeta, "49.99")
			},
		},
		{
			name: "No unprocessed events",
			setupEvents: func(t *testing.T, db *gorm.DB, website websites.Website) {
				// No events to set up
			},
			expectEventCount: 0,
			validate: func(t *testing.T, db *gorm.DB, result *events.EventProcessingResult) {
				assert.Empty(t, result.ProcessedEvents, "Should have no processed events")
				assert.Empty(t, result.ProcessingData, "Should have no processing data")

				// Verify no events in the events table
				var eventCount int64
				err := db.Model(&events.Event{}).Count(&eventCount).Error
				require.NoError(t, err)
				assert.Equal(t, int64(0), eventCount, "Should have 0 events in the events table")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Test panicked: %v", r)
				}
			}()

			// Set up a fresh database for each test case
			dbManager, logger := testsupport.SetupTestDBManager(t)
			db := dbManager.GetConnection()

			t.Log("Cleaning tables")
			testsupport.CleanAllTables(db)

			t.Log("Creating test website")
			website := testsupport.CreateTestWebsite(db, "example.com")
			t.Logf("Created test website with ID: %d, Domain: %s", website.ID, website.Domain)

			t.Log("Setting up test events")
			tc.setupEvents(t, db, website)

			// Debug: dump the content of IngestedEvent table
			var ingestedEvents []events.IngestedEvent
			err := db.Find(&ingestedEvents).Error
			if err != nil {
				t.Fatalf("Failed to retrieve events: %v", err)
			}
			t.Logf("Found %d events in the database", len(ingestedEvents))
			for i, e := range ingestedEvents {
				t.Logf("Event %d: ID=%d, WebsiteID=%d, Processed=%v", i, e.ID, e.WebsiteID, e.Processed)
			}

			// Process the events with step-by-step debugging
			t.Log("Starting ProcessUnprocessedEvents")

			// Count unprocessed events before processing
			var unprocessedCount int64
			err = db.Model(&events.IngestedEvent{}).Where("processed = ?", 0).Count(&unprocessedCount).Error
			if err != nil {
				t.Fatalf("Failed to count unprocessed events: %v", err)
			}
			t.Logf("Found %d unprocessed events before processing", unprocessedCount)

			// Process events
			t.Log("Calling ProcessUnprocessedEvents")
			result, err := events.ProcessUnprocessedEvents(dbManager, logger, 10)
			if err != nil {
				t.Fatalf("ProcessUnprocessedEvents failed with error: %v", err)
			}

			t.Log("ProcessUnprocessedEvents returned successfully")
			if result == nil {
				t.Fatal("Result is nil")
			} else {
				t.Logf("Result contains %d processed events", len(result.ProcessedEvents))
			}

			// Check the state of events after processing
			var processedCount int64
			err = db.Model(&events.IngestedEvent{}).Where("processed = ?", 1).Count(&processedCount).Error
			if err != nil {
				t.Fatalf("Failed to count processed events: %v", err)
			}
			t.Logf("After processing: %d processed events in database", processedCount)

			var eventsTableCount int64
			err = db.Model(&events.Event{}).Count(&eventsTableCount).Error
			if err != nil {
				t.Fatalf("Failed to count events in Events table: %v", err)
			}
			t.Logf("After processing: %d events in Events table", eventsTableCount)

			// Run validation
			tc.validate(t, db, result)
		})
	}
}

// TestCollectEvent tests multiple scenarios for event collection
func TestCollectEvent(t *testing.T) {
	// Set up the test environment
	dbManager, logger := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	testsupport.CleanAllTables(db)
	website := testsupport.CreateTestWebsite(db, "example.com")

	// Define test cases
	tests := []struct {
		name          string
		input         events.CollectEventInput
		expectedError bool
		validate      func(t *testing.T, event *events.IngestedEvent)
	}{
		{
			name: "Valid page view",
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.1",
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "https://google.com/search",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://example.com/page1",
			},
			expectedError: false,
			validate: func(t *testing.T, event *events.IngestedEvent) {
				assert.Equal(t, website.ID, event.WebsiteID)
				assert.Equal(t, "example.com", event.Hostname)
				assert.Equal(t, "/page1", event.Pathname)
				assert.Equal(t, "google.com", event.ReferrerHostname)
				assert.Equal(t, events.EventTypePageView, event.EventType)
				assert.Equal(t, 0, event.Processed, "Should be unprocessed")
			},
		},
		{
			name: "Custom event with metadata",
			input: events.CollectEventInput{
				IPAddress:       "192.168.1.1",
				UserAgent:       "Mozilla/5.0 (iPhone; test)",
				ReferrerURL:     "https://example.com/pricing",
				EventType:       events.EventTypeCustomEvent,
				CustomEventName: "signup_complete",
				CustomEventMeta: `{"category":"signup","action":"complete"}`,
				Timestamp:       time.Now().UTC(),
				RawUrl:          "https://example.com/signup",
			},
			expectedError: false,
			validate: func(t *testing.T, event *events.IngestedEvent) {
				assert.Equal(t, website.ID, event.WebsiteID)
				assert.Equal(t, "example.com", event.Hostname)
				assert.Equal(t, "/signup", event.Pathname)
				assert.Equal(t, "signup_complete", event.CustomEventName)
				assert.Equal(t, `{"category":"signup","action":"complete"}`, event.CustomEventMeta)
				assert.Equal(t, events.EventTypeCustomEvent, event.EventType)
				assert.Equal(t, 0, event.Processed, "Should be unprocessed")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Running test: %s", tc.name)
			// Clean up from previous test
			db.Exec("DELETE FROM ingested_events")

			// Collect the event
			err := events.CollectEvent(dbManager, logger, &tc.input)

			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Read back from DB to verify
				var savedEvent events.IngestedEvent
				err = db.First(&savedEvent).Error
				require.NoError(t, err)

				// Run validation logic
				tc.validate(t, &savedEvent)
			}
		})
	}
}

// TestCollectEventComprehensive tests all the logic in prepareTempEvent
func TestCollectEventComprehensive(t *testing.T) {
	// Set up test environment
	dbManager, logger := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	testsupport.CleanAllTables(db)

	// Create test websites
	baseWebsite := testsupport.CreateTestWebsite(db, "example.com")
	subdomainWebsite := testsupport.CreateTestWebsite(db, "blog.example.com")

	// Define test cases
	tests := []struct {
		name          string
		setup         func(t *testing.T)
		input         events.CollectEventInput
		expectedError bool
		errorContains string
		validate      func(t *testing.T, event *events.IngestedEvent)
	}{
		{
			name: "Direct hostname match",
			setup: func(t *testing.T) {
				// No additional setup needed
			},
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.1",
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "https://google.com/search",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://example.com/page1",
			},
			expectedError: false,
			validate: func(t *testing.T, event *events.IngestedEvent) {
				assert.Equal(t, baseWebsite.ID, event.WebsiteID)
				assert.Equal(t, "example.com", event.Hostname)
				assert.Equal(t, "/page1", event.Pathname)
				assert.Equal(t, "google.com", event.ReferrerHostname)
				assert.Equal(t, "/search", event.ReferrerPathname)
			},
		},
		{
			name: "Subdomain with exact match",
			setup: func(t *testing.T) {
				// No additional setup needed - blog.example.com exists
			},
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.2",
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "https://twitter.com/post",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://blog.example.com/article",
			},
			expectedError: false,
			validate: func(t *testing.T, event *events.IngestedEvent) {
				assert.Equal(t, subdomainWebsite.ID, event.WebsiteID)
				assert.Equal(t, "blog.example.com", event.Hostname)
				assert.Equal(t, "/article", event.Pathname)
				assert.Equal(t, "twitter.com", event.ReferrerHostname)
				assert.Equal(t, "/post", event.ReferrerPathname)
			},
		},
		{
			name: "Subdomain fallback with tracking enabled",
			setup: func(t *testing.T) {
				// Enable subdomain tracking for example.com
				err := settings.UpdateSubdomainTrackingSettings(db, "example.com", true)
				require.NoError(t, err)
			},
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.3",
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "https://facebook.com/share",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://api.example.com/data",
			},
			expectedError: false,
			validate: func(t *testing.T, event *events.IngestedEvent) {
				assert.Equal(t, baseWebsite.ID, event.WebsiteID)
				assert.Equal(t, "api.example.com", event.Hostname)
				assert.Equal(t, "/data", event.Pathname)
				assert.Equal(t, "facebook.com", event.ReferrerHostname)
				assert.Equal(t, "/share", event.ReferrerPathname)
			},
		},
		{
			name: "Subdomain fallback with tracking disabled",
			setup: func(t *testing.T) {
				// Disable subdomain tracking for example.com
				err := settings.UpdateSubdomainTrackingSettings(db, "example.com", false)
				require.NoError(t, err)
			},
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.4",
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "https://linkedin.com/post",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://shop.example.com/products",
			},
			expectedError: true,
			errorContains: "website not found for domain: shop.example.com",
			validate:      nil,
		},
		{
			name: "Deep subdomain fallback with tracking enabled",
			setup: func(t *testing.T) {
				// Enable subdomain tracking for example.com
				err := settings.UpdateSubdomainTrackingSettings(db, "example.com", true)
				require.NoError(t, err)
			},
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.5",
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "https://reddit.com/r/test",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://api.v2.example.com/graphql",
			},
			expectedError: false,
			validate: func(t *testing.T, event *events.IngestedEvent) {
				assert.Equal(t, baseWebsite.ID, event.WebsiteID)
				assert.Equal(t, "api.v2.example.com", event.Hostname)
				assert.Equal(t, "/graphql", event.Pathname)
				assert.Equal(t, "reddit.com", event.ReferrerHostname)
				assert.Equal(t, "/r/test", event.ReferrerPathname)
			},
		},
		{
			name: "Deep subdomain fallback with tracking disabled",
			setup: func(t *testing.T) {
				// Disable subdomain tracking for example.com
				err := settings.UpdateSubdomainTrackingSettings(db, "example.com", false)
				require.NoError(t, err)
			},
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.6",
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "https://hackernews.com/item",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://cdn.assets.example.com/images/logo.png",
			},
			expectedError: true,
			errorContains: "website not found for domain: cdn.assets.example.com",
			validate:      nil,
		},
		{
			name: "Custom event with metadata",
			setup: func(t *testing.T) {
				// Reset subdomain tracking to default
				err := settings.UpdateSubdomainTrackingSettings(db, "example.com", false)
				require.NoError(t, err)
			},
			input: events.CollectEventInput{
				IPAddress:       "192.168.1.7",
				UserAgent:       "Mozilla/5.0 (iPhone; test)",
				ReferrerURL:     "https://example.com/pricing",
				EventType:       events.EventTypeCustomEvent,
				CustomEventName: "signup_complete",
				CustomEventMeta: `{"category":"signup","action":"complete","value":99}`,
				Timestamp:       time.Now().UTC(),
				RawUrl:          "https://example.com/signup",
			},
			expectedError: false,
			validate: func(t *testing.T, event *events.IngestedEvent) {
				assert.Equal(t, baseWebsite.ID, event.WebsiteID)
				assert.Equal(t, "example.com", event.Hostname)
				assert.Equal(t, "/signup", event.Pathname)
				assert.Equal(t, "signup_complete", event.CustomEventName)
				assert.Equal(t, `{"category":"signup","action":"complete","value":99}`, event.CustomEventMeta)
				assert.Equal(t, events.EventTypeCustomEvent, event.EventType)
			},
		},
		{
			name: "Unknown domain - should fail",
			setup: func(t *testing.T) {
				// No setup needed
			},
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.8",
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "https://google.com/search",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://unknown-domain.com/page",
			},
			expectedError: true,
			errorContains: "website not found for domain: unknown-domain.com",
			validate:      nil,
		},
		{
			name: "Subdomain of unknown domain - should fail",
			setup: func(t *testing.T) {
				// No setup needed
			},
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.9",
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "https://google.com/search",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://www.unknown-domain.com/page",
			},
			expectedError: true,
			errorContains: "website not found for domain: www.unknown-domain.com",
			validate:      nil,
		},
		{
			name: "No referrer - should use direct",
			setup: func(t *testing.T) {
				// No setup needed
			},
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.10",
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "", // No referrer
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://example.com/direct",
			},
			expectedError: false,
			validate: func(t *testing.T, event *events.IngestedEvent) {
				assert.Equal(t, baseWebsite.ID, event.WebsiteID)
				assert.Equal(t, "example.com", event.Hostname)
				assert.Equal(t, "/direct", event.Pathname)
				assert.Equal(t, events.DirectOrUnknownReferrer, event.ReferrerHostname)
				assert.Equal(t, "", event.ReferrerPathname)
			},
		},
		{
			name: "Invalid referrer URL - should use direct",
			setup: func(t *testing.T) {
				// No setup needed
			},
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.11",
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "invalid-url",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://example.com/invalid-ref",
			},
			expectedError: false,
			validate: func(t *testing.T, event *events.IngestedEvent) {
				assert.Equal(t, baseWebsite.ID, event.WebsiteID)
				assert.Equal(t, "example.com", event.Hostname)
				assert.Equal(t, "/invalid-ref", event.Pathname)
				assert.Equal(t, events.DirectOrUnknownReferrer, event.ReferrerHostname)
				assert.Equal(t, "", event.ReferrerPathname)
			},
		},
		{
			name: "Empty User-Agent - should use default",
			setup: func(t *testing.T) {
				// No setup needed
			},
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.12",
				UserAgent:   "", // Empty user agent
				ReferrerURL: "https://google.com/search",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://example.com/no-ua",
			},
			expectedError: false,
			validate: func(t *testing.T, event *events.IngestedEvent) {
				assert.Equal(t, baseWebsite.ID, event.WebsiteID)
				assert.Equal(t, "example.com", event.Hostname)
				assert.Equal(t, "/no-ua", event.Pathname)
				assert.Equal(t, "Unknown User Agent", event.UserAgent)
			},
		},
		{
			name: "ccTLD domain - should handle properly",
			setup: func(t *testing.T) {
				// Create a ccTLD website
				testsupport.CreateTestWebsite(db, "example.co.uk")
			},
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.13",
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "https://bbc.co.uk/news",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://example.co.uk/uk-page",
			},
			expectedError: false,
			validate: func(t *testing.T, event *events.IngestedEvent) {
				assert.Equal(t, "example.co.uk", event.Hostname)
				assert.Equal(t, "/uk-page", event.Pathname)
				assert.Equal(t, "bbc.co.uk", event.ReferrerHostname)
				assert.Equal(t, "/news", event.ReferrerPathname)
			},
		},
		{
			name: "ccTLD subdomain with tracking enabled",
			setup: func(t *testing.T) {
				// Enable subdomain tracking for example.co.uk
				err := settings.UpdateSubdomainTrackingSettings(db, "example.co.uk", true)
				require.NoError(t, err)
			},
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.14",
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "https://guardian.co.uk/tech",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://shop.example.co.uk/products",
			},
			expectedError: false,
			validate: func(t *testing.T, event *events.IngestedEvent) {
				assert.Equal(t, "shop.example.co.uk", event.Hostname)
				assert.Equal(t, "/products", event.Pathname)
				assert.Equal(t, "guardian.co.uk", event.ReferrerHostname)
				assert.Equal(t, "/tech", event.ReferrerPathname)
			},
		},
		{
			name: "ccTLD subdomain with tracking disabled",
			setup: func(t *testing.T) {
				// Disable subdomain tracking for example.co.uk
				err := settings.UpdateSubdomainTrackingSettings(db, "example.co.uk", false)
				require.NoError(t, err)
			},
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.15",
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "https://times.co.uk/world",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://www.example.co.uk/disabled",
			},
			expectedError: true,
			errorContains: "website not found for domain: www.example.co.uk",
			validate:      nil,
		},
		{
			name: "Localhost subdomain with tracking enabled",
			setup: func(t *testing.T) {
				// Create localhost website
				testsupport.CreateTestWebsite(db, "localhost")
				// Enable subdomain tracking for localhost
				err := settings.UpdateSubdomainTrackingSettings(db, "localhost", true)
				require.NoError(t, err)
			},
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.16",
				UserAgent:   "Mozilla/5.0 (development)",
				ReferrerURL: "https://github.com/dev",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://api.localhost/test",
			},
			expectedError: false,
			validate: func(t *testing.T, event *events.IngestedEvent) {
				assert.Equal(t, "api.localhost", event.Hostname)
				assert.Equal(t, "/test", event.Pathname)
				assert.Equal(t, "github.com", event.ReferrerHostname)
				assert.Equal(t, "/dev", event.ReferrerPathname)
			},
		},
		{
			name: "Deep localhost subdomain with tracking enabled",
			setup: func(t *testing.T) {
				// Enable subdomain tracking for localhost
				err := settings.UpdateSubdomainTrackingSettings(db, "localhost", true)
				require.NoError(t, err)
			},
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.17",
				UserAgent:   "Mozilla/5.0 (development)",
				ReferrerURL: "https://stackoverflow.com/questions",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://api.v2.localhost/graphql",
			},
			expectedError: false,
			validate: func(t *testing.T, event *events.IngestedEvent) {
				assert.Equal(t, "api.v2.localhost", event.Hostname)
				assert.Equal(t, "/graphql", event.Pathname)
				assert.Equal(t, "stackoverflow.com", event.ReferrerHostname)
				assert.Equal(t, "/questions", event.ReferrerPathname)
			},
		},
		{
			name: "Localhost subdomain with tracking disabled",
			setup: func(t *testing.T) {
				// Disable subdomain tracking for localhost
				err := settings.UpdateSubdomainTrackingSettings(db, "localhost", false)
				require.NoError(t, err)
			},
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.18",
				UserAgent:   "Mozilla/5.0 (development)",
				ReferrerURL: "https://docs.localhost/guide",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://www.localhost/home",
			},
			expectedError: true,
			errorContains: "website not found for domain: www.localhost",
			validate:      nil,
		},
		{
			name: "Self-referral with exact domain match",
			setup: func(t *testing.T) {
				// No additional setup needed
			},
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.19",
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "https://example.com/page1", // Self-referral
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://example.com/page2",
			},
			expectedError: false,
			validate: func(t *testing.T, event *events.IngestedEvent) {
				assert.Equal(t, baseWebsite.ID, event.WebsiteID)
				assert.Equal(t, "example.com", event.Hostname)
				assert.Equal(t, "/page2", event.Pathname)
				assert.Equal(t, events.DirectOrUnknownReferrer, event.ReferrerHostname, "Self-referral should be treated as direct traffic")
				assert.Equal(t, "", event.ReferrerPathname, "Self-referral pathname should be empty")
			},
		},
		{
			name: "Non-self-referral with external domain",
			setup: func(t *testing.T) {
				// No additional setup needed
			},
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.22",
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "https://google.com/search", // External referrer
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://example.com/landing",
			},
			expectedError: false,
			validate: func(t *testing.T, event *events.IngestedEvent) {
				assert.Equal(t, baseWebsite.ID, event.WebsiteID)
				assert.Equal(t, "example.com", event.Hostname)
				assert.Equal(t, "/landing", event.Pathname)
				assert.Equal(t, "google.com", event.ReferrerHostname, "External referrer should be preserved")
				assert.Equal(t, "/search", event.ReferrerPathname, "External referrer pathname should be preserved")
			},
		},
	}

	// Run test cases
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Clean up events from previous tests
			db.Exec("DELETE FROM ingested_events")

			// Run setup
			if tc.setup != nil {
				tc.setup(t)
			}

			// Collect the event
			err := events.CollectEvent(dbManager, logger, &tc.input)

			if tc.expectedError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}

				// Verify no event was created
				var count int64
				err = db.Model(&events.IngestedEvent{}).Count(&count).Error
				require.NoError(t, err)
				assert.Equal(t, int64(0), count)
			} else {
				assert.NoError(t, err)

				// Read back from DB to verify
				var savedEvent events.IngestedEvent
				err = db.First(&savedEvent).Error
				require.NoError(t, err)

				// Run validation if provided
				if tc.validate != nil {
					tc.validate(t, &savedEvent)
				}
			}
		})
	}
}

// TestCollectEventIPExclusion tests IP exclusion logic
func TestCollectEventIPExclusion(t *testing.T) {
	dbManager, logger := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	testsupport.CleanAllTables(db)

	// Create test website
	testsupport.CreateTestWebsite(db, "example.com")

	// Initialize default settings to ensure cache is set up
	err := settings.SetupDefaultSettings(db)
	require.NoError(t, err)

	// Set up excluded IPs
	err = settings.UpdateSetting(db, "excluded_ips", "192.168.1.100,10.0.0.1,127.0.0.1")
	require.NoError(t, err)

	tests := []struct {
		name          string
		ipAddress     string
		expectedError bool
		shouldSkip    bool
	}{
		{
			name:          "Allowed IP",
			ipAddress:     "192.168.1.1",
			expectedError: false,
			shouldSkip:    false,
		},
		{
			name:          "Excluded IP - 192.168.1.100",
			ipAddress:     "192.168.1.100",
			expectedError: false,
			shouldSkip:    true,
		},
		{
			name:          "Excluded IP - 10.0.0.1",
			ipAddress:     "10.0.0.1",
			expectedError: false,
			shouldSkip:    true,
		},
		{
			name:          "Excluded IP - 127.0.0.1",
			ipAddress:     "127.0.0.1",
			expectedError: false,
			shouldSkip:    true,
		},
		{
			name:          "Not excluded IP",
			ipAddress:     "203.0.113.1",
			expectedError: false,
			shouldSkip:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Clean up events from previous tests
			db.Exec("DELETE FROM ingested_events")

			input := events.CollectEventInput{
				IPAddress:   tc.ipAddress,
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "https://google.com/search",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://example.com/page",
			}

			err := events.CollectEvent(dbManager, logger, &input)

			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Check if event was created
				var count int64
				err = db.Model(&events.IngestedEvent{}).Count(&count).Error
				require.NoError(t, err)

				if tc.shouldSkip {
					assert.Equal(t, int64(0), count, "Event should be skipped for excluded IP")
				} else {
					assert.Equal(t, int64(1), count, "Event should be created for allowed IP")
				}
			}
		})
	}
}

// TestCollectEventEdgeCases tests edge cases and error conditions
func TestCollectEventEdgeCases(t *testing.T) {
	dbManager, logger := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	testsupport.CleanAllTables(db)

	// Create test website
	testsupport.CreateTestWebsite(db, "example.com")

	tests := []struct {
		name          string
		input         events.CollectEventInput
		expectedError bool
		errorContains string
	}{
		{
			name: "Empty URL",
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.1",
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "https://google.com/search",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "", // Empty URL
			},
			expectedError: true,
			errorContains: "empty URL provided",
		},
		{
			name: "Invalid URL",
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.1",
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "https://google.com/search",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "not-a-valid-url",
			},
			expectedError: true,
			errorContains: "URL missing hostname",
		},
		{
			name: "URL without hostname",
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.1",
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "https://google.com/search",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "/just/a/path",
			},
			expectedError: true,
			errorContains: "URL missing hostname",
		},
		{
			name: "Localhost - should be skipped",
			input: events.CollectEventInput{
				IPAddress:   "127.0.0.1",
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "https://google.com/search",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "http://localhost:3000/page",
			},
			expectedError: false,
			errorContains: "",
		},
		{
			name: "URL with port",
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.1",
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "https://google.com/search",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://example.com:8080/page",
			},
			expectedError: false,
			errorContains: "",
		},
		{
			name: "URL with query parameters",
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.1",
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "https://google.com/search",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://example.com/page?utm_source=test&utm_medium=email",
			},
			expectedError: false,
			errorContains: "",
		},
		{
			name: "URL with fragment",
			input: events.CollectEventInput{
				IPAddress:   "192.168.1.1",
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "https://google.com/search",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://example.com/page#section-1",
			},
			expectedError: false,
			errorContains: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Clean up events from previous tests
			db.Exec("DELETE FROM ingested_events")

			err := events.CollectEvent(dbManager, logger, &tc.input)

			if tc.expectedError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}

				// Verify no event was created
				var count int64
				err = db.Model(&events.IngestedEvent{}).Count(&count).Error
				require.NoError(t, err)
				assert.Equal(t, int64(0), count)
			} else {
				assert.NoError(t, err)

				// For localhost, event should be skipped (no error but no event created)
				if strings.Contains(tc.input.RawUrl, "localhost") {
					var count int64
					err = db.Model(&events.IngestedEvent{}).Count(&count).Error
					require.NoError(t, err)
					assert.Equal(t, int64(0), count, "Localhost events should be skipped")
				} else {
					// Verify event was created
					var savedEvent events.IngestedEvent
					err = db.First(&savedEvent).Error
					require.NoError(t, err)
					assert.Equal(t, tc.input.EventType, savedEvent.EventType)
				}
			}
		})
	}
}

// TestStripSubdomainsInEventCollection tests the stripSubdomains function indirectly through event collection
func TestStripSubdomainsInEventCollection(t *testing.T) {
	dbManager, logger := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	testsupport.CleanAllTables(db)

	tests := []struct {
		name         string
		hostname     string
		expectedBase string
		description  string
	}{
		{
			name:         "Simple domain",
			hostname:     "example.com",
			expectedBase: "example.com",
			description:  "Simple domain should remain unchanged",
		},
		{
			name:         "WWW subdomain",
			hostname:     "www.example.com",
			expectedBase: "example.com",
			description:  "WWW subdomain should be stripped",
		},
		{
			name:         "Single subdomain",
			hostname:     "blog.example.com",
			expectedBase: "example.com",
			description:  "Single subdomain should be stripped",
		},
		{
			name:         "Multiple subdomains",
			hostname:     "api.v1.example.com",
			expectedBase: "example.com",
			description:  "Multiple subdomains should be stripped to base",
		},
		{
			name:         "ccTLD - co.uk",
			hostname:     "example.co.uk",
			expectedBase: "example.co.uk",
			description:  "ccTLD should be handled correctly",
		},
		{
			name:         "ccTLD subdomain - co.uk",
			hostname:     "www.example.co.uk",
			expectedBase: "example.co.uk",
			description:  "ccTLD subdomain should be stripped correctly",
		},
		{
			name:         "ccTLD - com.au",
			hostname:     "shop.example.com.au",
			expectedBase: "example.com.au",
			description:  "Australian ccTLD should be handled correctly",
		},
		{
			name:         "ccTLD - co.jp",
			hostname:     "mail.example.co.jp",
			expectedBase: "example.co.jp",
			description:  "Japanese ccTLD should be handled correctly",
		},
		{
			name:         "Single word domain",
			hostname:     "localtest",
			expectedBase: "localtest",
			description:  "Single word domain should remain unchanged",
		},
		{
			name:         "Deep subdomain with ccTLD",
			hostname:     "cdn.assets.example.co.uk",
			expectedBase: "example.co.uk",
			description:  "Deep subdomain with ccTLD should be stripped to base",
		},
		{
			name:         "Localhost subdomain",
			hostname:     "sub.localhost",
			expectedBase: "localhost",
			description:  "Localhost subdomain should be stripped to localhost",
		},
		{
			name:         "Deep localhost subdomain",
			hostname:     "api.v1.localhost",
			expectedBase: "localhost",
			description:  "Deep localhost subdomain should be stripped to localhost",
		},
		{
			name:         "WWW localhost subdomain",
			hostname:     "www.localhost",
			expectedBase: "localhost",
			description:  "WWW localhost subdomain should be stripped to localhost",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Clean up tables
			testsupport.CleanAllTables(db)

			// Create a website with the expected base domain
			testsupport.CreateTestWebsite(db, tc.expectedBase)

			// Initialize default settings to ensure cache is set up
			err := settings.SetupDefaultSettings(db)
			require.NoError(t, err)

			// Enable subdomain tracking for the base domain
			err = settings.UpdateSubdomainTrackingSettings(db, tc.expectedBase, true)
			require.NoError(t, err)

			// Try to collect an event for the hostname
			input := events.CollectEventInput{
				IPAddress:   "192.168.1.1",
				UserAgent:   "Mozilla/5.0 (test)",
				ReferrerURL: "https://google.com/search",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://" + tc.hostname + "/test",
			}

			err = events.CollectEvent(dbManager, logger, &input)

			if tc.hostname == tc.expectedBase {
				// Direct match should work
				assert.NoError(t, err)
			} else {
				// Subdomain should fall back to base domain
				assert.NoError(t, err)
			}

			// Verify event was created with correct hostname
			var savedEvent events.IngestedEvent
			err = db.First(&savedEvent).Error
			require.NoError(t, err)
			assert.Equal(t, tc.hostname, savedEvent.Hostname, tc.description)
		})
	}
}

// TestVisitorStatusDetermination tests that the system correctly identifies
// new vs. returning visitors across different time frames
func TestVisitorStatusDetermination(t *testing.T) {
	dbManager, logger := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	testsupport.CleanAllTables(db)

	// Create test website
	testsupport.CreateTestWebsite(db, "example.com")

	// Define base time for consistent timestamps
	baseTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

	// Test case 1: First event for a visitor - should be a new visitor and new session
	input1 := events.CollectEventInput{
		IPAddress:   "192.168.1.1",
		UserAgent:   "Mozilla/5.0 (test)",
		ReferrerURL: "https://google.com/search",
		EventType:   events.EventTypePageView,
		Timestamp:   baseTime,
		RawUrl:      "https://example.com/page1",
	}
	err := events.CollectEvent(dbManager, logger, &input1)
	require.NoError(t, err)

	// Process the event
	result1, err := events.ProcessUnprocessedEvents(dbManager, logger, 10)
	require.NoError(t, err)
	require.Len(t, result1.ProcessedEvents, 1)
	require.Len(t, result1.ProcessingData, 1)

	// Validate the first event
	data1 := result1.ProcessingData[0]
	assert.True(t, data1.IsNewVisitor, "First event should be marked as a new visitor")
	assert.True(t, data1.IsNewSession, "First event should be marked as a new session")

	// Test case 2: Second event for the same visitor 5 minutes later in the same hour
	// Should be NOT a new visitor and NOT a new session (session timeout typically 30 minutes)
	input2 := events.CollectEventInput{
		IPAddress:   "192.168.1.1",        // Same IP to generate same userSignature
		UserAgent:   "Mozilla/5.0 (test)", // Same User-Agent
		ReferrerURL: "https://example.com/page1",
		EventType:   events.EventTypePageView,
		Timestamp:   baseTime.Add(5 * time.Minute),
		RawUrl:      "https://example.com/page2",
	}
	err = events.CollectEvent(dbManager, logger, &input2)
	require.NoError(t, err)

	// Process the second event
	result2, err := events.ProcessUnprocessedEvents(dbManager, logger, 10)
	require.NoError(t, err)
	require.Len(t, result2.ProcessedEvents, 1)
	require.Len(t, result2.ProcessingData, 1)

	// Validate the second event
	data2 := result2.ProcessingData[0]
	assert.False(t, data2.IsNewVisitor, "Second event in same hour should NOT be a new visitor")
	assert.False(t, data2.IsNewSession, "Second event within 5 minutes should NOT be a new session")

	// Test case 3: Third event for the same visitor 3 hours later (different hour, same day)
	// Should NOT be a new visitor (same day) but should be a new session (exceeds timeout)
	input3 := events.CollectEventInput{
		IPAddress:   "192.168.1.1",
		UserAgent:   "Mozilla/5.0 (test)",
		ReferrerURL: "https://example.com/page2",
		EventType:   events.EventTypePageView,
		Timestamp:   baseTime.Add(3 * time.Hour), // Still 2024-01-01
		RawUrl:      "https://example.com/page3",
	}
	err = events.CollectEvent(dbManager, logger, &input3)
	require.NoError(t, err)

	// Process the third event
	result3, err := events.ProcessUnprocessedEvents(dbManager, logger, 10)
	require.NoError(t, err)
	require.Len(t, result3.ProcessedEvents, 1)
	require.Len(t, result3.ProcessingData, 1)

	// Validate the third event
	data3 := result3.ProcessingData[0]
	// Corrected Assertion: Visitor is NOT new because they visited earlier today
	assert.False(t, data3.IsNewVisitor, "Third event on the same day should NOT be a new visitor")
	// Session Assertion remains True
	assert.True(t, data3.IsNewSession, "Third event after 3 hours should be a new session")

	// Test case 4: Event for a different visitor in the same hour as the first event
	input4 := events.CollectEventInput{
		IPAddress:   "192.168.1.2", // Different IP to generate different userSignature
		UserAgent:   "Mozilla/5.0 (test)",
		ReferrerURL: "https://google.com/search",
		EventType:   events.EventTypePageView,
		Timestamp:   baseTime.Add(10 * time.Minute),
		RawUrl:      "https://example.com/page4",
	}
	err = events.CollectEvent(dbManager, logger, &input4)
	require.NoError(t, err)

	// Process the fourth event
	result4, err := events.ProcessUnprocessedEvents(dbManager, logger, 10)
	require.NoError(t, err)
	require.Len(t, result4.ProcessedEvents, 1)
	require.Len(t, result4.ProcessingData, 1)

	// Validate the fourth event
	data4 := result4.ProcessingData[0]
	assert.True(t, data4.IsNewVisitor, "Event for a different visitor in the same hour should be a new visitor")
	assert.True(t, data4.IsNewSession, "Event for a different visitor should be a new session")
}

// TestCollectEventSubdomainUserSignature tests the new logic for user signature generation
// with subdomain tracking enabled/disabled
func TestCollectEventSubdomainUserSignature(t *testing.T) {
	// Set up test environment
	dbManager, logger := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	testsupport.CleanAllTables(db)

	// Create test websites - base domain and subdomain
	baseWebsite := testsupport.CreateTestWebsite(db, "example.com")
	subdomainWebsite := testsupport.CreateTestWebsite(db, "blog.example.com")

	// Initialize default settings
	err := settings.SetupDefaultSettings(db)
	require.NoError(t, err)

	// Test cases
	tests := []struct {
		name                      string
		setup                     func(t *testing.T)
		hostname                  string
		ipAddress                 string
		userAgent                 string
		expectedWebsiteID         uint
		expectSubdomainTracking   bool
		expectedUserSignatureBase string // The domain used for user signature generation
		description               string
	}{
		{
			name: "Base domain with subdomain tracking disabled",
			setup: func(t *testing.T) {
				err := settings.UpdateSubdomainTrackingSettings(db, "example.com", false)
				require.NoError(t, err)
			},
			hostname:                  "example.com",
			ipAddress:                 "192.168.1.1",
			userAgent:                 "Mozilla/5.0 (test-agent-1)",
			expectedWebsiteID:         baseWebsite.ID,
			expectSubdomainTracking:   false,
			expectedUserSignatureBase: "example.com",
			description:               "Base domain should use its own hostname for user signature",
		},
		{
			name: "Base domain with subdomain tracking enabled",
			setup: func(t *testing.T) {
				err := settings.UpdateSubdomainTrackingSettings(db, "example.com", true)
				require.NoError(t, err)
			},
			hostname:                  "example.com",
			ipAddress:                 "192.168.1.2",
			userAgent:                 "Mozilla/5.0 (test-agent-2)",
			expectedWebsiteID:         baseWebsite.ID,
			expectSubdomainTracking:   false, // Base domain itself doesn't use subdomain tracking logic
			expectedUserSignatureBase: "example.com",
			description:               "Base domain should still use its own hostname even when subdomain tracking is enabled",
		},
		{
			name: "Subdomain with exact match and subdomain tracking disabled",
			setup: func(t *testing.T) {
				err := settings.UpdateSubdomainTrackingSettings(db, "example.com", false)
				require.NoError(t, err)
			},
			hostname:                  "blog.example.com",
			ipAddress:                 "192.168.1.3",
			userAgent:                 "Mozilla/5.0 (test-agent-3)",
			expectedWebsiteID:         subdomainWebsite.ID,
			expectSubdomainTracking:   false,
			expectedUserSignatureBase: "blog.example.com",
			description:               "Subdomain with exact website match should use its own hostname for user signature",
		},
		{
			name: "Subdomain with exact match and subdomain tracking enabled",
			setup: func(t *testing.T) {
				err := settings.UpdateSubdomainTrackingSettings(db, "example.com", true)
				require.NoError(t, err)
			},
			hostname:                  "blog.example.com",
			ipAddress:                 "192.168.1.4",
			userAgent:                 "Mozilla/5.0 (test-agent-4)",
			expectedWebsiteID:         subdomainWebsite.ID,
			expectSubdomainTracking:   true,          // Subdomain tracking applies even for exact matches
			expectedUserSignatureBase: "example.com", // Should use base domain for consistency
			description:               "Subdomain with exact website match should use base domain for user signature when subdomain tracking is enabled",
		},
		{
			name: "Unknown subdomain with subdomain tracking disabled - should fail",
			setup: func(t *testing.T) {
				err := settings.UpdateSubdomainTrackingSettings(db, "example.com", false)
				require.NoError(t, err)
			},
			hostname:                  "shop.example.com",
			ipAddress:                 "192.168.1.5",
			userAgent:                 "Mozilla/5.0 (test-agent-5)",
			expectedWebsiteID:         0, // Should fail
			expectSubdomainTracking:   false,
			expectedUserSignatureBase: "",
			description:               "Unknown subdomain should fail when subdomain tracking is disabled",
		},
		{
			name: "Unknown subdomain with subdomain tracking enabled - should use base domain",
			setup: func(t *testing.T) {
				err := settings.UpdateSubdomainTrackingSettings(db, "example.com", true)
				require.NoError(t, err)
			},
			hostname:                  "shop.example.com",
			ipAddress:                 "192.168.1.6",
			userAgent:                 "Mozilla/5.0 (test-agent-6)",
			expectedWebsiteID:         baseWebsite.ID, // Should fall back to base domain
			expectSubdomainTracking:   true,
			expectedUserSignatureBase: "example.com", // Should use base domain for user signature
			description:               "Unknown subdomain should fall back to base domain and use base domain for user signature",
		},
		{
			name: "Deep subdomain with subdomain tracking enabled",
			setup: func(t *testing.T) {
				err := settings.UpdateSubdomainTrackingSettings(db, "example.com", true)
				require.NoError(t, err)
			},
			hostname:                  "api.v2.shop.example.com",
			ipAddress:                 "192.168.1.7",
			userAgent:                 "Mozilla/5.0 (test-agent-7)",
			expectedWebsiteID:         baseWebsite.ID, // Should fall back to base domain
			expectSubdomainTracking:   true,
			expectedUserSignatureBase: "example.com", // Should use base domain for user signature
			description:               "Deep subdomain should fall back to base domain and use base domain for user signature",
		},
		{
			name: "Same visitor across different subdomains with subdomain tracking enabled",
			setup: func(t *testing.T) {
				err := settings.UpdateSubdomainTrackingSettings(db, "example.com", true)
				require.NoError(t, err)
			},
			hostname:                  "api.example.com",
			ipAddress:                 "192.168.1.8",                // Same IP as previous test
			userAgent:                 "Mozilla/5.0 (test-agent-8)", // Same user agent as previous test
			expectedWebsiteID:         baseWebsite.ID,
			expectSubdomainTracking:   true,
			expectedUserSignatureBase: "example.com", // Should use base domain for consistent user signature
			description:               "Same visitor across different subdomains should have consistent user signature based on base domain",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Clean up events from previous tests
			db.Exec("DELETE FROM ingested_events")

			// Run setup
			if tc.setup != nil {
				tc.setup(t)
			}

			// Create input for CollectEvent
			input := events.CollectEventInput{
				IPAddress:   tc.ipAddress,
				UserAgent:   tc.userAgent,
				ReferrerURL: "https://google.com/search",
				EventType:   events.EventTypePageView,
				Timestamp:   time.Now().UTC(),
				RawUrl:      "https://" + tc.hostname + "/test-page",
			}

			// Collect the event
			err := events.CollectEvent(dbManager, logger, &input)

			if tc.expectedWebsiteID == 0 {
				// Should fail
				assert.Error(t, err, tc.description)
				assert.Contains(t, err.Error(), "website not found", tc.description)

				// Verify no event was created
				var count int64
				err = db.Model(&events.IngestedEvent{}).Count(&count).Error
				require.NoError(t, err)
				assert.Equal(t, int64(0), count, "No event should be created when website is not found")
			} else {
				// Should succeed
				assert.NoError(t, err, tc.description)

				// Read back from DB to verify
				var savedEvent events.IngestedEvent
				err = db.First(&savedEvent).Error
				require.NoError(t, err, "Event should be saved in database")

				// Verify basic event properties
				assert.Equal(t, tc.expectedWebsiteID, savedEvent.WebsiteID, tc.description)
				assert.Equal(t, tc.hostname, savedEvent.Hostname, "Hostname should be preserved as-is")
				assert.Equal(t, "/test-page", savedEvent.Pathname, "Pathname should be extracted correctly")

				// Generate expected user signature for comparison
				privateKey := config.GetConfig().PrivateKey
				expectedUserSignature := visitors.BuildUniqueVisitorId(tc.expectedUserSignatureBase, tc.ipAddress, tc.userAgent, privateKey)

				// Verify user signature is generated correctly
				assert.Equal(t, expectedUserSignature, savedEvent.UserSignature,
					fmt.Sprintf("%s - User signature should be based on '%s'", tc.description, tc.expectedUserSignatureBase))

				// Additional verification: if this is a subdomain tracking case,
				// verify that the user signature would be different if generated with the subdomain
				if tc.expectSubdomainTracking {
					subdomainSignature := visitors.BuildUniqueVisitorId(tc.hostname, tc.ipAddress, tc.userAgent, privateKey)
					assert.NotEqual(t, subdomainSignature, savedEvent.UserSignature,
						"User signature should be based on base domain, not subdomain hostname")
				}
			}
		})
	}
}

func TestNormalizeOperatingSystem(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "macOS variations",
			input:    "Mac OS X",
			expected: "MacOS",
		},
		{
			name:     "macOS with darwin",
			input:    "Darwin",
			expected: "MacOS",
		},
		{
			name:     "Linux variations",
			input:    "GNU/Linux",
			expected: "Linux",
		},
		{
			name:     "Linux simple",
			input:    "Linux",
			expected: "Linux",
		},
		{
			name:     "iOS variations",
			input:    "iPhone OS",
			expected: "iOS",
		},
		{
			name:     "iOS simple",
			input:    "iOS",
			expected: "iOS",
		},
		{
			name:     "Android",
			input:    "Android",
			expected: "Android",
		},
		{
			name:     "Windows",
			input:    "Windows",
			expected: "Windows",
		},
		{
			name:     "Unknown OS",
			input:    "Unknown OS",
			expected: "Unknown os",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: events.UnknownOS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := events.NormalizeOperatingSystem(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
