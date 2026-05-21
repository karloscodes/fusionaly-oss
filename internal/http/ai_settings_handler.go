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
		{
			"key":   "ai_base_url",
			"value": settings.GetAIBaseURL(db),
		},
		{
			"key":   "ai_model",
			"value": settings.GetAIModel(db),
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
	aiBaseURL := strings.TrimSpace(ctx.FormValue("ai_base_url"))
	aiModel := strings.TrimSpace(ctx.FormValue("ai_model"))
	if openAIKey == "" && aiBaseURL == "" && aiModel == "" {
		var body struct {
			OpenAIKey string `json:"openai_api_key"`
			AIBaseURL string `json:"ai_base_url"`
			AIModel   string `json:"ai_model"`
		}
		if err := ctx.BodyParser(&body); err == nil {
			openAIKey = strings.TrimSpace(body.OpenAIKey)
			aiBaseURL = strings.TrimSpace(body.AIBaseURL)
			aiModel = strings.TrimSpace(body.AIModel)
		}
	}

	// Persist provider settings (base URL + model). SaveAIBaseURL stores the
	// default when blank; SaveAIModel trims and stores as-is.
	if err := settings.SaveAIBaseURL(db, aiBaseURL); err != nil {
		ctx.Logger.Error("Failed to save AI base URL")
		flash.SetFlash(ctx.Ctx, "error", "Failed to save AI settings")
		return ctx.Redirect("/admin/administration/ai", fiber.StatusFound)
	}
	if err := settings.SaveAIModel(db, aiModel); err != nil {
		ctx.Logger.Error("Failed to save AI model")
		flash.SetFlash(ctx.Ctx, "error", "Failed to save AI settings")
		return ctx.Redirect("/admin/administration/ai", fiber.StatusFound)
	}

	// Don't save masked keys (user didn't change the existing value)
	if strings.HasPrefix(openAIKey, "****") {
		flash.SetFlash(ctx.Ctx, "info", "No changes made to AI settings")
		return ctx.Redirect("/admin/administration/ai", fiber.StatusFound)
	}

	if openAIKey != "" {
		// Only enforce OpenAI's "sk-" key format when pointed at OpenAI itself.
		// Other OpenAI-compatible providers (OpenRouter, local mocks) use their
		// own key formats.
		usingOpenAI := strings.Contains(settings.GetAIBaseURL(db), "api.openai.com")
		if usingOpenAI && !strings.HasPrefix(openAIKey, "sk-") {
			flash.SetFlash(ctx.Ctx, "error", "Invalid OpenAI API key format. Key should start with 'sk-'")
			return ctx.Redirect("/admin/administration/ai", fiber.StatusFound)
		}

		if err := settings.SaveOpenAIKey(db, openAIKey); err != nil {
			ctx.Logger.Error("Failed to save OpenAI API key")
			flash.SetFlash(ctx.Ctx, "error", "Failed to save AI settings")
			return ctx.Redirect("/admin/administration/ai", fiber.StatusFound)
		}
	}

	flash.SetFlash(ctx.Ctx, "success", "AI settings saved successfully")
	return ctx.Redirect("/admin/administration/ai", fiber.StatusFound)
}
