package http

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/karloscodes/cartridge"
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
		return ctx.FlashError("Invalid website ID").Redirect("/admin/websites", fiber.StatusFound)
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
		"models":             ai.AvailableModels,
	}

	return ctx.Inertia("Lens", props)
}

// WebsiteLensAskAIAction handles AI question submission (POST -> Inertia render)
func WebsiteLensAskAIAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	websiteIDStr := ctx.Params("id")
	websiteID, err := strconv.Atoi(websiteIDStr)
	if err != nil || websiteID <= 0 {
		return ctx.FlashError("Invalid website ID").Redirect("/admin/websites", fiber.StatusFound)
	}

	question := ctx.FormValue("query")
	// Empty model lets GetQueryFromOpenAI fall back to ai.DefaultModel.
	model := ctx.FormValue("model")
	if question == "" {
		return ctx.FlashError("Please enter a question").Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	openAIKey, err := ai.GetOpenAIApiKey(db)
	if err != nil || openAIKey == "" {
		return ctx.FlashError("OpenAI API key is not configured. Please configure it in AI Settings.").Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	aiResult, err := ai.GetQueryFromOpenAI(context.Background(), db, question, openAIKey, websiteID, model, ctx.Logger)
	if err != nil {
		ctx.Logger.Error("Failed to get query from OpenAI", slog.Any("error", err))
		return ctx.FlashError("Failed to generate query: "+err.Error()).Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	results, err := ai.ExecuteQuery(db, aiResult.SQL, aiResult.QueryType)
	if err != nil {
		ctx.Logger.Error("Failed to execute AI query", slog.Any("error", err))
		return ctx.FlashError("Query execution failed: "+err.Error()).Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
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
		"models":             ai.AvailableModels,
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

	return ctx.Inertia("Lens", props)
}

// WebsiteLensSaveAction saves an AI-generated query (POST -> Redirect)
func WebsiteLensSaveAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	websiteIDStr := ctx.Params("id")
	websiteID, err := strconv.Atoi(websiteIDStr)
	if err != nil || websiteID <= 0 {
		return ctx.FlashError("Invalid website ID").Redirect("/admin/websites", fiber.StatusFound)
	}

	title := ctx.FormValue("title")
	generatedSQL := ctx.FormValue("generated_sql")
	queryType := ctx.FormValue("query_type")
	vegaSpec := ctx.FormValue("vega_spec")
	model := ctx.FormValue("model")

	if title == "" || generatedSQL == "" {
		return ctx.FlashError("Title and SQL are required").Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	// A saved query re-executes on every Lens load, so validate it as read-only
	// before persisting — not just at execution time.
	if err := ai.ValidateReadOnlyQuery(generatedSQL); err != nil {
		return ctx.FlashError("Only read-only SELECT queries can be saved").Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	websiteIDUint := uint(websiteID)
	if _, err := ai.CreateSavedQueryWithVega(db, title, generatedSQL, vegaSpec, &websiteIDUint, queryType, model); err != nil {
		ctx.Logger.Error("Failed to save query", slog.Any("error", err))
		return ctx.FlashError("Failed to save query").Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	return ctx.FlashSuccess("Query saved successfully").Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
}

// WebsiteLensUpdateAction updates a saved query by regenerating SQL (POST -> Redirect)
func WebsiteLensUpdateAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	websiteIDStr := ctx.Params("id")
	websiteID, err := strconv.Atoi(websiteIDStr)
	if err != nil || websiteID <= 0 {
		return ctx.FlashError("Invalid website ID").Redirect("/admin/websites", fiber.StatusFound)
	}

	queryID, _ := strconv.Atoi(ctx.FormValue("id"))
	newTitle := ctx.FormValue("title")
	// Empty model lets GetQueryFromOpenAI fall back to ai.DefaultModel.
	model := ctx.FormValue("model")

	if queryID <= 0 || newTitle == "" {
		return ctx.FlashError("Invalid query ID or title").Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	savedQuery, err := ai.GetSavedQuery(db, uint(queryID))
	if err != nil {
		return ctx.FlashError("Query not found").Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	openAIKey, err := ai.GetOpenAIApiKey(db)
	if err != nil || openAIKey == "" {
		return ctx.FlashError("OpenAI API key is not configured").Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	aiResult, err := ai.GetQueryFromOpenAI(context.Background(), db, newTitle, openAIKey, websiteID, model, ctx.Logger)
	if err != nil {
		ctx.Logger.Error("Failed to regenerate query", slog.Any("error", err))
		return ctx.FlashError("Failed to regenerate query: "+err.Error()).Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	if err := ai.UpdateSavedQueryWithWebsiteAndVega(db, uint(queryID), newTitle, aiResult.SQL, aiResult.QueryType, aiResult.VegaSpec, model, savedQuery.WebsiteID); err != nil {
		ctx.Logger.Error("Failed to update query", slog.Any("error", err))
		return ctx.FlashError("Failed to update query").Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	return ctx.FlashSuccess("Query updated successfully").Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
}

// WebsiteLensDeleteAction deletes a saved query (POST -> Redirect)
func WebsiteLensDeleteAction(ctx *cartridge.Context) error {
	websiteIDStr := ctx.Params("id")
	websiteID, err := strconv.Atoi(websiteIDStr)
	if err != nil || websiteID <= 0 {
		return ctx.FlashError("Invalid website ID").Redirect("/admin/websites", fiber.StatusFound)
	}

	queryID, _ := strconv.Atoi(ctx.FormValue("id"))
	if queryID <= 0 {
		return ctx.FlashError("Invalid query ID").Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	if err := ai.DeleteSavedQuery(ctx.DB(), uint(queryID)); err != nil {
		ctx.Logger.Error("Failed to delete query", slog.Any("error", err))
		return ctx.FlashError("Failed to delete query").Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	return ctx.FlashSuccess("Query deleted successfully").Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
}

// WebsiteLensCloneAction clones a saved query (POST -> Redirect)
func WebsiteLensCloneAction(ctx *cartridge.Context) error {
	websiteIDStr := ctx.Params("id")
	websiteID, err := strconv.Atoi(websiteIDStr)
	if err != nil || websiteID <= 0 {
		return ctx.FlashError("Invalid website ID").Redirect("/admin/websites", fiber.StatusFound)
	}

	queryID, _ := strconv.Atoi(ctx.FormValue("id"))
	if queryID <= 0 {
		return ctx.FlashError("Invalid query ID").Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	if _, err := ai.CloneSavedQuery(ctx.DB(), uint(queryID)); err != nil {
		ctx.Logger.Error("Failed to clone query", slog.Any("error", err))
		return ctx.FlashError("Failed to clone query").Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
	}

	return ctx.FlashSuccess("Query cloned successfully").Redirect("/admin/websites/"+websiteIDStr+"/lens", fiber.StatusFound)
}
