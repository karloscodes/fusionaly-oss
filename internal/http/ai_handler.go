package http

import (
	"context"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/karloscodes/cartridge"

	"fusionaly/internal/ai"
)

// AIAskQuestionAction handles POST /admin/api/ai/ask (JSON API)
func AIAskQuestionAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	var req struct {
		Question  string `json:"question"`
		WebsiteID uint   `json:"website_id"`
		Model     string `json:"model"`
	}

	if err := ctx.Ctx.BodyParser(&req); err != nil {
		return ctx.Ctx.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Question == "" {
		return ctx.Ctx.Status(400).JSON(fiber.Map{
			"error": "Question is required",
		})
	}

	if req.WebsiteID == 0 {
		return ctx.Ctx.Status(400).JSON(fiber.Map{
			"error": "Website ID is required",
		})
	}

	// Get OpenAI key from settings
	openAIKey, err := ai.GetOpenAIApiKey(db)
	if err != nil || openAIKey == "" {
		return ctx.Ctx.Status(400).JSON(fiber.Map{
			"error": "OpenAI API key is not configured",
		})
	}

	// Set a timeout for the AI request
	reqCtx, cancel := context.WithTimeout(ctx.Ctx.Context(), 60*time.Second)
	defer cancel()

	// Default to standard model if not specified
	model := req.Model
	if model == "" {
		model = ai.DefaultModel
	}

	aiResult, err := ai.GetQueryFromOpenAI(reqCtx, db, req.Question, openAIKey, int(req.WebsiteID), model, slog.Default())
	if err != nil {
		slog.Error("AI question failed", slog.Any("error", err), slog.Uint64("website_id", uint64(req.WebsiteID)))
		return ctx.Ctx.Status(500).JSON(fiber.Map{
			"error": "Failed to process AI request: " + err.Error(),
		})
	}

	// Execute the query
	results, err := ai.ExecuteQuery(db, aiResult.SQL, aiResult.QueryType)
	if err != nil {
		return ctx.Ctx.Status(500).JSON(fiber.Map{
			"error": "Query execution failed: " + err.Error(),
		})
	}

	return ctx.Ctx.JSON(fiber.Map{
		"question":   req.Question,
		"query":      aiResult.SQL,
		"results":    results,
		"query_type": aiResult.QueryType,
		"vega_spec":  aiResult.VegaSpec,
		"summary":    aiResult.Summary,
		"follow_ups": aiResult.FollowUps,
		"website_id": req.WebsiteID,
	})
}

// AISaveQueryAction handles POST /admin/api/ai/save (JSON API)
func AISaveQueryAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	var req struct {
		WebsiteID uint   `json:"website_id"`
		Question  string `json:"question"`
		Query     string `json:"query"`
		QueryType string `json:"query_type"`
		VegaSpec  string `json:"vega_spec,omitempty"`
		Model     string `json:"model,omitempty"`
	}

	if err := ctx.Ctx.BodyParser(&req); err != nil {
		return ctx.Ctx.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	websiteIDPtr := &req.WebsiteID
	saved, err := ai.CreateSavedQueryWithVega(db, req.Question, req.Query, req.VegaSpec, websiteIDPtr, req.QueryType, req.Model)
	if err != nil {
		slog.Error("Failed to save AI query", slog.Any("error", err), slog.Uint64("website_id", uint64(req.WebsiteID)))
		return ctx.Ctx.Status(500).JSON(fiber.Map{
			"error": "Failed to save query. Please try again.",
		})
	}

	return ctx.Ctx.JSON(saved)
}

// AIGetSavedQueriesAction handles GET /admin/api/ai/saved/:websiteId (JSON API)
func AIGetSavedQueriesAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	websiteID, err := ctx.Ctx.ParamsInt("websiteId")
	if err != nil || websiteID <= 0 {
		return ctx.Ctx.Status(400).JSON(fiber.Map{
			"error": "Invalid website ID",
		})
	}

	queries, err := ai.GetSavedQueriesByWebsiteID(db, uint(websiteID))
	if err != nil {
		slog.Error("Failed to get saved queries", slog.Any("error", err), slog.Int("website_id", websiteID))
		return ctx.Ctx.Status(500).JSON(fiber.Map{
			"error": "Failed to retrieve saved queries. Please try again.",
		})
	}

	return ctx.Ctx.JSON(fiber.Map{
		"queries": queries,
	})
}

// AIDeleteSavedQueryAction handles DELETE /admin/api/ai/saved/:id (JSON API)
func AIDeleteSavedQueryAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	id, err := ctx.Ctx.ParamsInt("id")
	if err != nil || id <= 0 {
		return ctx.Ctx.Status(400).JSON(fiber.Map{
			"error": "Invalid query ID",
		})
	}

	if err := ai.DeleteSavedQuery(db, uint(id)); err != nil {
		slog.Error("Failed to delete saved query", slog.Any("error", err), slog.Int("query_id", id))
		return ctx.Ctx.Status(500).JSON(fiber.Map{
			"error": "Failed to delete query. Please try again.",
		})
	}

	return ctx.Ctx.JSON(fiber.Map{
		"success": true,
	})
}

// AIGetStatusAction handles GET /admin/api/ai/status (JSON API)
func AIGetStatusAction(ctx *cartridge.Context) error {
	db := ctx.DB()
	openAIKey, _ := ai.GetOpenAIApiKey(db)
	return ctx.Ctx.JSON(fiber.Map{
		"configured": openAIKey != "",
	})
}
