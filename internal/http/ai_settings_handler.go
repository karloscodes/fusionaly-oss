package http

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/flash"
	"github.com/karloscodes/cartridge/inertia"

	"fusionaly/internal/settings"
)

// AISettingsPageAction renders the AI settings administration page (GET)
func AISettingsPageAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	// Get current OpenAI API key (masked for display)
	openAIKey, _ := settings.GetOpenAIKey(db)
	maskedKey := ""
	if openAIKey != "" {
		// Show asterisks to indicate a key is set
		maskedKey = "********" + openAIKey[max(0, len(openAIKey)-4):]
	}

	settingsData := []map[string]string{
		{
			"key":   "openai_api_key",
			"value": maskedKey,
		},
	}

	return inertia.RenderPage(ctx.Ctx, "AdministrationAI", inertia.Props{
		"settings": settingsData,
	})
}

// AISettingsFormAction handles POST form submission for AI settings (PRG pattern)
func AISettingsFormAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	// The frontend posts via the vanilla Inertia protocol (useForm/router.post),
	// which sends a JSON body that FormValue can't read. Try form-encoded first,
	// then fall back to the Inertia JSON body (same as the geolite form).
	openAIKey := strings.TrimSpace(ctx.FormValue("openai_api_key"))
	if openAIKey == "" {
		var body struct {
			OpenAIKey string `json:"openai_api_key"`
		}
		if err := ctx.BodyParser(&body); err == nil {
			openAIKey = strings.TrimSpace(body.OpenAIKey)
		}
	}

	// Don't save masked keys (user didn't change the existing value)
	if strings.HasPrefix(openAIKey, "****") {
		flash.SetFlash(ctx.Ctx, "info", "No changes made to AI settings")
		return ctx.Redirect("/admin/administration/ai", fiber.StatusFound)
	}

	if openAIKey != "" {
		// No provider-specific key-format check: OpenRouter (the default) and
		// other OpenAI-compatible providers use their own key formats. We only
		// require a non-empty key, which is already guaranteed here.
		if err := settings.SaveOpenAIKey(db, openAIKey); err != nil {
			ctx.Logger.Error("Failed to save OpenAI API key")
			flash.SetFlash(ctx.Ctx, "error", "Failed to save AI settings")
			return ctx.Redirect("/admin/administration/ai", fiber.StatusFound)
		}
	}

	flash.SetFlash(ctx.Ctx, "success", "AI settings saved successfully")
	return ctx.Redirect("/admin/administration/ai", fiber.StatusFound)
}
