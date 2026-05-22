package http

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/inertia"

	"fusionaly/internal/config"
	"fusionaly/internal/jobs"
	"fusionaly/internal/settings"
	"fusionaly/internal/websites"
	"github.com/karloscodes/cartridge/cache"
)

// SystemExportDatabaseAction exports the SQLite database file
func SystemExportDatabaseAction(ctx *cartridge.Context) error {
	// Type-assert to get fusionaly-specific config fields
	cfg := ctx.Config.(*config.Config)

	// Get the full database file path from config
	dbPath := cfg.DatabaseName
	if dbPath == "" {
		// Fallback to constructed path if not set
		dbPath = filepath.Join(cfg.DatabasePath, fmt.Sprintf("%s-%s.db", cfg.AppName, cfg.Environment))
	}

	// Check if database file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		ctx.Logger.Error("Database file not found", slog.String("path", dbPath))
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Database file not found",
		})
	}

	// Open the database file
	file, err := os.Open(dbPath)
	if err != nil {
		ctx.Logger.Error("Failed to open database file", slog.Any("error", err))
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to read database file",
		})
	}
	defer file.Close()

	// Get file info for size
	fileInfo, err := file.Stat()
	if err != nil {
		ctx.Logger.Error("Failed to get database file info", slog.Any("error", err))
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to get database file info",
		})
	}

	// Set headers for file download
	ctx.Set("Content-Type", "application/octet-stream")
	ctx.Set("Content-Disposition", "attachment; filename=fusionaly-backup.db")
	ctx.Set("Content-Length", string(rune(fileInfo.Size())))

	ctx.Logger.Info("Database exported", slog.String("path", dbPath), slog.Int64("size", fileInfo.Size()))

	// Stream the file to the response
	_, err = io.Copy(ctx.Response().BodyWriter(), file)
	if err != nil {
		ctx.Logger.Error("Failed to stream database file", slog.Any("error", err))
		return err
	}

	return nil
}

// AdministrationIndexAction redirects to the first administration page
func AdministrationIndexAction(ctx *cartridge.Context) error {
	return ctx.Redirect("/admin/administration/ingestion", fiber.StatusFound)
}

// AdministrationIngestionPageAction renders the Ingestion administration page
func AdministrationIngestionPageAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	// Get settings for SSR using settings context
	settingsData, err := settings.GetAllSettingsForDisplay(db)
	if err != nil {
		ctx.Logger.Error("Failed to fetch settings", slog.Any("error", err))
		settingsData = []settings.SettingResponse{}
	}

	// Fetch websites for the selector
	websitesData, err := websites.GetWebsitesForSelector(db)
	if err != nil {
		ctx.Logger.Error("Failed to fetch websites for selector", slog.Any("error", err))
		websitesData = []map[string]interface{}{}
	}

	return ctx.Inertia("AdministrationIngestion", inertia.Props{
		"settings": settingsData,
		"websites": websitesData,
	})
}

// AdministrationAccountPageAction renders the Account administration page
func AdministrationAccountPageAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	// Get settings for SSR using settings context
	settingsData, err := settings.GetAllSettingsForDisplay(db)
	if err != nil {
		ctx.Logger.Error("Failed to fetch settings", slog.Any("error", err))
		settingsData = []settings.SettingResponse{}
	}

	// Fetch websites for the selector
	websitesData, err := websites.GetWebsitesForSelector(db)
	if err != nil {
		ctx.Logger.Error("Failed to fetch websites for selector", slog.Any("error", err))
		websitesData = []map[string]interface{}{}
	}

	return ctx.Inertia("AdministrationAccount", inertia.Props{
		"settings": settingsData,
		"websites": websitesData,
	})
}

// AdministrationAgentsPageAction renders the Agents administration page
func AdministrationAgentsPageAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	// Fetch websites for the selector
	websitesData, err := websites.GetWebsitesForSelector(db)
	if err != nil {
		websitesData = []map[string]interface{}{}
	}

	// Get Agent API key (masked for display, last 4 chars visible)
	agentAPIKey, _ := settings.GetAgentAPIKey(db)
	var maskedAPIKey string
	if agentAPIKey != "" {
		if len(agentAPIKey) > 4 {
			maskedAPIKey = "••••••••••••••••••••••••••••" + agentAPIKey[len(agentAPIKey)-4:]
		} else {
			maskedAPIKey = agentAPIKey
		}
	}

	return ctx.Inertia("AdministrationAgents", inertia.Props{
		"websites":             websitesData,
		"agent_api_key":        maskedAPIKey,
		"agent_api_key_exists": agentAPIKey != "",
	})
}

