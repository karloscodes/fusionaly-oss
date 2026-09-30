package middleware

import (
	"log/slog"
	"net/http"

	"github.com/karloscodes/cartridge"
	"gorm.io/gorm"

	"fusionaly/internal/onboarding"
)

// OnboardingCheck middleware redirects to setup if onboarding is required.
// This should be applied to routes that require the system to be set up.
// Dependencies are injected via the factory function for clean architecture.
func OnboardingCheck(db *gorm.DB, logger *slog.Logger) cartridge.HandlerFunc {
	return func(c *cartridge.Context) error {

		// Check if onboarding is required
		required, err := onboarding.IsOnboardingRequired(db)
		if err != nil {
			logger.Error("Failed to check if onboarding is required in middleware", slog.Any("error", err))
			return c.Status(http.StatusInternalServerError).SendString("System error")
		}

		if required {
			// If this is an API request, return JSON
			if c.Get("Accept") == "application/json" || c.Get("Content-Type") == "application/json" {
				return c.Status(http.StatusPreconditionRequired).JSON(cartridge.Map{
					"error":     "System setup required",
					"setup_url": "/setup",
				})
			}

			// Otherwise, redirect to setup page
			logger.Info("Onboarding required, redirecting to setup",
				slog.String("path", c.Path()),
				slog.String("method", c.Method()))
			return c.Redirect("/setup", http.StatusFound)
		}

		// Continue to next middleware/handler
		return c.Next()
	}
}

// SetupOnly blocks the setup wizard once an admin exists. Without it, anyone
// could walk the wizard again and create a second admin account.
func SetupOnly(db *gorm.DB, logger *slog.Logger) cartridge.HandlerFunc {
	return func(c *cartridge.Context) error {
		required, err := onboarding.IsOnboardingRequired(db)
		if err != nil {
			logger.Error("Failed to check if onboarding is required in middleware", slog.Any("error", err))
			return c.Status(http.StatusInternalServerError).SendString("System error")
		}

		if !required {
			return c.Redirect("/login", http.StatusFound)
		}

		return c.Next()
	}
}
