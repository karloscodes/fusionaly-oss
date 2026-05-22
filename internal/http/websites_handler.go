package http

import (
	"encoding/json"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"log/slog"

	"fusionaly/internal/events"
	"fusionaly/internal/settings"
	"fusionaly/internal/websites"
	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/inertia"
)

// WebsitesIndexAction handles listing all websites (Inertia)
func WebsitesIndexAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	// Get all websites with event count statistics
	websitesWithCounts, err := websites.GetWebsitesWithStats(db, 30)
	if err != nil {
		ctx.Logger.Error("Failed to get websites with stats", slog.Any("error", err))
		return ctx.FlashError("Failed to load websites").Redirect("/admin", fiber.StatusFound)
	}

	// If no websites exist, redirect to the creation page
	if len(websitesWithCounts) == 0 {
		ctx.Logger.Info("No websites found - redirecting to website creation")
		return ctx.Redirect("/admin/websites/new", fiber.StatusFound)
	}

	return ctx.Inertia("Websites", inertia.Props{
		"title":    "Websites",
		"websites": websitesWithCounts,
	})
}

// WebsiteNewPageAction handles showing the website creation form page (Inertia)
func WebsiteNewPageAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	// Fetch available websites for the AI overlay dropdown
	websitesData, err := websites.GetWebsitesForSelector(db)
	if err != nil {
		ctx.Logger.Error("failed to get websites", slog.Any("error", err))
		// Continue with the page rendering even if website fetch fails
		websitesData = []map[string]interface{}{} // Ensure it's an empty slice, not nil
	}

	return ctx.Inertia("WebsiteNew", inertia.Props{
		"title":    "New Website",
		"websites": websitesData,
	})
}

// WebsiteCreateAction handles creating a new website (form submission)
func WebsiteCreateAction(ctx *cartridge.Context) error {
	// Log form submission details for debugging
	ctx.Logger.Info("Received website creation form submission",
		slog.String("method", ctx.Method()),
		slog.String("content_type", ctx.Get("Content-Type")),
		slog.String("raw_body", string(ctx.Body())),
		slog.String("csrf_token", ctx.FormValue("_csrf")),
		slog.String("domain", ctx.FormValue("domain")),
	)

	// Parse form data - Bind is content-type aware (form-encoded or Inertia.js JSON)
	var in struct {
		Domain string `json:"domain" form:"domain"`
	}
	_ = ctx.Bind(&in)
	domain := in.Domain

	// Log form values
	ctx.Logger.Info("Form values",
		slog.String("domain", domain),
	)

	// Validate domain
	if domain == "" {
		ctx.Logger.Warn("Domain field is empty")
		return ctx.FlashError("Domain is required").Redirect("/admin/websites/new", fiber.StatusFound)
	}

	db := ctx.DB()

	// Create website
	website := websites.Website{
		Domain: domain,
	}

	// Log attempt to create website
	ctx.Logger.Info("Attempting to create website", slog.String("domain", domain))

	if err := websites.CreateWebsite(db, &website); err != nil {
		ctx.Logger.Error("Failed to create website", slog.Any("error", err), slog.String("domain", domain))
		return ctx.FlashError("Failed to create website: "+err.Error()).Redirect("/admin/websites/new", fiber.StatusFound)
	}

	// Log success
	ctx.Logger.Info("Website created successfully",
		slog.Uint64("id", uint64(website.ID)),
		slog.String("domain", website.Domain))

	// Success - redirect to setup page
	return ctx.FlashSuccess("Website created successfully").Redirect("/admin/websites/"+strconv.Itoa(int(website.ID))+"/setup", fiber.StatusFound)
}

// WebsiteSetupPageAction handles showing the website setup page after creation (Inertia)
func WebsiteSetupPageAction(ctx *cartridge.Context) error {
	// Get website ID from params
	id, err := ctx.ParamsInt("id")
	if err != nil {
		ctx.Logger.Error("Invalid website ID", slog.Any("error", err))
		return ctx.Redirect("/admin", fiber.StatusFound)
	}

	db := ctx.DB()

	// Get website
	website, err := websites.GetWebsiteByID(db, uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.FlashError("Website not found").Redirect("/admin", fiber.StatusFound)
		}
		ctx.Logger.Error("Failed to get website", slog.Any("error", err), slog.Int("id", id))
		return ctx.Redirect("/admin", fiber.StatusFound)
	}

	return ctx.Inertia("WebsiteSetup", inertia.Props{
		"website": inertia.Props{
			"id":     website.ID,
			"domain": website.Domain,
		},
	})
}

// WebsiteEditPageAction handles showing the website edit form (Inertia)
func WebsiteEditPageAction(ctx *cartridge.Context) error {
	// Get website ID from params
	id, err := ctx.ParamsInt("id")
	if err != nil {
		ctx.Logger.Error("Invalid website ID", slog.Any("error", err))
		return ctx.FlashError("Invalid website ID").Redirect("/admin", fiber.StatusFound)
	}

	db := ctx.DB()

	// Get website
	website, err := websites.GetWebsiteByID(db, uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.FlashError("Website not found").Redirect("/admin", fiber.StatusFound)
		}
		ctx.Logger.Error("Failed to get website", slog.Any("error", err), slog.Int("id", id))
		return ctx.FlashError("Failed to load website").Redirect("/admin", fiber.StatusFound)
	}

	// Fetch all distinct events for this website
	allDistinctEvents, err := events.GetDistinctEventNamesForWebsite(db, uint(id), 30)
	if err != nil {
		ctx.Logger.Error("Failed to fetch distinct events for website", slog.Any("error", err), slog.Int("id", id))
		// Don't fail the page, just provide empty events
		allDistinctEvents = []events.EventNameInfo{}
	}

	// Fetch current conversion goals for this website
	conversionGoals, err := settings.GetWebsiteGoals(db, uint(id))
	if err != nil {
		ctx.Logger.Error("Failed to fetch goals for website", slog.Any("error", err), slog.Int("id", id))
		// Don't fail the page, just provide empty goals
		conversionGoals = []string{}
	}

	// Fetch subdomain tracking setting for this website
	subdomainTrackingEnabled := settings.IsSubdomainTrackingEnabled(db, website.Domain)

	return ctx.Inertia("WebsiteEdit", inertia.Props{
		"title":                      "Edit Website",
		"website":                    website,
		"all_distinct_events":        allDistinctEvents,
		"conversion_goals":           conversionGoals,
		"subdomain_tracking_enabled": subdomainTrackingEnabled,
	})
}

