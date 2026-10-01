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

func TestGetWebsitesWithStats(t *testing.T) {
	t.Run("leaves page hides out of the event count", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)
		site := testsupport.CreateTestWebsite(db, "example.com")
		at := time.Now().UTC().Add(-time.Hour)
		require.NoError(t, db.Create(&[]events.Event{
			{WebsiteID: site.ID, UserSignature: "v1", Hostname: "example.com", Pathname: "/", EventType: events.EventTypePageView, Timestamp: at},
			{WebsiteID: site.ID, UserSignature: "v1", Hostname: "example.com", Pathname: "/", EventType: events.EventTypePageHide, Timestamp: at.Add(time.Minute)},
		}).Error)

		stats, err := websites.GetWebsitesWithStats(db, 30)

		require.NoError(t, err)
		require.Len(t, stats, 1)
		assert.Equal(t, int64(1), stats[0].EventCount)
	})
}

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

func TestCreateWebsite(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	testsupport.CleanAllTables(db)

	t.Run("creates a new domain", func(t *testing.T) {
		site := websites.Website{Domain: "new.example.com"}

		err := websites.CreateWebsite(db, &site)

		require.NoError(t, err)
		assert.NotZero(t, site.ID)
	})

	t.Run("rejects a domain that already exists", func(t *testing.T) {
		require.NoError(t, websites.CreateWebsite(db, &websites.Website{Domain: "dup.example.com"}))

		err := websites.CreateWebsite(db, &websites.Website{Domain: "dup.example.com"})

		assert.ErrorIs(t, err, websites.ErrWebsiteExists)
	})
}

func TestValidateDomain(t *testing.T) {
	t.Run("accepts host names", func(t *testing.T) {
		for _, domain := range []string{"example.com", "www.example.co.uk", "my-site.io", "localhost", "xn--bcher-kva.example"} {
			assert.NoError(t, websites.ValidateDomain(domain), domain)
		}
	})

	t.Run("rejects anything that is not a host name", func(t *testing.T) {
		for _, domain := range []string{"bad domain", "https://example.com", "example.com/path", "example.com:8080", "-example.com", "example..com", "<script>", ""} {
			assert.Error(t, websites.ValidateDomain(domain), domain)
		}
	})
}