// AdministrationSystemPageAction renders the System administration page
func AdministrationSystemPageAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	// Fetch websites for the selector
	websitesData, err := websites.GetWebsitesForSelector(db)
	if err != nil {
		websitesData = []map[string]interface{}{}
	}

	// Check if logs should be loaded
	showLogs := ctx.Query("show_logs") == "true"
	var logs string

	if showLogs {
		// Determine log file path from environment variable
		logPath := os.Getenv("FUSIONALY_LOG_FILE")
		if logPath == "" {
			// Default log path if not configured
			logPath = filepath.Join("logs", "fusionaly.log")
		}

		// Check if log file exists
		if _, err := os.Stat(logPath); !os.IsNotExist(err) {
			// Read the log file
			content, err := os.ReadFile(logPath)
			if err == nil {
				// Limit log size to last 100KB to prevent overwhelming the browser
				maxSize := 100 * 1024 // 100KB
				logs = string(content)
				if len(content) > maxSize {
					logs = "... (log truncated, showing last 100KB) ...\n\n" + logs[len(logs)-maxSize:]
				}
			}
		} else {
			logs = "No log file found at: " + logPath
		}
	}

	// Get GeoLite credentials (masked for display)
	geoAccountID, geoLicenseKey, _ := settings.GetGeoLiteCredentials(db)

	// Get GeoLite last update time
	lastUpdateStr, _ := settings.GetSetting(db, jobs.KeyGeoLiteLastUpdate)
	var geoLastUpdate string
	if lastUpdateStr != "" {
		if t, err := time.Parse(time.RFC3339, lastUpdateStr); err == nil {
			geoLastUpdate = t.Format("January 2, 2006 at 3:04 PM")
		}
	}

	// Get GeoLite download error (if any)
	geoDownloadError, _ := settings.GetSetting(db, jobs.KeyGeoLiteDownloadError)

	// Check if GeoLite database file exists
	cfg := config.GetConfig()
	geoDBPath := cfg.GeoDBPath
	if geoDBPath == "" {
		geoDBPath = filepath.Join("storage", "GeoLite2-City.mmdb")
	}
	_, geoDBErr := os.Stat(geoDBPath)
	geoDBExists := geoDBErr == nil

	return ctx.Inertia("AdministrationSystem", inertia.Props{
		"websites":               websitesData,
		"show_logs":              showLogs,
		"logs":                   logs,
		"geolite_account_id":     geoAccountID,
		"geolite_license_key":    geoLicenseKey,
		"geolite_last_update":    geoLastUpdate,
		"geolite_db_exists":      geoDBExists,
		"geolite_download_error": geoDownloadError,
	})
}

// Notification represents a system message shown in the feedback widget.
type Notification struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	Expires string `json:"expires"` // ISO 8601 date, e.g. "2026-04-22"
}

// systemNotifications returns active notifications for the current state.
func systemNotifications() []Notification {
	var notifications []Notification

	managerVersion := os.Getenv("MATCHA_MANAGER_VERSION")
	if managerVersion == "" || managerVersion == "dev" {
		notifications = append(notifications, Notification{
			ID:      "cli-selfupdate-fix-2026-03",
			Title:   "CLI update available",
			Body:    "If you are using the Fusionaly manager, run this command on your server to patch a self-updating issue:\n\ncurl -fsSL https://github.com/karloscodes/fusionaly-oss/releases/latest/download/fusionaly-linux-$(uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/') -o /tmp/fusionaly && chmod +x /tmp/fusionaly && sudo mv /tmp/fusionaly /usr/local/bin/fusionaly",
			Expires: "2026-04-22",
		})
	}

	return notifications
}

// SystemHealthAction returns the system health status for UI warning indicators
func SystemHealthAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	// Check GeoLite status
	geoAccountID, geoLicenseKey, _ := settings.GetGeoLiteCredentials(db)
	geoConfigured := geoAccountID != "" && geoLicenseKey != ""

	cfg := config.GetConfig()
	geoDBPath := cfg.GeoDBPath
	if geoDBPath == "" {
		geoDBPath = filepath.Join("storage", "GeoLite2-City.mmdb")
	}
	_, geoDBErr := os.Stat(geoDBPath)
	geoDBExists := geoDBErr == nil

	geoDownloadError, _ := settings.GetSetting(db, jobs.KeyGeoLiteDownloadError)

	// Determine overall health and warning message
	var warning string
	if geoConfigured && !geoDBExists && geoDownloadError != "" {
		warning = "GeoLite database download failed"
	} else if geoConfigured && !geoDBExists {
		warning = "GeoLite database not yet downloaded"
	}

	return ctx.JSON(fiber.Map{
		"healthy":            warning == "",
		"warning":            warning,
		"geolite_configured": geoConfigured,
		"geolite_db_exists":  geoDBExists,
		"geolite_error":      geoDownloadError,
		"notifications":      systemNotifications(),
	})
}

