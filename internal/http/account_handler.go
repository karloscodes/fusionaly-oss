package http

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/crypto"
	"github.com/karloscodes/cartridge/flash"
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
		flash.SetFlash(ctx.Ctx, "error", "Authentication required")
		return ctx.Redirect("/admin/administration/account", fiber.StatusFound)
	}

	// Validate input
	if strings.TrimSpace(currentPassword) == "" {
		flash.SetFlash(ctx.Ctx, "error", "Current password is required")
		return ctx.Redirect("/admin/administration/account", fiber.StatusFound)
	}

	if strings.TrimSpace(newPassword) == "" {
		flash.SetFlash(ctx.Ctx, "error", "New password is required")
		return ctx.Redirect("/admin/administration/account", fiber.StatusFound)
	}

	if len(newPassword) < 8 {
		flash.SetFlash(ctx.Ctx, "error", "New password must be at least 8 characters long")
		return ctx.Redirect("/admin/administration/account", fiber.StatusFound)
	}

	db := ctx.DB()

	// Find user by ID
	user, err := users.FindByID(db, userID)
	if err != nil {
		ctx.Logger.Error("Failed to find user for password change", slog.Uint64("userID", uint64(userID)), slog.Any("error", err))
		flash.SetFlash(ctx.Ctx, "error", "User not found")
		return ctx.Redirect("/admin/administration/account", fiber.StatusFound)
	}

	// Verify current password
	if !crypto.VerifyPassword(user.EncryptedPassword, currentPassword) {
		ctx.Logger.Warn("Invalid current password provided during password change", slog.Uint64("userID", uint64(userID)))
		flash.SetFlash(ctx.Ctx, "error", "Current password is incorrect")
		return ctx.Redirect("/admin/administration/account", fiber.StatusFound)
	}

	// Change password
	if err := users.ChangePassword(db, user.Email, newPassword); err != nil {
		ctx.Logger.Error("Failed to change password", slog.Uint64("userID", uint64(userID)), slog.Any("error", err))
		flash.SetFlash(ctx.Ctx, "error", "Failed to change password")
		return ctx.Redirect("/admin/administration/account", fiber.StatusFound)
	}

	ctx.Logger.Info("Password changed successfully", slog.Uint64("userID", uint64(userID)), slog.String("email", user.Email))
	flash.SetFlash(ctx.Ctx, "success", "Password changed successfully")
	return ctx.Redirect("/admin/administration/account", fiber.StatusFound)
}

// Note: License-related handlers (AccountUpdateLicenseFormAction, AccountCheckLicenseFormAction)
// are available in Fusionaly Pro. See https://fusionaly.com/#pricing
