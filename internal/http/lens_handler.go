package http

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/flash"
	"github.com/karloscodes/cartridge/inertia"
	"gorm.io/gorm"

	"fusionaly/internal/ai"
	"fusionaly/internal/websites"
)

// InitResultItem represents a query result for the initial_results prop
type InitResultItem struct {
	ID        uint                     `json:"id"`
	Results   []map[string]interface{} `json:"results,omitempty"`
	Error     string                   `json:"error,omitempty"`
	QueryType string                   `json:"query_type,omitempty"`
	VegaSpec  string                   `json:"vega_spec,omitempty"`
}

// AIResultProp represents the AI result to pass back to the frontend
type AIResultProp struct {
	Question  string                   `json:"question"`
	Query     string                   `json:"query"`
	Results   []map[string]interface{} `json:"results"`
	QueryType string                   `json:"queryType"`
	VegaSpec  string                   `json:"vegaSpec,omitempty"`
	Summary   string                   `json:"summary,omitempty"`
	FollowUps []string                 `json:"followUps,omitempty"`
	WebsiteID int                      `json:"websiteId"`
}

// buildInitialResults executes all saved queries and returns their results
func buildInitialResults(db *gorm.DB, savedQueries []ai.SavedQuery) []InitResultItem {
	// Always return empty slice, never nil (frontend expects array)
	results := make([]InitResultItem, 0)
	for _, sq := range savedQueries {
		item := InitResultItem{
			ID:        sq.ID,
			QueryType: sq.QueryType,
			VegaSpec:  sq.VegaSpec,
		}

		queryResults, err := ai.ExecuteQuery(db, sq.GeneratedSQL, sq.QueryType)
		if err != nil {
			item.Error = err.Error()
		} else {
			item.Results = queryResults
		}

		results = append(results, item)
	}
	return results
}

// lensWebsiteDomain looks up a website domain by ID, returning "" on failure.
func lensWebsiteDomain(db *gorm.DB, websiteID int) string {
	website, err := websites.GetWebsiteByID(db, uint(websiteID))
	if err != nil {
		return ""
	}
	return website.Domain
}

// WebsiteLensAction renders the Lens page (uses the "Lens" Inertia component)
func WebsiteLensAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	websiteID, err := strconv.Atoi(ctx.Params("id"))
	if err != nil || websiteID <= 0 {
		flash.SetFlash(ctx.Ctx, "error", "Invalid website ID")
		return ctx.Redirect("/admin/websites", fiber.StatusFound)
	}

	// Get saved queries for this website
	savedQueries, err := ai.GetSavedQueriesByWebsiteID(db, uint(websiteID))
	if err != nil {
		ctx.Logger.Error("Failed to get saved queries", slog.Any("error", err))
		savedQueries = []ai.SavedQuery{}
	}

	// Execute all saved queries to get initial results
	initialResults := buildInitialResults(db, savedQueries)

	// Get websites for selector
	websitesData, err := websites.GetDistinctWebsites(db)
	if err != nil {
		ctx.Logger.Error("failed to get websites", slog.Any("error", err))
		websitesData = []websites.Website{}
	}

	// Determine whether the OpenAI key is configured so the page can render
	// its no-key empty state instead of the Ask UI.
	openAIKey, _ := ai.GetOpenAIApiKey(db)

	props := inertia.Props{
		"current_website_id": websiteID,
		"website_domain":     lensWebsiteDomain(db, websiteID),
		"websites":           websitesData,
		"saved_queries":      savedQueries,
		"initial_results":    initialResults,
		"ai_configured":      openAIKey != "",
	}

	return inertia.RenderPage(ctx.Ctx, "Lens", props)
}

