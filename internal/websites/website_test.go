package websites_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/events"
	"fusionaly/internal/settings"
	"fusionaly/internal/websites"
	"fusionaly/internal/testsupport"
)

func TestGetWebsiteOrNotFound(t *testing.T) {
	// Set up test database
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	testsupport.CleanAllTables(db)

	// Create test website
	testWebsite := testsupport.CreateTestWebsite(db, "example.com")

	t.Run("Exact hostname match", func(t *testing.T) {
		websiteID, err := websites.GetWebsiteOrNotFound(db, "example.com")

		assert.NoError(t, err)
		assert.Equal(t, testWebsite.ID, websiteID)
	})

	t.Run("No match for non-existent domain", func(t *testing.T) {
		websiteID, err := websites.GetWebsiteOrNotFound(db, "unknown-domain.com")

		assert.Error(t, err)
		assert.Equal(t, uint(0), websiteID)

		// Check that it's the right type of error
		var websiteNotFoundErr *websites.WebsiteNotFoundError
		assert.ErrorAs(t, err, &websiteNotFoundErr)
		assert.Equal(t, "unknown-domain.com", websiteNotFoundErr.Domain)
	})
}

func TestStripSubdomains(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		expected string
	}{
		{
			name:     "Simple subdomain",
			hostname: "www.example.com",
			expected: "example.com",
		},
		{
			name:     "Multiple subdomains",
			hostname: "api.v1.example.com",
			expected: "example.com",
		},
		{
			name:     "No subdomain",
			hostname: "example.com",
			expected: "example.com",
		},
		{
			name:     "Country code TLD",
			hostname: "www.example.co.uk",
			expected: "example.co.uk",
		},
		{
			name:     "Localhost",
			hostname: "localhost",
			expected: "localhost",
		},
		{
			name:     "Single part domain",
			hostname: "example",
			expected: "example",
		},
		{
			name:     "Localhost subdomain",
			hostname: "sub.localhost",
			expected: "localhost",
		},
		{
			name:     "Deep localhost subdomain",
			hostname: "api.v1.localhost",
			expected: "localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Since stripSubdomains is not exported, we test it indirectly
			// by using the CollectEvent function which uses prepareTempEvent
			// which uses stripSubdomains for subdomain fallback

			// Set up test database
			dbManager, logger := testsupport.SetupTestDBManager(t)
			db := dbManager.GetConnection()
			testsupport.CleanAllTables(db)

			// Create a base domain website
			testsupport.CreateTestWebsite(db, tt.expected)

			// Initialize default settings to ensure cache is set up
			err := settings.SetupDefaultSettings(db)
			require.NoError(t, err)

			// Enable subdomain tracking for testing
			err = settings.UpdateSubdomainTrackingSettings(db, tt.expected, true)
			require.NoError(t, err)

			// Test that the hostname resolves to the base domain through CollectEvent
			input := &events.CollectEventInput{
				EventType:   events.EventTypePageView,
				UserAgent:   "Mozilla/5.0 (Test Agent)",
				IPAddress:   "192.168.1.1",
				RawUrl:      "https://" + tt.hostname + "/test",
				ReferrerURL: "",
				Timestamp:   time.Now(),
			}

			err = events.CollectEvent(dbManager, logger, input)

			if tt.expected == "localhost" {
				// Localhost should work with subdomain fallback when tracking is enabled
				assert.NoError(t, err)
			} else if tt.expected == "example" {
				// Single-word domains should only match exact hostnames
				if tt.hostname == tt.expected {
					assert.NoError(t, err)
				} else {
					assert.Error(t, err)
				}
			} else {
				// Regular domains should work with subdomain fallback
				assert.NoError(t, err)
			}
		})
	}
}
