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
	"github.com/karloscodes/cartridge/cache"
	"github.com/karloscodes/cartridge/flash"
	"fusionaly/internal/settings"
	"fusionaly/internal/websites"
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

	return inertia.RenderPage(ctx.Ctx, "AdministrationIngestion", inertia.Props{
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

	return inertia.RenderPage(ctx.Ctx, "AdministrationAccount", inertia.Props{
		"settings": settingsData,
		"websites": websitesData,
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

	return inertia.RenderPage(ctx.Ctx, "AdministrationSystem", inertia.Props{
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
	})
}

// SystemPurgeCacheFormAction handles POST form submission for cache purge (Inertia)
func SystemPurgeCacheFormAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	// Clear generic_cache table using cache package
	rowsAffected, err := cache.PurgeAllCaches(db)
	if err != nil {
		ctx.Logger.Error("Failed to clear generic_cache", slog.Any("error", err))
		flash.SetFlash(ctx.Ctx, "error", "Failed to clear caches")
		return ctx.Redirect("/admin/administration/system", fiber.StatusFound)
	}

	ctx.Logger.Info("Caches purged successfully", slog.Int64("rows_deleted", rowsAffected))
	flash.SetFlash(ctx.Ctx, "success", "All caches have been purged successfully")
	return ctx.Redirect("/admin/administration/system", fiber.StatusFound)
}

// SystemGeoLiteFormAction handles POST form submission for GeoLite settings (Inertia)
func SystemGeoLiteFormAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	// Parse form data - try both form value and JSON body (for Inertia.js)
	accountID := ctx.FormValue("geolite_account_id")
	licenseKey := ctx.FormValue("geolite_license_key")

	// Try parsing as JSON for Inertia.js requests
	if accountID == "" && licenseKey == "" {
		var jsonBody struct {
			AccountID  string `json:"geolite_account_id"`
			LicenseKey string `json:"geolite_license_key"`
		}
		if err := ctx.BodyParser(&jsonBody); err == nil {
			accountID = jsonBody.AccountID
			licenseKey = jsonBody.LicenseKey
		}
	}

	// Save GeoLite credentials
	if err := settings.SaveGeoLiteCredentials(db, accountID, licenseKey); err != nil {
		ctx.Logger.Error("Failed to save GeoLite settings", slog.Any("error", err))
		flash.SetFlash(ctx.Ctx, "error", "Failed to save GeoLite settings")
		return ctx.Redirect("/admin/administration/system", fiber.StatusFound)
	}

	ctx.Logger.Info("GeoLite settings updated",
		slog.String("account_id", accountID),
		slog.Bool("has_license_key", licenseKey != ""))

	// Trigger immediate download if credentials were provided
	if accountID != "" && licenseKey != "" {
		cfg := ctx.Config.(*config.Config)
		jobs.TriggerImmediateDownload(db, ctx.Logger, cfg)
		flash.SetFlash(ctx.Ctx, "success", "GeoLite settings saved. Database download started in the background.")
	} else {
		flash.SetFlash(ctx.Ctx, "success", "GeoLite settings saved successfully")
	}
	return ctx.Redirect("/admin/administration/system", fiber.StatusFound)
}

// SystemGeoLiteDownloadAction triggers an immediate GeoLite database download (Inertia)
func SystemGeoLiteDownloadAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	// Check if credentials are configured
	accountID, licenseKey, _ := settings.GetGeoLiteCredentials(db)
	if accountID == "" || licenseKey == "" {
		flash.SetFlash(ctx.Ctx, "error", "GeoLite credentials not configured. Please enter your Account ID and License Key first.")
		return ctx.Redirect("/admin/administration/system", fiber.StatusFound)
	}

	// Trigger immediate download
	cfg := ctx.Config.(*config.Config)
	jobs.TriggerImmediateDownload(db, ctx.Logger, cfg)

	ctx.Logger.Info("Manual GeoLite database download triggered")
	flash.SetFlash(ctx.Ctx, "success", "Database download started in the background. Refresh this page in a moment to check status.")
	return ctx.Redirect("/admin/administration/system", fiber.StatusFound)
}
