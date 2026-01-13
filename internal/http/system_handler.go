package http

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/inertia"

	"fusionaly/internal/config"
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

	return inertia.RenderPage(ctx.Ctx, "AdministrationSystem", inertia.Props{
		"websites":  websitesData,
		"show_logs": showLogs,
		"logs":      logs,
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
