package http

import (
	"net"
	"strings"

	"github.com/gofiber/fiber/v2"
	"log/slog"

	"fusionaly/internal/settings"
	"github.com/karloscodes/cartridge"
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
	// Bind is content-type aware (form-encoded or Inertia's JSON form.post())
	var in struct {
		ExcludedIPs string `json:"excluded_ips" form:"excluded_ips"`
	}
	_ = ctx.Bind(&in)
	excludedIPs := in.ExcludedIPs

	// Validate IP list
	if valid, msg := validateIPList(excludedIPs); !valid {
		ctx.Logger.Warn("invalid IP format submitted", slog.String("error", msg))
		return ctx.FlashError(msg).Redirect("/admin/administration/ingestion", fiber.StatusFound)
	}

	db := ctx.DB()

	// Update setting
	if err := settings.UpdateSetting(db, "excluded_ips", excludedIPs); err != nil {
		ctx.Logger.Error("failed to update excluded_ips setting", slog.Any("error", err))
		return ctx.FlashError("Failed to update IP filtering settings").Redirect("/admin/administration/ingestion", fiber.StatusFound)
	}

	ctx.Logger.Info("excluded IPs updated via form")
	return ctx.FlashSuccess("Ingestion settings saved successfully!").Redirect("/admin/administration/ingestion", fiber.StatusFound)
}

// Note: AISettingsFormAction is available in Fusionaly Pro
