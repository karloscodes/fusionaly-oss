package v1

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	cartridgemiddleware "github.com/karloscodes/cartridge/middleware"
	"github.com/stretchr/testify/assert"

	"fusionaly/internal/events"
)

// TestSecFetchSiteProtection verifies that the Sec-Fetch-Site middleware
// blocks server-to-server requests while allowing legitimate browser requests
func TestSecFetchSiteProtection(t *testing.T) {
	// Create a minimal Fiber app with the middleware
	app := fiber.New()

	// Apply the same middlewares as production (strict check + validation)
	strictSecFetchCheck := func(c *fiber.Ctx) error {
		if c.Method() == "POST" && c.Get("Sec-Fetch-Site") == "" {
			return c.SendStatus(fiber.StatusForbidden)
		}
		return c.Next()
	}

	secFetchForEvents := cartridgemiddleware.SecFetchSiteMiddleware(cartridgemiddleware.SecFetchSiteConfig{
		AllowedValues: []string{"cross-site", "same-site", "same-origin", "none"},
		Methods:       []string{"POST"},
	})

	// Mock handler that just returns 200
	app.Post("/x/api/v1/events", strictSecFetchCheck, secFetchForEvents, func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	// Create test event payload
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
			name:               "Allow none (direct navigation)",
			secFetchSiteHeader: "none",
			expectedStatus:     fiber.StatusOK,
			description:        "Direct navigation (rare for POST but valid)",
		},
		{
			name:               "Block request without Sec-Fetch-Site (server-to-server)",
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

			// Set Sec-Fetch-Site header (or omit it to simulate server-to-server)
			if tt.secFetchSiteHeader != "" {
				req.Header.Set("Sec-Fetch-Site", tt.secFetchSiteHeader)
			}

			resp, err := app.Test(req, -1)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode, tt.description)

			if tt.expectedStatus == fiber.StatusForbidden {
				t.Logf("✅ Successfully blocked: %s", tt.description)
			} else {
				t.Logf("✅ Successfully allowed: %s", tt.description)
			}
		})
	}
}

// TestServerToServerBlocking demonstrates that common spoofing attempts are blocked
func TestServerToServerBlocking(t *testing.T) {
	app := fiber.New()

	// Apply the same middlewares as production
	strictSecFetchCheck := func(c *fiber.Ctx) error {
		if c.Method() == "POST" && c.Get("Sec-Fetch-Site") == "" {
			return c.SendStatus(fiber.StatusForbidden)
		}
		return c.Next()
	}

	secFetchForEvents := cartridgemiddleware.SecFetchSiteMiddleware(cartridgemiddleware.SecFetchSiteConfig{
		AllowedValues: []string{"cross-site", "same-site", "same-origin", "none"},
		Methods:       []string{"POST"},
	})

	app.Post("/x/api/v1/events", strictSecFetchCheck, secFetchForEvents, func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	eventData := CreateEventParams{
		URL:       "https://example.com/page",
		Referrer:  "https://google.com",
		Timestamp: time.Now(),
		EventType: events.EventTypePageView,
		UserAgent: "curl/7.68.0", // Spoofed user agent
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
		{
			name:        "wget request",
			userAgent:   "Wget/1.20.3",
			origin:      "https://example.com",
			description: "wget command",
		},
	}

	for _, attempt := range spoofingAttempts {
		t.Run(attempt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/x/api/v1/events", bytes.NewReader(jsonPayload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", attempt.userAgent)
			req.Header.Set("Origin", attempt.origin) // Spoofed Origin

			// Note: Sec-Fetch-Site is NOT set (server-to-server requests can't set it)

			resp, err := app.Test(req, -1)
			assert.NoError(t, err)
			assert.Equal(t, fiber.StatusForbidden, resp.StatusCode,
				"Should block %s", attempt.description)

			t.Logf("✅ Successfully blocked: %s", attempt.description)
		})
	}
}
