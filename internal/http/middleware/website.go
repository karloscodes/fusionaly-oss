package middleware

import (
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"fusionaly/internal/websites"
)

// WebsiteFilter sets the website_id in the request context.
// Dependencies are injected via the factory function for clean architecture.
func WebsiteFilter(db *gorm.DB, logger *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Try to get website_id from query param or header
		websiteIDStr := c.Query("website_id", c.Get("X-Website-ID"))
		if websiteIDStr != "" {
			websiteID, err := strconv.ParseInt(websiteIDStr, 10, 64)
			if err != nil {
				logger.Warn("Invalid website_id provided",
					slog.String("website_id", websiteIDStr),
					slog.Any("error", err))
				return c.Status(fiber.StatusBadRequest).SendString("Invalid website_id")
			}
			c.Locals("website_id", int(websiteID))
			logger.Debug("Applied website filter", slog.Int("website_id", c.Locals("website_id").(int)))
		} else {
			logger.Debug("No website_id provided, attempting to set default website")
			firstWebsite, err := websites.GetFirstWebsite(db)
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					logger.Debug("No websites found in database - continuing without website_id")
					// Don't set website_id, let individual controllers handle this case
				} else {
					logger.Error("Failed to get first website for default", slog.Any("error", err))
					// Don't set website_id, let individual controllers handle this case
				}
			} else {
				c.Locals("website_id", int(firstWebsite.ID))
				logger.Debug("Set default website", slog.Int("website_id", int(firstWebsite.ID)), slog.String("domain", firstWebsite.Domain))
			}
		}

		return c.Next()
	}
}
