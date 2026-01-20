package v1

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/stretchr/testify/assert"

	"fusionaly/internal/events"
)

// secFetchSiteValidator is the same middleware used in production routes.go
// Duplicated here to test in isolation without full app setup.
func secFetchSiteValidator(c *fiber.Ctx) error {
	if c.Method() != fiber.MethodPost {
		return c.Next()
	}

	secFetchSite := c.Get("Sec-Fetch-Site")

	if secFetchSite == "" {
		slog.Warn("blocked_event",
			slog.String("reason", "missing_sec_fetch_site"),
			slog.String("origin", c.Get("Origin")),
			slog.String("ip", c.IP()),
			slog.String("user_agent", c.Get("User-Agent")),
			slog.String("path", c.Path()),
		)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "browser_required",
		})
	}

	switch secFetchSite {
	case "cross-site", "same-site", "same-origin":
		return c.Next()
	default:
		slog.Warn("blocked_event",
			slog.String("reason", "invalid_sec_fetch_site"),
			slog.String("value", secFetchSite),
			slog.String("origin", c.Get("Origin")),
			slog.String("ip", c.IP()),
			slog.String("user_agent", c.Get("User-Agent")),
			slog.String("path", c.Path()),
		)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "browser_required",
		})
	}
}

// TestSecFetchSiteProtection verifies that the Sec-Fetch-Site middleware
// blocks server-to-server requests while allowing legitimate browser requests
func TestSecFetchSiteProtection(t *testing.T) {
	app := fiber.New()
	app.Post("/x/api/v1/events", secFetchSiteValidator, func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	eventData := CreateEventParams{
		URL:       "https://example.com/page",
		Referrer:  "https://google.com",
		Timestamp: time.Now(),
		EventType: events.EventTypePageView,
		UserAgent: "Mozilla/5.0",
	}
	jsonPayload, _ := json.Marshal(eventData)

	tests := []struct {
		name               string
		secFetchSiteHeader string
		expectedStatus     int
		description        string
	}{
		{
			name:               "Allow cross-site browser request",
			secFetchSiteHeader: "cross-site",
			expectedStatus:     fiber.StatusOK,
			description:        "Legitimate browser request from tracked website",
		},
		{
			name:               "Allow same-site browser request",
			secFetchSiteHeader: "same-site",
			expectedStatus:     fiber.StatusOK,
			description:        "Browser request from same site (subdomain)",
		},
		{
			name:               "Allow same-origin browser request",
			secFetchSiteHeader: "same-origin",
			expectedStatus:     fiber.StatusOK,
			description:        "Browser request from same origin",
		},
		{
			name:               "Block none (direct navigation)",
			secFetchSiteHeader: "none",
			expectedStatus:     fiber.StatusForbidden,
			description:        "Direct navigation should be blocked for POST events",
		},
		{
			name:               "Block request without Sec-Fetch-Site",
			secFetchSiteHeader: "",
			expectedStatus:     fiber.StatusForbidden,
			description:        "Server-to-server request (curl, Postman, scripts) - BLOCKED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/x/api/v1/events", bytes.NewReader(jsonPayload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "Mozilla/5.0 (Test Browser)")
			req.Header.Set("Origin", "https://example.com")

			if tt.secFetchSiteHeader != "" {
				req.Header.Set("Sec-Fetch-Site", tt.secFetchSiteHeader)
			}

			resp, err := app.Test(req, -1)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode, tt.description)
		})
	}
}

// TestSecFetchSiteWithCORS verifies that 403 responses include CORS headers
// This was a bug where blocked requests had no CORS headers, causing browser errors
func TestSecFetchSiteWithCORS(t *testing.T) {
	app := fiber.New()

	// Apply CORS first (same order as production)
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "POST,GET,OPTIONS",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	app.Post("/x/api/v1/events", secFetchSiteValidator, func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	eventData := CreateEventParams{
		URL:       "https://example.com/page",
		Timestamp: time.Now(),
		EventType: events.EventTypePageView,
	}
	jsonPayload, _ := json.Marshal(eventData)

	t.Run("403 response includes CORS headers", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/x/api/v1/events", bytes.NewReader(jsonPayload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://example.com")
		// No Sec-Fetch-Site header - should get 403

		resp, err := app.Test(req, -1)
		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)

		// CORS headers must be present on 403 responses
		assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"),
			"403 response must include CORS headers so browser can read the error")
	})

	t.Run("OPTIONS preflight works", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/x/api/v1/events", nil)
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("Access-Control-Request-Method", "POST")

		resp, err := app.Test(req, -1)
		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusNoContent, resp.StatusCode)
		assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	})
}

// TestServerToServerBlocking demonstrates that common spoofing attempts are blocked
func TestServerToServerBlocking(t *testing.T) {
	app := fiber.New()
	app.Post("/x/api/v1/events", secFetchSiteValidator, func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	eventData := CreateEventParams{
		URL:       "https://example.com/page",
		Referrer:  "https://google.com",
		Timestamp: time.Now(),
		EventType: events.EventTypePageView,
		UserAgent: "curl/7.68.0",
	}
	jsonPayload, _ := json.Marshal(eventData)

	spoofingAttempts := []struct {
		name        string
		userAgent   string
		origin      string
		description string
	}{
		{
			name:        "curl request",
			userAgent:   "curl/7.68.0",
			origin:      "https://example.com",
			description: "curl command with spoofed Origin header",
		},
		{
			name:        "Postman request",
			userAgent:   "PostmanRuntime/7.29.0",
			origin:      "https://example.com",
			description: "Postman API client",
		},
		{
			name:        "Python requests",
			userAgent:   "python-requests/2.28.1",
			origin:      "https://example.com",
			description: "Python script using requests library",
		},
		{
			name:        "Node.js fetch",
			userAgent:   "node-fetch/1.0",
			origin:      "https://example.com",
			description: "Node.js server-side fetch",
		},
	}

	for _, attempt := range spoofingAttempts {
		t.Run(attempt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/x/api/v1/events", bytes.NewReader(jsonPayload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", attempt.userAgent)
			req.Header.Set("Origin", attempt.origin)
			// No Sec-Fetch-Site header - server-to-server requests can't set it

			resp, err := app.Test(req, -1)
			assert.NoError(t, err)
			assert.Equal(t, fiber.StatusForbidden, resp.StatusCode,
				"Should block %s", attempt.description)
		})
	}
}
