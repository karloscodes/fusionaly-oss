package http

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/crypto"
	"log/slog"

	"fusionaly/internal/users"
)

// AccountChangePasswordFormAction handles POST form submission for password change (Inertia)
func AccountChangePasswordFormAction(ctx *cartridge.Context) error {
	currentPassword := ctx.FormValue("current_password")
	newPassword := ctx.FormValue("new_password")

	// Get current user ID from session
	userID, authenticated := ctx.Session.GetUserID(ctx.Ctx)
	if !authenticated {
		return ctx.FlashError("Authentication required").Redirect("/admin/administration/account", fiber.StatusFound)
	}

	// Validate input
	if strings.TrimSpace(currentPassword) == "" {
		return ctx.FlashError("Current password is required").Redirect("/admin/administration/account", fiber.StatusFound)
	}

	if strings.TrimSpace(newPassword) == "" {
		return ctx.FlashError("New password is required").Redirect("/admin/administration/account", fiber.StatusFound)
	}

	if len(newPassword) < 8 {
		return ctx.FlashError("New password must be at least 8 characters long").Redirect("/admin/administration/account", fiber.StatusFound)
	}

	db := ctx.DB()

	// Find user by ID
	user, err := users.FindByID(db, userID)
	if err != nil {
		ctx.Logger.Error("Failed to find user for password change", slog.Uint64("userID", uint64(userID)), slog.Any("error", err))
		return ctx.FlashError("User not found").Redirect("/admin/administration/account", fiber.StatusFound)
	}

	// Verify current password
	if !crypto.VerifyPassword(user.EncryptedPassword, currentPassword) {
		ctx.Logger.Warn("Invalid current password provided during password change", slog.Uint64("userID", uint64(userID)))
		return ctx.FlashError("Current password is incorrect").Redirect("/admin/administration/account", fiber.StatusFound)
	}

	// Change password
	if err := users.ChangePassword(db, user.Email, newPassword); err != nil {
		ctx.Logger.Error("Failed to change password", slog.Uint64("userID", uint64(userID)), slog.Any("error", err))
		return ctx.FlashError("Failed to change password").Redirect("/admin/administration/account", fiber.StatusFound)
	}

	ctx.Logger.Info("Password changed successfully", slog.Uint64("userID", uint64(userID)), slog.String("email", user.Email))
	return ctx.FlashSuccess("Password changed successfully").Redirect("/admin/administration/account", fiber.StatusFound)
}

// Note: Fusionaly has no license/seat model. The former Pro license handlers
// (AccountUpdateLicenseFormAction, AccountCheckLicenseFormAction) are intentionally
// not present — all features are available in the single Fusionaly product.
