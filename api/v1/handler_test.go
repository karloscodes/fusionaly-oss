// Package v1_test contains tests for the API v1 handlers
package v1_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"fusionaly/internal/visitors"
	"fusionaly/internal/websites"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"fusionaly/internal/config"
	"fusionaly/internal/events"
	"fusionaly/internal/settings"
	"fusionaly/internal/testsupport"
)

func TestGetSDKHandler(t *testing.T) {
	t.Run("uses https behind kamal-proxy", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		app := testsupport.CreateMinimalTestApp(t, dbManager.GetConnection())
		req := httptest.NewRequest("GET", "/y/api/v1/sdk.js", nil)
		req.Host = "analytics.example.com"
		req.RemoteAddr = "172.18.0.2:41234" // kamal-proxy in the Docker network
		req.Header.Set("X-Forwarded-Proto", "https")

		rec := httptest.NewRecorder() // app.Test would replace RemoteAddr

		app.ServeHTTP(rec, req)

		assert.Contains(t, rec.Body.String(), `host:"https://analytics.example.com"`)
	})

	t.Run("ignores X-Forwarded-Proto from a public peer", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		app := testsupport.CreateMinimalTestApp(t, dbManager.GetConnection())
		req := httptest.NewRequest("GET", "/y/api/v1/sdk.js", nil)
		req.Host = "analytics.example.com"
		req.RemoteAddr = "198.51.100.9:5000"
		req.Header.Set("X-Forwarded-Proto", "https")

		rec := httptest.NewRecorder() // app.Test would replace RemoteAddr

		app.ServeHTTP(rec, req)

		assert.Contains(t, rec.Body.String(), `host:"http://analytics.example.com"`)
	})

	t.Run("returns SDK with correct headers", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()

		app := testsupport.CreateMinimalTestApp(t, db)

		req := httptest.NewRequest("GET", "/y/api/v1/sdk.js", nil)

		resp, err := app.Test(req, 30000)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/javascript", resp.Header.Get("Content-Type"))
		assert.Equal(t, "public, max-age=3600", resp.Header.Get("Cache-Control"))
		assert.Equal(t, "cross-origin", resp.Header.Get("Cross-Origin-Resource-Policy"))
		assert.NotEmpty(t, resp.Header.Get("ETag"))
	})

	t.Run("returns 304 with CORP header when ETag matches", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()

		app := testsupport.CreateMinimalTestApp(t, db)

		// First request to get the ETag
		req := httptest.NewRequest("GET", "/y/api/v1/sdk.js", nil)
		resp, err := app.Test(req, 30000)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		etag := resp.Header.Get("ETag")
		require.NotEmpty(t, etag)

		// Second request with If-None-Match should return 304 with CORP header
		req2 := httptest.NewRequest("GET", "/y/api/v1/sdk.js", nil)
		req2.Header.Set("If-None-Match", etag)

		resp2, err := app.Test(req2, 30000)
		require.NoError(t, err)

		assert.Equal(t, http.StatusNotModified, resp2.StatusCode)
		assert.Equal(t, "cross-origin", resp2.Header.Get("Cross-Origin-Resource-Policy"))
		assert.Equal(t, "public, max-age=3600", resp2.Header.Get("Cache-Control"))
		assert.Equal(t, etag, resp2.Header.Get("ETag"))
	})
}

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
		req.Header.Set("Sec-Fetch-Site", "cross-site")

		resp, err := app.Test(req, 30000)
		require.NoError(t, err)

		if resp.StatusCode != http.StatusAccepted {
			respBody, _ := io.ReadAll(resp.Body)
			t.Logf("Response body: %s", string(respBody))
			t.Logf("Response status: %d", resp.StatusCode)
		}

		assert.Equal(t, http.StatusAccepted, resp.StatusCode)
		// The events route must expose Retry-After on every response, or the
		// browser hides it from the SDK when the handler answers 503.
		assert.Contains(t, resp.Header.Get("Access-Control-Expose-Headers"), "Retry-After")

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

	t.Run("accepts valid event with registered subdomain origin", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)

		// Website registered directly as a subdomain (not the base domain)
		website := testsupport.CreateTestWebsite(db, "blog.example.com")
		require.NotZero(t, website.ID, "Website ID should not be zero")

		app := testsupport.CreateMinimalTestApp(t, db)

		payload := map[string]interface{}{
			"url":           "https://blog.example.com/test",
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
		req.Header.Set("Origin", "https://blog.example.com")
		req.Header.Set("X-Forwarded-For", "127.0.0.1")
		req.Header.Set("Sec-Fetch-Site", "cross-site")

		resp, err := app.Test(req, 30000)
		require.NoError(t, err)

		if resp.StatusCode != http.StatusAccepted {
			respBody, _ := io.ReadAll(resp.Body)
			t.Logf("Response body: %s", string(respBody))
			t.Logf("Response status: %d", resp.StatusCode)
		}

		assert.Equal(t, http.StatusAccepted, resp.StatusCode)

		var count int64
		err = dbManager.GetConnection().Model(&events.IngestedEvent{}).Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), count, "Expected one event in the ingest database")
	})

	t.Run("accepts subdomain traffic for a base-domain website when subdomain tracking is enabled", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)

		// Website registered by its base/apex domain, with subdomain tracking turned on
		website := testsupport.CreateTestWebsite(db, "example.com")
		require.NotZero(t, website.ID, "Website ID should not be zero")
		require.NoError(t, settings.UpdateSubdomainTrackingSettings(db, "example.com", true))

		app := testsupport.CreateMinimalTestApp(t, db)

		payload := map[string]interface{}{
			"url":           "https://app.example.com/test",
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
		req.Header.Set("Origin", "https://app.example.com")
		req.Header.Set("X-Forwarded-For", "127.0.0.1")
		req.Header.Set("Sec-Fetch-Site", "cross-site")

		resp, err := app.Test(req, 30000)
		require.NoError(t, err)

		if resp.StatusCode != http.StatusAccepted {
			respBody, _ := io.ReadAll(resp.Body)
			t.Logf("Response body: %s", string(respBody))
			t.Logf("Response status: %d", resp.StatusCode)
		}

		assert.Equal(t, http.StatusAccepted, resp.StatusCode)

		var count int64
		err = dbManager.GetConnection().Model(&events.IngestedEvent{}).Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), count, "Expected one event in the ingest database")
	})

	t.Run("rejects subdomain traffic for a base-domain website when subdomain tracking is disabled", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)

		// Website registered by its base/apex domain, subdomain tracking left off (default)
		website := testsupport.CreateTestWebsite(db, "example.com")
		require.NotZero(t, website.ID, "Website ID should not be zero")

		app := testsupport.CreateMinimalTestApp(t, db)

		payload := map[string]interface{}{
			"url":           "https://app.example.com/test",
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
		req.Header.Set("Origin", "https://app.example.com")
		req.Header.Set("X-Forwarded-For", "127.0.0.1")
		req.Header.Set("Sec-Fetch-Site", "cross-site")

		resp, err := app.Test(req, 30000)
		require.NoError(t, err)

		assert.Equal(t, http.StatusForbidden, resp.StatusCode)

		var count int64
		err = dbManager.GetConnection().Model(&events.IngestedEvent{}).Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(0), count, "Expected no events in the ingest database")
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
		req.Header.Set("Sec-Fetch-Site", "cross-site")

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
		req.Header.Set("Sec-Fetch-Site", "cross-site")

		resp, err := app.Test(req, 30000)
		require.NoError(t, err)

		assert.Equal(t, http.StatusForbidden, resp.StatusCode)

		var count int64
		err = dbManager.GetConnection().Model(&events.IngestedEvent{}).Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(0), count, "Expected no events in the ingest database")
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
		assert.Equal(t, http.StatusTooEarly, resp.StatusCode)
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

	t.Run("resolves subdomain to base domain data when subdomain tracking is on", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)

		website := testsupport.CreateTestWebsite(db, "example.com")
		require.NotZero(t, website.ID)
		require.NoError(t, settings.UpdateSubdomainTrackingSettings(db, "example.com", true))

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

	t.Run("returns empty events for unregistered website without Origin", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)

		app := testsupport.CreateMinimalTestApp(t, db)

		req := httptest.NewRequest("GET", "/x/api/v1/me?url=https://unknown-domain.com", nil)
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
		assert.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"))
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