// WebsiteUpdateAction handles updating a website (form submission)
func WebsiteUpdateAction(ctx *cartridge.Context) error {
	// Get website ID from params
	id, err := ctx.ParamsInt("id")
	if err != nil {
		ctx.Logger.Error("Invalid website ID", slog.Any("error", err))
		return ctx.FlashError("Invalid website ID").Redirect("/admin", fiber.StatusFound)
	}

	// Parse form data - Bind is content-type aware (form-encoded or Inertia.js JSON)
	var in struct {
		ConversionGoals          string `json:"conversion_goals" form:"conversion_goals"`
		SubdomainTrackingEnabled string `json:"subdomain_tracking_enabled" form:"subdomain_tracking_enabled"`
	}
	_ = ctx.Bind(&in)
	conversionGoalsJSON := in.ConversionGoals
	subdomainTrackingEnabledStr := in.SubdomainTrackingEnabled

	subdomainTrackingEnabled := subdomainTrackingEnabledStr == "true"

	db := ctx.DB()

	// Get website
	website, err := websites.GetWebsiteByID(db, uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.Logger.Warn("Website not found", slog.Int("id", id))
			return ctx.FlashError("Website not found").Redirect("/admin", fiber.StatusFound)
		}
		ctx.Logger.Error("Failed to get website", slog.Any("error", err), slog.Int("id", id))
		return ctx.FlashError("Failed to update website").Redirect("/admin", fiber.StatusFound)
	}

	ctx.Logger.Info("Updating website settings",
		slog.String("domain", website.Domain),
		slog.Bool("subdomain_tracking", subdomainTrackingEnabled))

	// Handle conversion goals update
	if conversionGoalsJSON != "" {
		ctx.Logger.Info("Processing conversion goals JSON", slog.String("json", conversionGoalsJSON))
		var goals []string
		if err := json.Unmarshal([]byte(conversionGoalsJSON), &goals); err != nil {
			ctx.Logger.Error("Failed to parse conversion goals", slog.Any("error", err), slog.String("json", conversionGoalsJSON))
			return ctx.FlashError("Invalid conversion goals format").Redirect("/admin/websites/"+strconv.Itoa(id)+"/edit", fiber.StatusFound)
		}

		ctx.Logger.Info("Parsed goals", slog.Any("goals", goals))

		// Save goals for this website
		if err := settings.SaveWebsiteGoals(db, uint(id), goals); err != nil {
			ctx.Logger.Error("Failed to save conversion goals", slog.Any("error", err), slog.Int("id", id))
			return ctx.FlashError("Failed to save conversion goals").Redirect("/admin/websites/"+strconv.Itoa(id)+"/edit", fiber.StatusFound)
		}
	} else {
		ctx.Logger.Warn("No conversion goals JSON provided in form submission")
	}

	// Handle subdomain tracking setting
	ctx.Logger.Info("Processing subdomain tracking setting", slog.Bool("enabled", subdomainTrackingEnabled), slog.String("domain", website.Domain))
	if err := settings.UpdateSubdomainTrackingSettings(db, website.Domain, subdomainTrackingEnabled); err != nil {
		ctx.Logger.Error("Failed to update subdomain tracking setting", slog.Any("error", err), slog.String("domain", website.Domain))
		return ctx.FlashError("Failed to update subdomain tracking setting").Redirect("/admin/websites/"+strconv.Itoa(id)+"/edit", fiber.StatusFound)
	}

	// Success - redirect back to the edit page
	return ctx.FlashSuccess("Website updated successfully").Redirect("/admin/websites/"+strconv.Itoa(id)+"/edit", fiber.StatusFound)
}

// WebsiteDeleteAction handles deleting a website (form submission)
func WebsiteDeleteAction(ctx *cartridge.Context) error {
	// Get website ID from params
	id, err := ctx.ParamsInt("id")
	if err != nil {
		ctx.Logger.Error("Invalid website ID", slog.Any("error", err))
		return ctx.FlashError("Invalid website ID").Redirect("/admin", fiber.StatusFound)
	}

	db := ctx.DB()

	// Delete website
	if err := websites.DeleteWebsite(db, uint(id)); err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.FlashError("Website not found").Redirect("/admin", fiber.StatusFound)
		}
		ctx.Logger.Error("Failed to delete website", slog.Any("error", err), slog.Int("id", id))
		return ctx.FlashError("Failed to delete website").Redirect("/admin", fiber.StatusFound)
	}

	// Success - redirect to websites list
	return ctx.FlashSuccess("Website deleted successfully").Redirect("/admin", fiber.StatusFound)
}
