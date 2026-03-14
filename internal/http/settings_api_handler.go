package http

import (
	"net"
	"strings"

	"github.com/gofiber/fiber/v2"
	"log/slog"

	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/flash"
	"fusionaly/internal/settings"
)

// validateIPList validates a comma-separated list of IP addresses or CIDR ranges
func validateIPList(ipList string) (bool, string) {
	if ipList == "" {
		return true, ""
	}

	ips := strings.Split(ipList, ",")
	for _, entry := range ips {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		if strings.Contains(entry, "/") {
			// CIDR range
			if _, _, err := net.ParseCIDR(entry); err != nil {
				return false, "Invalid IP range format: " + entry
			}
		} else {
			// Single IP
			if net.ParseIP(entry) == nil {
				return false, "Invalid IP address format: " + entry
			}
		}
	}

	return true, ""
}

// IngestionSettingsFormAction handles POST form submission for ingestion settings (Inertia)
func IngestionSettingsFormAction(ctx *cartridge.Context) error {
	excludedIPs := ctx.FormValue("excluded_ips")

	// Validate IP list
	if valid, msg := validateIPList(excludedIPs); !valid {
		ctx.Logger.Warn("invalid IP format submitted", slog.String("error", msg))
		flash.SetFlash(ctx.Ctx, "error", msg)
		return ctx.Redirect("/admin/administration/ingestion", fiber.StatusFound)
	}

	db := ctx.DB()

	// Update setting
	if err := settings.UpdateSetting(db, "excluded_ips", excludedIPs); err != nil {
		ctx.Logger.Error("failed to update excluded_ips setting", slog.Any("error", err))
		flash.SetFlash(ctx.Ctx, "error", "Failed to update IP filtering settings")
		return ctx.Redirect("/admin/administration/ingestion", fiber.StatusFound)
	}

	ctx.Logger.Info("excluded IPs updated via form")
	flash.SetFlash(ctx.Ctx, "success", "Ingestion settings saved successfully!")
	return ctx.Redirect("/admin/administration/ingestion", fiber.StatusFound)
}

// Note: AISettingsFormAction is available in Fusionaly Pro
