package middleware

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"fusionaly/internal/onboarding"
)

// OnboardingCheck middleware redirects to setup if onboarding is required.
// This should be applied to routes that require the system to be set up.
// Dependencies are injected via the factory function for clean architecture.
func OnboardingCheck(db *gorm.DB, logger *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {

		// Check if onboarding is required
		required, err := onboarding.IsOnboardingRequired(db)
		if err != nil {
			logger.Error("Failed to check if onboarding is required in middleware", slog.Any("error", err))
			return c.Status(fiber.StatusInternalServerError).SendString("System error")
		}

		if required {
			// If this is an API request, return JSON
			if c.Get("Accept") == "application/json" || c.Get("Content-Type") == "application/json" {
				return c.Status(fiber.StatusPreconditionRequired).JSON(fiber.Map{
					"error":     "System setup required",
					"setup_url": "/setup",
				})
			}

			// Otherwise, redirect to setup page
			logger.Info("Onboarding required, redirecting to setup",
				slog.String("path", c.Path()),
				slog.String("method", c.Method()))
			return c.Redirect("/setup", fiber.StatusFound)
		}

		// Continue to next middleware/handler
		return c.Next()
	}
}
