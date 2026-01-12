// Package v1_test contains tests for the API v1 handlers
package v1_test

import (
	"bytes"
	"encoding/json"
	"fusionaly/internal/visitors"
	"fusionaly/internal/websites"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/config"
	"fusionaly/internal/events"
	"fusionaly/internal/testsupport"
)

func TestCreateEventPublicAPIHandler(t *testing.T) {
	t.Run("accepts valid event with registered origin", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)

		website := testsupport.CreateTestWebsite(db, "example.com")
		require.NotZero(t, website.ID, "Website ID should not be zero")

		app := testsupport.CreateMinimalTestApp(t, db)

		payload := map[string]interface{}{
			"url":           "https://example.com/test",
			"referrer":      "https://referer.com",
			"timestamp":     time.Now(),
			"eventType":     events.EventTypePageView,
			"eventKey":      "",
			"eventMetadata": map[string]interface{}{},
			"userAgent":     "Mozilla/5.0 (Test Agent)",
		}

		jsonPayload, err := json.Marshal(payload)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/x/api/v1/events", bytes.NewReader(jsonPayload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Test-Agent")
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("X-Forwarded-For", "127.0.0.1")
		req.Header.Set("Sec-Fetch-Site", "cross-site") // Required for browser-only validation

		resp, err := app.Test(req, 30000)
		require.NoError(t, err)

		if resp.StatusCode != http.StatusAccepted {
			respBody, _ := io.ReadAll(resp.Body)
			t.Logf("Response body: %s", string(respBody))
			t.Logf("Response status: %d", resp.StatusCode)
		}

		assert.Equal(t, http.StatusAccepted, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		if strings.Contains(string(body), "<html") {
			t.Logf("Response contains HTML error page: %s", string(body))
			t.FailNow()
		}

		var respBody map[string]interface{}
		err = json.Unmarshal(body, &respBody)
		require.NoError(t, err)

		assert.Equal(t, "Event added successfully", respBody["message"])
		assert.Equal(t, float64(http.StatusAccepted), respBody["status"])

		var count int64
		err = dbManager.GetConnection().Model(&events.IngestedEvent{}).Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), count, "Expected one event in the ingest database")
	})

	t.Run("rejects request from unregistered origin", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)

		// NOTE: No website created - origin validation should fail
		app := testsupport.CreateMinimalTestApp(t, db)

		payload := map[string]interface{}{
			"url":           "https://nonexistent-domain.com/test",
			"referrer":      "https://referer.com",
			"timestamp":     time.Now(),
			"eventType":     events.EventTypePageView,
			"eventKey":      "",
			"eventMetadata": map[string]interface{}{},
			"userAgent":     "Mozilla/5.0 (Test Agent)",
		}

		jsonPayload, err := json.Marshal(payload)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/x/api/v1/events", bytes.NewReader(jsonPayload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Test-Agent")
		req.Header.Set("Origin", "https://nonexistent-domain.com")
		req.Header.Set("X-Forwarded-For", "127.0.0.1")
		req.Header.Set("Sec-Fetch-Site", "cross-site") // Required for browser-only validation

		resp, err := app.Test(req, 30000)
		require.NoError(t, err)

		assert.Equal(t, http.StatusForbidden, resp.StatusCode)

		var count int64
		err = dbManager.GetConnection().Model(&events.IngestedEvent{}).Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(0), count, "Expected no events in the ingest database")
	})

	t.Run("rejects request without Origin header", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)

		website := testsupport.CreateTestWebsite(db, "example.com")
		require.NotZero(t, website.ID, "Website ID should not be zero")

		app := testsupport.CreateMinimalTestApp(t, db)

		payload := map[string]interface{}{
			"url":           "https://example.com/test",
			"referrer":      "https://referer.com",
			"timestamp":     time.Now(),
			"eventType":     events.EventTypePageView,
			"eventKey":      "",
			"eventMetadata": map[string]interface{}{},
			"userAgent":     "Mozilla/5.0 (Test Agent)",
		}

		jsonPayload, err := json.Marshal(payload)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/x/api/v1/events", bytes.NewReader(jsonPayload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Test-Agent")
		// No Origin header set
		req.Header.Set("X-Forwarded-For", "127.0.0.1")
		req.Header.Set("Sec-Fetch-Site", "cross-site") // Required for browser-only validation

		resp, err := app.Test(req, 30000)
		require.NoError(t, err)

		assert.Equal(t, http.StatusForbidden, resp.StatusCode)

		var count int64
		err = dbManager.GetConnection().Model(&events.IngestedEvent{}).Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(0), count, "Expected no events in the ingest database")
	})

	t.Run("rejects request without Sec-Fetch-Site header (server-to-server)", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)

		website := testsupport.CreateTestWebsite(db, "example.com")
		require.NotZero(t, website.ID, "Website ID should not be zero")

		app := testsupport.CreateMinimalTestApp(t, db)

		payload := map[string]interface{}{
			"url":           "https://example.com/test",
			"referrer":      "https://referer.com",
			"timestamp":     time.Now(),
			"eventType":     events.EventTypePageView,
			"eventKey":      "",
			"eventMetadata": map[string]interface{}{},
			"userAgent":     "Mozilla/5.0 (Test Agent)",
		}

		jsonPayload, err := json.Marshal(payload)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/x/api/v1/events", bytes.NewReader(jsonPayload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Test-Agent")
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("X-Forwarded-For", "127.0.0.1")
		// No Sec-Fetch-Site header - simulating server-to-server request

		resp, err := app.Test(req, 30000)
		require.NoError(t, err)

		assert.Equal(t, http.StatusForbidden, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var respBody map[string]interface{}
		err = json.Unmarshal(body, &respBody)
		require.NoError(t, err)

		assert.Equal(t, "forbidden", respBody["error"])
		assert.Equal(t, "browser requests only", respBody["message"])
	})
}