// Real browsers must never get a 403 from ingestion because of Sec-Fetch-Site.
// Safari before 16.4, older WebViews, and some privacy extensions send no
// header. This check came back twice (45e0b6b removed it, 942bf73 restored it)
// and dropped real visitors both times. Admin routes keep the strict check.
func TestIngestionAcceptsEveryBrowser(t *testing.T) {
	secFetchSites := []string{"", "cross-site", "same-site", "same-origin", "none"}
	routes := []string{"/x/api/v1/events", "/x/api/v1/events/beacon"}

	for _, route := range routes {
		for _, secFetchSite := range secFetchSites {
			name := secFetchSite
			if name == "" {
				name = "no header"
			}
			t.Run(route+" with Sec-Fetch-Site "+name, func(t *testing.T) {
				dbManager, _ := testsupport.SetupTestDBManager(t)
				db := dbManager.GetConnection()
				testsupport.CleanAllTables(db)
				testsupport.CreateTestWebsite(db, "example.com")
				app := testsupport.CreateMinimalTestApp(t, db)
				payload, err := json.Marshal(map[string]interface{}{
					"url":       "https://example.com/pricing",
					"timestamp": time.Now(),
					"eventType": events.EventTypePageView,
					"userAgent": "Mozilla/5.0 (iPhone; CPU iPhone OS 15_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.6 Mobile/15E148 Safari/604.1",
				})
				require.NoError(t, err)
				req := httptest.NewRequest("POST", route, bytes.NewReader(payload))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Origin", "https://example.com")
				req.Header.Set("X-Forwarded-For", "127.0.0.1")
				if secFetchSite != "" {
					req.Header.Set("Sec-Fetch-Site", secFetchSite)
				}

				resp, err := app.Test(req, 30000)

				require.NoError(t, err)
				assert.Equal(t, http.StatusAccepted, resp.StatusCode)
				var count int64
				require.NoError(t, db.Model(&events.IngestedEvent{}).Count(&count).Error)
				assert.Equal(t, int64(1), count, "Expected the event in the ingest database")
			})
		}
	}

	t.Run("admin routes still reject cross-site POSTs", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)
		app := testsupport.CreateMinimalTestApp(t, db)
		req := httptest.NewRequest("POST", "/admin/websites", strings.NewReader(`{"domain":"evil.com"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("Sec-Fetch-Site", "cross-site")

		resp, err := app.Test(req, 30000)

		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
}

func TestIngestionRejectsBadBodies(t *testing.T) {
	bodies := map[string]string{
		"an empty body":      `{}`,
		"a url without host": `{"url":"/pricing","eventType":1}`,
		"a url that is text": `{"url":"not a url","eventType":1}`,
	}

	for name, body := range bodies {
		t.Run("answers 400 for "+name, func(t *testing.T) {
			dbManager, _ := testsupport.SetupTestDBManager(t)
			db := dbManager.GetConnection()
			testsupport.CleanAllTables(db)
			testsupport.CreateTestWebsite(db, "example.com")
			app := testsupport.CreateMinimalTestApp(t, db)
			req := httptest.NewRequest("POST", "/x/api/v1/events", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", "https://example.com")

			resp, err := app.Test(req, 30000)

			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			var count int64
			require.NoError(t, db.Model(&events.IngestedEvent{}).Count(&count).Error)
			assert.Equal(t, int64(0), count, "Expected no stored event")
		})
	}
}

func TestIngestionEventTypes(t *testing.T) {
	routes := []string{"/x/api/v1/events", "/x/api/v1/events/beacon"}

	post := func(t *testing.T, route string, eventType int) (int, int64) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)
		testsupport.CreateTestWebsite(db, "example.com")
		app := testsupport.CreateMinimalTestApp(t, db)
		payload, err := json.Marshal(map[string]interface{}{
			"url":       "https://example.com/pricing",
			"timestamp": time.Now(),
			"eventType": eventType,
			"eventKey":  "signup",
			"userAgent": "Mozilla/5.0 (Test Agent)",
		})
		require.NoError(t, err)
		req := httptest.NewRequest("POST", route, bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("X-Forwarded-For", "127.0.0.1")

		resp, err := app.Test(req, 30000)

		require.NoError(t, err)
		var count int64
		require.NoError(t, db.Model(&events.IngestedEvent{}).Count(&count).Error)
		return resp.StatusCode, count
	}

	for _, route := range routes {
		t.Run(route, func(t *testing.T) {
			for _, eventType := range []int{0, 4, 99} {
				t.Run(fmt.Sprintf("rejects event type %d with 400", eventType), func(t *testing.T) {
					status, count := post(t, route, eventType)

					assert.Equal(t, http.StatusBadRequest, status)
					assert.Equal(t, int64(0), count, "Expected no stored event")
				})
			}

			known := []events.EventType{events.EventTypePageView, events.EventTypeCustomEvent, events.EventTypePageHide}
			for _, eventType := range known {
				t.Run(fmt.Sprintf("accepts event type %d", eventType), func(t *testing.T) {
					status, count := post(t, route, int(eventType))

					assert.Equal(t, http.StatusAccepted, status)
					assert.Equal(t, int64(1), count, "Expected the event in the ingest database")
				})
			}
		})
	}
}

func TestVisitorInfoOriginRules(t *testing.T) {
	visitedAt := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

	seedVisit := func(t *testing.T, db *gorm.DB, domain, path string) {
		t.Helper()
		website := testsupport.CreateTestWebsite(db, domain)
		signature := visitors.BuildUniqueVisitorId(domain, "1.2.3.4", "Origin-Agent", config.GetConfig().PrivateKey)
		event := events.Event{
			WebsiteID:        website.ID,
			UserSignature:    signature,
			Hostname:         domain,
			Pathname:         path,
			ReferrerHostname: events.DirectOrUnknownReferrer,
			EventType:        events.EventTypePageView,
			Timestamp:        visitedAt,
			CreatedAt:        visitedAt,
		}
		require.NoError(t, db.Create(&event).Error)
	}

	request := func(target, origin string) *http.Request {
		req := httptest.NewRequest("GET", target, nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		req.Header.Set("User-Agent", "Origin-Agent")
		req.Header.Set("X-Forwarded-For", "1.2.3.4")
		return req
	}

	t.Run("with Origin of a site that is not tracked, it gives no data", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)
		seedVisit(t, db, "example.com", "/private-page")
		app := testsupport.CreateMinimalTestApp(t, db)

		resp, err := app.Test(request("/x/api/v1/me?w=example.com&url=https://example.com/", "https://evil.com"), 30000)

		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"))
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.NotContains(t, string(body), "private-page")
		assert.NotContains(t, string(body), "visitorId")
	})

	t.Run("with Origin of a subdomain while subdomain tracking is off, it gives no data", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)
		seedVisit(t, db, "example.com", "/private-page")
		app := testsupport.CreateMinimalTestApp(t, db)

		resp, err := app.Test(request("/x/api/v1/me", "https://evil.example.com"), 30000)

		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"))
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.NotContains(t, string(body), "private-page")
	})

	t.Run("with Origin of a subdomain while subdomain tracking is on, it gives the site's data", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)
		seedVisit(t, db, "example.com", "/own-page")
		require.NoError(t, settings.UpdateSubdomainTrackingSettings(db, "example.com", true))
		app := testsupport.CreateMinimalTestApp(t, db)

		resp, err := app.Test(request("/x/api/v1/me", "https://blog.example.com"), 30000)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "https://blog.example.com", resp.Header.Get("Access-Control-Allow-Origin"))
	})

	t.Run("with Origin null, it gives no data", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)
		seedVisit(t, db, "example.com", "/private-page")
		app := testsupport.CreateMinimalTestApp(t, db)

		resp, err := app.Test(request("/x/api/v1/me?w=example.com", "null"), 30000)

		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"))
	})

	t.Run("with Origin of the tracked site, it gives only that site's data", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)
		seedVisit(t, db, "example.com", "/own-page")
		seedVisit(t, db, "other.com", "/other-page")
		app := testsupport.CreateMinimalTestApp(t, db)

		resp, err := app.Test(request("/x/api/v1/me?w=other.com", "https://example.com"), 30000)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "https://example.com", resp.Header.Get("Access-Control-Allow-Origin"))
		assert.Contains(t, resp.Header.Get("Vary"), "Origin")
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), "example.com/own-page")
		assert.NotContains(t, string(body), "other-page")
	})

	t.Run("with preflight from the tracked site, it allows that origin only", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)
		testsupport.CreateTestWebsite(db, "example.com")
		app := testsupport.CreateMinimalTestApp(t, db)
		allowed := httptest.NewRequest("OPTIONS", "/x/api/v1/me", nil)
		allowed.Header.Set("Origin", "https://example.com")
		allowed.Header.Set("Access-Control-Request-Method", "GET")
		denied := httptest.NewRequest("OPTIONS", "/x/api/v1/me", nil)
		denied.Header.Set("Origin", "https://evil.com")
		denied.Header.Set("Access-Control-Request-Method", "GET")

		allowedResp, err := app.Test(allowed, 30000)
		require.NoError(t, err)
		deniedResp, err := app.Test(denied, 30000)
		require.NoError(t, err)

		assert.Equal(t, "https://example.com", allowedResp.Header.Get("Access-Control-Allow-Origin"))
		assert.Empty(t, deniedResp.Header.Get("Access-Control-Allow-Origin"))
	})
}
