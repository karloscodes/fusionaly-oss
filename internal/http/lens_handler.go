package http

import (
	"github.com/gofiber/fiber/v2"
	"log/slog"
	"gorm.io/gorm"

	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/inertia"
	"fusionaly/internal/websites"
)

// WebsiteLensAction renders the Lens page (Pro feature paywall in OSS)
func WebsiteLensAction(ctx *cartridge.Context) error {
	websiteId, err := ctx.ParamsInt("id")
	if err != nil {
		ctx.Logger.Error("Invalid website ID in URL", slog.Any("error", err))
		return ctx.Redirect("/admin/websites", fiber.StatusFound)
	}

	db := ctx.DB()

	// Get website to verify it exists and get domain
	website, err := websites.GetWebsiteByID(db, uint(websiteId))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.Logger.Warn("Website not found", slog.Int("websiteId", websiteId))
			return ctx.Redirect("/admin/websites", fiber.StatusFound)
		}
		ctx.Logger.Error("Failed to get website", slog.Any("error", err))
		return ctx.Redirect("/admin/websites", fiber.StatusFound)
	}

	// Get websites for selector
	websitesData, err := websites.GetDistinctWebsites(db)
	if err != nil {
		ctx.Logger.Error("failed to get websites", slog.Any("error", err))
		websitesData = []websites.Website{}
	}

	return inertia.RenderPage(ctx.Ctx, "Lens", inertia.Props{
		"current_website_id": websiteId,
		"website_domain":     website.Domain,
		"websites":           websitesData,
	})
}