func TestGetVisitorInfoHandler(t *testing.T) {
	t.Run("returns 425 for early data replay", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)

		website := testsupport.CreateTestWebsite(db, "example.com")
		require.NotZero(t, website.ID)

		app := testsupport.CreateMinimalTestApp(t, db)

		req := httptest.NewRequest("GET", "/x/api/v1/me?w=example.com", nil)
		req.Header.Set("Early-Data", "1")

		resp, err := app.Test(req, 30000)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusTooEarly, resp.StatusCode)
	})

	t.Run("returns visitor info with events from processed table", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)

		website := testsupport.CreateTestWebsite(db, "example.com")
		require.NotZero(t, website.ID)

		var count int64
		db.Model(&websites.Website{}).Where("domain = ?", "example.com").Count(&count)

		cfg := config.GetConfig()
		signature := visitors.BuildUniqueVisitorId(
			"example.com",
			"1.2.3.4",
			"Mozilla/5.0 (Test Agent)",
			cfg.PrivateKey,
		)

		now := time.Now().UTC()
		eventsList := []events.Event{
			{
				WebsiteID:        website.ID,
				UserSignature:    signature,
				Hostname:         "example.com",
				Pathname:         "/latest",
				ReferrerHostname: events.DirectOrUnknownReferrer,
				EventType:        events.EventTypePageView,
				Timestamp:        now,
				CreatedAt:        now,
			},
			{
				WebsiteID:        website.ID,
				UserSignature:    signature,
				Hostname:         "example.com",
				Pathname:         "/first",
				ReferrerHostname: "news.ycombinator.com",
				ReferrerPathname: "/story",
				EventType:        events.EventTypeCustomEvent,
				CustomEventName:  "signup",
				Timestamp:        now.Add(-5 * time.Minute),
				CreatedAt:        now.Add(-5 * time.Minute),
			},
		}
		require.NoError(t, db.Create(&eventsList).Error)

		app := testsupport.CreateMinimalTestApp(t, db)

		req := httptest.NewRequest("GET", "/x/api/v1/me?url=https://example.com/path", nil)
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Test Agent)")
		req.Header.Set("X-Forwarded-For", "1.2.3.4")

		resp, err := app.Test(req, 30000)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var payload map[string]interface{}
		err = json.Unmarshal(body, &payload)
		require.NoError(t, err)

		signature, ok := payload["visitorId"].(string)
		require.True(t, ok)
		assert.NotEmpty(t, signature)

		user, ok := payload["visitorAlias"].(string)
		require.True(t, ok)
		assert.Equal(t, visitors.VisitorAlias(signature), user)
		assert.NotEmpty(t, payload["visitorId"])

		if country, ok := payload["country"].(string); ok {
			assert.Equal(t, events.UnknownCountry, country)
		}

		_, hasGeneratedAt := payload["generatedAt"].(string)
		assert.True(t, hasGeneratedAt, "generatedAt should be present")

		// Verify sensitive fields are not exposed
		_, exists := payload["websiteId"]
		assert.False(t, exists)
		_, exists = payload["domain"]
		assert.False(t, exists)
		_, exists = payload["requestHost"]
		assert.False(t, exists)
		_, exists = payload["privacyMode"]
		assert.False(t, exists)
		_, exists = payload["ipAddress"]
		assert.False(t, exists)
		_, exists = payload["userAgent"]
		assert.False(t, exists)
		_, exists = payload["userSignature"]
		assert.False(t, exists)

		eventsRaw, ok := payload["events"].([]interface{})
		require.True(t, ok)
		assert.Len(t, eventsRaw, 2)

		firstEvent := eventsRaw[0].(map[string]interface{})
		assert.Equal(t, "example.com/latest", firstEvent["url"])
		assert.Equal(t, float64(events.EventTypePageView), firstEvent["eventType"])
		_, hasReferrer := firstEvent["referrer"]
		assert.False(t, hasReferrer)

		secondEvent := eventsRaw[1].(map[string]interface{})
		assert.Equal(t, "example.com/first", secondEvent["url"])
		assert.Equal(t, float64(events.EventTypeCustomEvent), secondEvent["eventType"])
		assert.Equal(t, "signup", secondEvent["customEventKey"])
		assert.Equal(t, "news.ycombinator.com/story", secondEvent["referrer"])
	})

	t.Run("falls back to Host header when no context headers provided", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)

		website := testsupport.CreateTestWebsite(db, "example.com")
		require.NotZero(t, website.ID)

		cfg := config.GetConfig()
		signature := visitors.BuildUniqueVisitorId("example.com", "5.6.7.8", "Host-Fallback-Agent", cfg.PrivateKey)

		now := time.Now().UTC()
		event := events.Event{
			WebsiteID:        website.ID,
			UserSignature:    signature,
			Hostname:         "example.com",
			Pathname:         "/fallback",
			ReferrerHostname: events.DirectOrUnknownReferrer,
			EventType:        events.EventTypePageView,
			Timestamp:        now,
			CreatedAt:        now,
		}
		require.NoError(t, db.Create(&event).Error)

		app := testsupport.CreateMinimalTestApp(t, db)

		req := httptest.NewRequest("GET", "/x/api/v1/me", nil)
		req.Host = "example.com"
		req.Header.Set("User-Agent", "Host-Fallback-Agent")
		req.Header.Set("X-Forwarded-For", "5.6.7.8")

		resp, err := app.Test(req, 30000)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var payload map[string]interface{}
		err = json.Unmarshal(body, &payload)
		require.NoError(t, err)

		assert.Equal(t, signature, payload["visitorId"])
		assert.Equal(t, visitors.VisitorAlias(signature), payload["visitorAlias"])
	})

	t.Run("resolves subdomain to base domain data", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)

		website := testsupport.CreateTestWebsite(db, "example.com")
		require.NotZero(t, website.ID)

		cfg := config.GetConfig()
		signature := visitors.BuildUniqueVisitorId("example.com", "7.8.9.0", "Subdomain-Agent", cfg.PrivateKey)

		now := time.Now().UTC()
		event := events.Event{
			WebsiteID:        website.ID,
			UserSignature:    signature,
			Hostname:         "example.com",
			Pathname:         "/base",
			ReferrerHostname: events.DirectOrUnknownReferrer,
			EventType:        events.EventTypePageView,
			Timestamp:        now,
			CreatedAt:        now,
		}
		require.NoError(t, db.Create(&event).Error)

		app := testsupport.CreateMinimalTestApp(t, db)

		req := httptest.NewRequest("GET", "/x/api/v1/me?url=https://app.example.com/base", nil)
		req.Host = "app.example.com"
		req.Header.Set("User-Agent", "Subdomain-Agent")
		req.Header.Set("X-Forwarded-For", "7.8.9.0")

		resp, err := app.Test(req, 30000)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var payload map[string]interface{}
		err = json.Unmarshal(body, &payload)
		require.NoError(t, err)

		assert.Equal(t, signature, payload["visitorId"])
		assert.Equal(t, visitors.VisitorAlias(signature), payload["visitorAlias"])

		eventsRaw, ok := payload["events"].([]interface{})
		require.True(t, ok)
		assert.Len(t, eventsRaw, 1)
	})

	t.Run("honors website query parameter", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)

		website := testsupport.CreateTestWebsite(db, "example.com")
		require.NotZero(t, website.ID)

		cfg := config.GetConfig()
		signature := visitors.BuildUniqueVisitorId("example.com", "4.3.2.1", "Param-Agent", cfg.PrivateKey)

		now := time.Now().UTC()
		event := events.Event{
			WebsiteID:        website.ID,
			UserSignature:    signature,
			Hostname:         "example.com",
			Pathname:         "/param",
			ReferrerHostname: events.DirectOrUnknownReferrer,
			EventType:        events.EventTypePageView,
			Timestamp:        now,
			CreatedAt:        now,
		}
		require.NoError(t, db.Create(&event).Error)

		app := testsupport.CreateMinimalTestApp(t, db)

		req := httptest.NewRequest("GET", "/x/api/v1/me?w=example.com&url=https://admin.example.com/param", nil)
		req.Host = "admin.example.com"
		req.Header.Set("User-Agent", "Param-Agent")
		req.Header.Set("X-Forwarded-For", "4.3.2.1")

		resp, err := app.Test(req, 30000)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var payload map[string]interface{}
		err = json.Unmarshal(body, &payload)
		require.NoError(t, err)

		assert.Equal(t, signature, payload["visitorId"])
		assert.Equal(t, visitors.VisitorAlias(signature), payload["visitorAlias"])

		eventsRaw, ok := payload["events"].([]interface{})
		require.True(t, ok)
		assert.Len(t, eventsRaw, 1)
	})

	t.Run("returns empty events for unregistered website", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)

		app := testsupport.CreateMinimalTestApp(t, db)

		req := httptest.NewRequest("GET", "/x/api/v1/me?url=https://unknown-domain.com", nil)
		req.Header.Set("Origin", "https://unknown-domain.com")
		req.Header.Set("User-Agent", "Test-Agent")
		req.Header.Set("X-Forwarded-For", "1.2.3.4")

		resp, err := app.Test(req, 30000)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var payload map[string]interface{}
		err = json.Unmarshal(body, &payload)
		require.NoError(t, err)

		assert.NotEmpty(t, payload["visitorId"])
		assert.NotEmpty(t, payload["visitorAlias"])

		eventsRaw, ok := payload["events"].([]interface{})
		require.True(t, ok)
		assert.Len(t, eventsRaw, 0)
	})

	t.Run("falls back to ingested events when processed events empty", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)

		website := testsupport.CreateTestWebsite(db, "example.com")
		require.NotZero(t, website.ID)

		cfg := config.GetConfig()
		signature := visitors.BuildUniqueVisitorId("example.com", "9.9.9.9", "Fallback-Agent", cfg.PrivateKey)

		now := time.Now().UTC()
		ingested := events.IngestedEvent{
			WebsiteID:        website.ID,
			UserSignature:    signature,
			Hostname:         "example.com",
			Pathname:         "/ingested",
			ReferrerHostname: events.DirectOrUnknownReferrer,
			EventType:        events.EventTypePageView,
			Timestamp:        now,
			UserAgent:        "Fallback-Agent",
			RawURL:           "https://example.com/ingested",
			Processed:        0,
			CreatedAt:        now,
		}
		require.NoError(t, db.Create(&ingested).Error)

		app := testsupport.CreateMinimalTestApp(t, db)

		req := httptest.NewRequest("GET", "/x/api/v1/me?url=https://example.com/ingested", nil)
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("User-Agent", "Fallback-Agent")
		req.Header.Set("X-Forwarded-For", "9.9.9.9")

		resp, err := app.Test(req, 30000)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var payload map[string]interface{}
		err = json.Unmarshal(body, &payload)
		require.NoError(t, err)

		eventsRaw, ok := payload["events"].([]interface{})
		require.True(t, ok)
		assert.Len(t, eventsRaw, 1)

		event := eventsRaw[0].(map[string]interface{})
		assert.Equal(t, "example.com/ingested", event["url"])
		assert.Equal(t, float64(events.EventTypePageView), event["eventType"])
	})
}
