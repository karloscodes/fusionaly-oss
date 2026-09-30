package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/karloscodes/cartridge"
	"gorm.io/gorm"

	"fusionaly/internal/settings"
)

// AgentAPIKeyAuth middleware validates the API key for agent endpoints.
// Expects: Authorization: Bearer <api_key>
func AgentAPIKeyAuth(db *gorm.DB, logger *slog.Logger) cartridge.HandlerFunc {
	return func(c *cartridge.Context) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(http.StatusUnauthorized).JSON(cartridge.Map{
				"error": "Missing Authorization header",
			})
		}

		// Extract Bearer token
		if !strings.HasPrefix(authHeader, "Bearer ") {
			return c.Status(http.StatusUnauthorized).JSON(cartridge.Map{
				"error": "Invalid Authorization header format. Expected: Bearer <api_key>",
			})
		}

		providedKey := strings.TrimPrefix(authHeader, "Bearer ")
		if providedKey == "" {
			return c.Status(http.StatusUnauthorized).JSON(cartridge.Map{
				"error": "API key is empty",
			})
		}

		// Get stored API key
		storedKey, err := settings.GetAgentAPIKey(db)
		if err != nil || storedKey == "" {
			logger.Warn("Agent API key not configured", slog.Any("error", err))
			return c.Status(http.StatusUnauthorized).JSON(cartridge.Map{
				"error": "Agent API key not configured. Generate one in Administration settings.",
			})
		}

		// Constant-time comparison to prevent timing attacks
		if !secureCompare(providedKey, storedKey) {
			return c.Status(http.StatusUnauthorized).JSON(cartridge.Map{
				"error": "Invalid API key",
			})
		}

		return c.Next()
	}
}

// secureCompare performs constant-time string comparison
func secureCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}