// WebsiteLensAskAIAction handles AI question submission (POST -> Inertia render)
func WebsiteLensAskAIAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	websiteIDStr := ctx.Params("id")
	websiteID, err := strconv.Atoi(websiteIDStr)
	if err != nil || websiteID <= 0 {
		flash.SetFlash(ctx.Ctx, "error", "Invalid website ID")
		return ctx.Redirect("/admin/websites", fiber.StatusFound)
	}

	question := ctx.FormValue("query")
	model := ctx.FormValue("model")
	if model == "" {
		model = ai.DefaultModel
	}
	if question == "" {
		flash.SetFlash(ctx.Ctx, "error", "Please enter a question")
		return ctx.Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	openAIKey, err := ai.GetOpenAIApiKey(db)
	if err != nil || openAIKey == "" {
		flash.SetFlash(ctx.Ctx, "error", "OpenAI API key is not configured. Please configure it in AI Settings.")
		return ctx.Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	aiResult, err := ai.GetQueryFromOpenAI(context.Background(), db, question, openAIKey, websiteID, model, ctx.Logger)
	if err != nil {
		ctx.Logger.Error("Failed to get query from OpenAI", slog.Any("error", err))
		flash.SetFlash(ctx.Ctx, "error", "Failed to generate query: "+err.Error())
		return ctx.Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	results, err := ai.ExecuteQuery(db, aiResult.SQL, aiResult.QueryType)
	if err != nil {
		ctx.Logger.Error("Failed to execute AI query", slog.Any("error", err))
		flash.SetFlash(ctx.Ctx, "error", "Query execution failed: "+err.Error())
		return ctx.Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	// Cache the successful result (best-effort)
	if err := ai.SetCachedQuery(db, question, websiteID, model, aiResult.SQL, aiResult.QueryType, aiResult.VegaSpec, results); err != nil {
		ctx.Logger.Warn("Failed to cache AI query result", slog.Any("error", err))
	}

	savedQueries, _ := ai.GetSavedQueriesByWebsiteID(db, uint(websiteID))
	initialResults := buildInitialResults(db, savedQueries)

	if results == nil {
		results = make([]map[string]interface{}, 0)
	}

	websitesData, _ := websites.GetDistinctWebsites(db)

	props := inertia.Props{
		"current_website_id": websiteID,
		"website_domain":     lensWebsiteDomain(db, websiteID),
		"websites":           websitesData,
		"saved_queries":      savedQueries,
		"initial_results":    initialResults,
		"ai_configured":      true,
		"ai_result": AIResultProp{
			Question:  question,
			Query:     aiResult.SQL,
			Results:   results,
			QueryType: aiResult.QueryType,
			VegaSpec:  aiResult.VegaSpec,
			Summary:   aiResult.Summary,
			FollowUps: aiResult.FollowUps,
			WebsiteID: websiteID,
		},
	}

	return inertia.RenderPage(ctx.Ctx, "Lens", props)
}

// WebsiteLensSaveAction saves an AI-generated query (POST -> Redirect)
func WebsiteLensSaveAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	websiteIDStr := ctx.Params("id")
	websiteID, err := strconv.Atoi(websiteIDStr)
	if err != nil || websiteID <= 0 {
		flash.SetFlash(ctx.Ctx, "error", "Invalid website ID")
		return ctx.Redirect("/admin/websites", fiber.StatusFound)
	}

	title := ctx.FormValue("title")
	generatedSQL := ctx.FormValue("generated_sql")
	queryType := ctx.FormValue("query_type")
	vegaSpec := ctx.FormValue("vega_spec")
	model := ctx.FormValue("model")

	if title == "" || generatedSQL == "" {
		flash.SetFlash(ctx.Ctx, "error", "Title and SQL are required")
		return ctx.Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	// A saved query re-executes on every Lens load, so validate it as read-only
	// before persisting — not just at execution time.
	if err := ai.ValidateReadOnlyQuery(generatedSQL); err != nil {
		flash.SetFlash(ctx.Ctx, "error", "Only read-only SELECT queries can be saved")
		return ctx.Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	websiteIDUint := uint(websiteID)
	if _, err := ai.CreateSavedQueryWithVega(db, title, generatedSQL, vegaSpec, &websiteIDUint, queryType, model); err != nil {
		ctx.Logger.Error("Failed to save query", slog.Any("error", err))
		flash.SetFlash(ctx.Ctx, "error", "Failed to save query")
		return ctx.Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	flash.SetFlash(ctx.Ctx, "success", "Query saved successfully")
	return ctx.Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
}

// WebsiteLensUpdateAction updates a saved query by regenerating SQL (POST -> Redirect)
func WebsiteLensUpdateAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	websiteIDStr := ctx.Params("id")
	websiteID, err := strconv.Atoi(websiteIDStr)
	if err != nil || websiteID <= 0 {
		flash.SetFlash(ctx.Ctx, "error", "Invalid website ID")
		return ctx.Redirect("/admin/websites", fiber.StatusFound)
	}

	queryID, _ := strconv.Atoi(ctx.FormValue("id"))
	newTitle := ctx.FormValue("title")
	model := ctx.FormValue("model")
	if model == "" {
		model = ai.DefaultModel
	}

	if queryID <= 0 || newTitle == "" {
		flash.SetFlash(ctx.Ctx, "error", "Invalid query ID or title")
		return ctx.Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	savedQuery, err := ai.GetSavedQuery(db, uint(queryID))
	if err != nil {
		flash.SetFlash(ctx.Ctx, "error", "Query not found")
		return ctx.Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	openAIKey, err := ai.GetOpenAIApiKey(db)
	if err != nil || openAIKey == "" {
		flash.SetFlash(ctx.Ctx, "error", "OpenAI API key is not configured")
		return ctx.Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	aiResult, err := ai.GetQueryFromOpenAI(context.Background(), db, newTitle, openAIKey, websiteID, model, ctx.Logger)
	if err != nil {
		ctx.Logger.Error("Failed to regenerate query", slog.Any("error", err))
		flash.SetFlash(ctx.Ctx, "error", "Failed to regenerate query: "+err.Error())
		return ctx.Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	if err := ai.UpdateSavedQueryWithWebsiteAndVega(db, uint(queryID), newTitle, aiResult.SQL, aiResult.QueryType, aiResult.VegaSpec, model, savedQuery.WebsiteID); err != nil {
		ctx.Logger.Error("Failed to update query", slog.Any("error", err))
		flash.SetFlash(ctx.Ctx, "error", "Failed to update query")
		return ctx.Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	flash.SetFlash(ctx.Ctx, "success", "Query updated successfully")
	return ctx.Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
}

// WebsiteLensDeleteAction deletes a saved query (POST -> Redirect)
func WebsiteLensDeleteAction(ctx *cartridge.Context) error {
	websiteIDStr := ctx.Params("id")
	websiteID, err := strconv.Atoi(websiteIDStr)
	if err != nil || websiteID <= 0 {
		flash.SetFlash(ctx.Ctx, "error", "Invalid website ID")
		return ctx.Redirect("/admin/websites", fiber.StatusFound)
	}

	queryID, _ := strconv.Atoi(ctx.FormValue("id"))
	if queryID <= 0 {
		flash.SetFlash(ctx.Ctx, "error", "Invalid query ID")
		return ctx.Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	if err := ai.DeleteSavedQuery(ctx.DB(), uint(queryID)); err != nil {
		ctx.Logger.Error("Failed to delete query", slog.Any("error", err))
		flash.SetFlash(ctx.Ctx, "error", "Failed to delete query")
		return ctx.Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	flash.SetFlash(ctx.Ctx, "success", "Query deleted successfully")
	return ctx.Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
}

// WebsiteLensCloneAction clones a saved query (POST -> Redirect)
func WebsiteLensCloneAction(ctx *cartridge.Context) error {
	websiteIDStr := ctx.Params("id")
	websiteID, err := strconv.Atoi(websiteIDStr)
	if err != nil || websiteID <= 0 {
		flash.SetFlash(ctx.Ctx, "error", "Invalid website ID")
		return ctx.Redirect("/admin/websites", fiber.StatusFound)
	}

	queryID, _ := strconv.Atoi(ctx.FormValue("id"))
	if queryID <= 0 {
		flash.SetFlash(ctx.Ctx, "error", "Invalid query ID")
		return ctx.Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	if _, err := ai.CloneSavedQuery(ctx.DB(), uint(queryID)); err != nil {
		ctx.Logger.Error("Failed to clone query", slog.Any("error", err))
		flash.SetFlash(ctx.Ctx, "error", "Failed to clone query")
		return ctx.Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	flash.SetFlash(ctx.Ctx, "success", "Query cloned successfully")
	return ctx.Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
}