// SystemPurgeCacheFormAction handles POST form submission for cache purge (Inertia)
func SystemPurgeCacheFormAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	// Clear generic_cache table using cache package
	rowsAffected, err := cache.PurgeAllCaches(db)
	if err != nil {
		ctx.Logger.Error("Failed to clear generic_cache", slog.Any("error", err))
		return ctx.FlashError("Failed to clear caches").Redirect("/admin/administration/system", fiber.StatusFound)
	}

	ctx.Logger.Info("Caches purged successfully", slog.Int64("rows_deleted", rowsAffected))
	return ctx.FlashSuccess("All caches have been purged successfully").Redirect("/admin/administration/system", fiber.StatusFound)
}

// SystemGeoLiteFormAction handles POST form submission for GeoLite settings (Inertia)
func SystemGeoLiteFormAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	// Parse form data - Input is content-type aware (form-encoded or Inertia.js JSON)
	accountID := ctx.Input("geolite_account_id")
	licenseKey := ctx.Input("geolite_license_key")

	// Save GeoLite credentials
	if err := settings.SaveGeoLiteCredentials(db, accountID, licenseKey); err != nil {
		ctx.Logger.Error("Failed to save GeoLite settings", slog.Any("error", err))
		return ctx.FlashError("Failed to save GeoLite settings").Redirect("/admin/administration/system", fiber.StatusFound)
	}

	ctx.Logger.Info("GeoLite settings updated",
		slog.String("account_id", accountID),
		slog.Bool("has_license_key", licenseKey != ""))

	// Trigger immediate download if credentials were provided
	if accountID != "" && licenseKey != "" {
		cfg := ctx.Config.(*config.Config)
		jobs.TriggerImmediateDownload(db, ctx.Logger, cfg)
		return ctx.FlashSuccess("GeoLite settings saved. Database download started in the background.").Redirect("/admin/administration/system", fiber.StatusFound)
	}
	return ctx.FlashSuccess("GeoLite settings saved successfully").Redirect("/admin/administration/system", fiber.StatusFound)
}

// SystemGeoLiteDownloadAction triggers an immediate GeoLite database download (Inertia)
func SystemGeoLiteDownloadAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	// Check if credentials are configured
	accountID, licenseKey, _ := settings.GetGeoLiteCredentials(db)
	if accountID == "" || licenseKey == "" {
		return ctx.FlashError("GeoLite credentials not configured. Please enter your Account ID and License Key first.").Redirect("/admin/administration/system", fiber.StatusFound)
	}

	// Trigger immediate download
	cfg := ctx.Config.(*config.Config)
	jobs.TriggerImmediateDownload(db, ctx.Logger, cfg)

	ctx.Logger.Info("Manual GeoLite database download triggered")
	return ctx.FlashSuccess("Database download started in the background. Refresh this page in a moment to check status.").Redirect("/admin/administration/system", fiber.StatusFound)
}

// SystemAgentAPIKeyAction returns or creates the Agent API key (JSON response)
func SystemAgentAPIKeyAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	key, err := settings.GetOrCreateAgentAPIKey(db)
	if err != nil {
		ctx.Logger.Error("Failed to get/create Agent API key", slog.Any("error", err))
		return ctx.Ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve API key",
		})
	}

	return ctx.Ctx.JSON(fiber.Map{
		"api_key": key,
	})
}

// SystemAgentAPIKeyRegenerateAction regenerates the Agent API key (PRG pattern)
func SystemAgentAPIKeyRegenerateAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	_, err := settings.RegenerateAgentAPIKey(db)
	if err != nil {
		ctx.Logger.Error("Failed to regenerate Agent API key", slog.Any("error", err))
		return ctx.FlashError("Failed to regenerate API key").Redirect("/admin/administration/agents", fiber.StatusFound)
	}

	ctx.Logger.Info("Agent API key regenerated")
	return ctx.FlashSuccess("API key regenerated successfully. Copy your new key - it won't be shown again in full.").Redirect("/admin/administration/agents", fiber.StatusFound)
}
