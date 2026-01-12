package http

import (
	"github.com/gofiber/fiber/v2"
	"github.com/karloscodes/cartridge"
	"log/slog"

	"fusionaly/internal/onboarding"
)

// HomeIndexAction handles the root path with onboarding check
func HomeIndexAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	required, err := onboarding.IsOnboardingRequired(db)
	if err != nil {
		ctx.Logger.Error("Failed to check if onboarding is required", slog.Any("error", err))
		return ctx.Redirect("/login", fiber.StatusFound)
	}

	if required {
		ctx.Logger.Info("Root path accessed, redirecting to onboarding")
		return ctx.Redirect("/setup", fiber.StatusFound)
	}

	ctx.Logger.Info("Root path accessed, redirecting to login")
	return ctx.Redirect("/login", fiber.StatusFound)
}

// DemoIndexAction serves the demo page for E2E testing
func DemoIndexAction(ctx *cartridge.Context) error {
	return ctx.SendFile("demo.html")
}
