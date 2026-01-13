package http

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/flash"
	"github.com/karloscodes/cartridge/inertia"
	"gorm.io/gorm"

	"fusionaly/internal/onboarding"
	"fusionaly/internal/settings"
)

const (
	onboardingSessionCookieName = "fusionaly_onboarding_session"
)

// generateSessionID generates a secure random session ID
func generateSessionID() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// OnboardingCheckAction checks if onboarding is required (JSON API for initial check)
func OnboardingCheckAction(ctx *cartridge.Context) error {
	db := ctx.DB()

	required, err := onboarding.IsOnboardingRequired(db)
	if err != nil {
		ctx.Logger.Error("Failed to check if onboarding is required", slog.Any("error", err))
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to check onboarding status",
		})
	}

	return ctx.JSON(fiber.Map{
		"required": required,
	})
}

// getOrCreateOnboardingSession gets existing session or creates a new one
func getOrCreateOnboardingSession(ctx *cartridge.Context, db *gorm.DB) (*onboarding.OnboardingSession, error) {
	// Clear any existing authentication cookies to ensure clean onboarding
	if ctx.Session != nil {
		ctx.Session.ClearSession(ctx.Ctx)
	}

	// Try to get existing session from cookie
	sessionID := ctx.Cookies(onboardingSessionCookieName)
	if sessionID != "" {
		session, err := onboarding.GetOnboardingSession(db, sessionID)
		if err == nil {
			return session, nil
		}
		// Session expired or invalid, will create new one
	}

	// Generate new session ID
	newSessionID, err := generateSessionID()
	if err != nil {
		return nil, err
	}

	// Create new onboarding session
	session, err := onboarding.CreateOnboardingSession(db, newSessionID)
	if err != nil {
		return nil, err
	}

	// Store session ID in cookie
	ctx.Cookie(&fiber.Cookie{
		Name:     onboardingSessionCookieName,
		Value:    newSessionID,
		Path:     "/",
		MaxAge:   3600, // 1 hour
		HTTPOnly: true,
		Secure:   ctx.Config.IsProduction(),
		SameSite: "Lax",
	})

	return session, nil
}

// OnboardingPageAction serves the onboarding wizard page (Inertia)
func OnboardingPageAction(ctx *cartridge.Context) error {
	forceParam := ctx.Query("force")
	forceSetup := forceParam == "1" || strings.ToLower(forceParam) == "true"

	db := ctx.DB()

	// Check if onboarding is required
	required, err := onboarding.IsOnboardingRequired(db)
	if err != nil {
		ctx.Logger.Error("Failed to check if onboarding is required", slog.Any("error", err))
		return ctx.Status(fiber.StatusInternalServerError).SendString("System error")
	}

	if !required && !forceSetup {
		// Redirect to login if onboarding is not required
		flash.SetFlash(ctx.Ctx, "info", "Setup is already complete. Please log in.")
		return ctx.Redirect("/login", fiber.StatusFound)
	}

	// Get or create onboarding session
	session, err := getOrCreateOnboardingSession(ctx, db)
	if err != nil {
		ctx.Logger.Error("Failed to get/create onboarding session", slog.Any("error", err))
		flash.SetFlash(ctx.Ctx, "error", "Failed to start onboarding session")
		return ctx.Redirect("/login", fiber.StatusFound)
	}

	// Build props based on current step
	props := inertia.Props{
		"title": "Initial Setup - Fusionaly",
		"step":  string(session.Step),
		"email": session.Data.Email,
	}

	return inertia.RenderPage(ctx.Ctx, "Onboarding", props)
}

// OnboardingUserFormAction handles user account form submission (PRG pattern)
func OnboardingUserFormAction(ctx *cartridge.Context) error {
	email := strings.TrimSpace(ctx.FormValue("email"))

	// Get session ID from cookie
	sessionID := ctx.Cookies(onboardingSessionCookieName)
	if sessionID == "" {
		flash.SetFlash(ctx.Ctx, "error", "No active onboarding session")
		return ctx.Redirect("/setup", fiber.StatusFound)
	}

	db := ctx.DB()

	// Get onboarding session
	session, err := onboarding.GetOnboardingSession(db, sessionID)
	if err != nil {
		ctx.Logger.Error("Failed to get onboarding session", slog.Any("error", err))
		flash.SetFlash(ctx.Ctx, "error", "Invalid or expired onboarding session")
		return ctx.Redirect("/setup", fiber.StatusFound)
	}

	// Validate current step
	if session.Step != onboarding.StepUserAccount {
		flash.SetFlash(ctx.Ctx, "error", "Invalid step")
		return ctx.Redirect("/setup", fiber.StatusFound)
	}

	// Validate email
	if email == "" {
		flash.SetFlash(ctx.Ctx, "error", "Email is required")
		return ctx.Redirect("/setup", fiber.StatusFound)
	}

	// Basic email validation
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		flash.SetFlash(ctx.Ctx, "error", "Please enter a valid email address")
		return ctx.Redirect("/setup", fiber.StatusFound)
	}

	// Update session with email and move to password step
	session.Data.Email = email
	err = onboarding.UpdateOnboardingSession(db, sessionID, onboarding.StepPassword, session.Data)
	if err != nil {
		ctx.Logger.Error("Failed to update onboarding session", slog.Any("error", err))
		flash.SetFlash(ctx.Ctx, "error", "Failed to save progress")
		return ctx.Redirect("/setup", fiber.StatusFound)
	}

	return ctx.Redirect("/setup", fiber.StatusFound)
}

// OnboardingPasswordFormAction handles password form submission (PRG pattern)
func OnboardingPasswordFormAction(ctx *cartridge.Context) error {
	password := ctx.FormValue("password")
	confirmPassword := ctx.FormValue("confirm_password")

	// Get session ID from cookie
	sessionID := ctx.Cookies(onboardingSessionCookieName)
	if sessionID == "" {
		flash.SetFlash(ctx.Ctx, "error", "No active onboarding session")
		return ctx.Redirect("/setup", fiber.StatusFound)
	}

	db := ctx.DB()

	// Get onboarding session
	session, err := onboarding.GetOnboardingSession(db, sessionID)
	if err != nil {
		ctx.Logger.Error("Failed to get onboarding session", slog.Any("error", err))
		flash.SetFlash(ctx.Ctx, "error", "Invalid or expired onboarding session")
		return ctx.Redirect("/setup", fiber.StatusFound)
	}

	// Validate current step
	if session.Step != onboarding.StepPassword {
		flash.SetFlash(ctx.Ctx, "error", "Invalid step")
		return ctx.Redirect("/setup", fiber.StatusFound)
	}

	// Validate password
	if strings.TrimSpace(password) == "" {
		flash.SetFlash(ctx.Ctx, "error", "Password is required")
		return ctx.Redirect("/setup", fiber.StatusFound)
	}

	if password != confirmPassword {
		flash.SetFlash(ctx.Ctx, "error", "Passwords do not match")
		return ctx.Redirect("/setup", fiber.StatusFound)
	}

	if len(password) < 8 {
		flash.SetFlash(ctx.Ctx, "error", "Password must be at least 8 characters long")
		return ctx.Redirect("/setup", fiber.StatusFound)
	}

	// Update session with password and move to GeoLite step
	session.Data.Password = password
	err = onboarding.UpdateOnboardingSession(db, sessionID, onboarding.StepGeoLite, session.Data)
	if err != nil {
		ctx.Logger.Error("Failed to update onboarding session", slog.Any("error", err))
		flash.SetFlash(ctx.Ctx, "error", "Failed to save progress")
		return ctx.Redirect("/setup", fiber.StatusFound)
	}

	return ctx.Redirect("/setup", fiber.StatusFound)
}

// completeOnboarding finishes the onboarding process by creating the user
func completeOnboarding(db *gorm.DB, logger *slog.Logger, c *fiber.Ctx, sessionMgr *cartridge.SessionManager, session *onboarding.OnboardingSession) error {
	// Prepare completion data
	completionData := onboarding.CompletionData{
		Email:    session.Data.Email,
		Password: session.Data.Password,
	}

	// Use onboarding context function to complete
	result, err := onboarding.CompleteOnboarding(db, logger, completionData)
	if err != nil {
		return err
	}

	// Log in the user by setting their session
	if sessionMgr != nil {
		if err := sessionMgr.SetSession(c, result.UserID); err != nil {
			logger.Error("Failed to set user session", slog.Any("error", err))
			// Don't fail the process for this, user can log in manually
		}
	}

	// Mark onboarding as completed
	err = onboarding.CompleteOnboardingSession(db, session.ID)
	if err != nil {
		logger.Error("Failed to mark onboarding as completed", slog.Any("error", err))
		// Don't fail the process for this
	}

	// Clear onboarding session cookie
	c.ClearCookie(onboardingSessionCookieName)

	logger.Info("Onboarding completed successfully",
		slog.String("user_email", result.UserEmail))

	return nil
}

// OnboardingGeoLiteFormAction handles GeoLite configuration form submission (PRG pattern)
func OnboardingGeoLiteFormAction(ctx *cartridge.Context) error {
	// Get session ID from cookie
	sessionID := ctx.Cookies(onboardingSessionCookieName)
	if sessionID == "" {
		flash.SetFlash(ctx.Ctx, "error", "No active onboarding session")
		return ctx.Redirect("/setup", fiber.StatusFound)
	}

	db := ctx.DB()

	// Get onboarding session
	session, err := onboarding.GetOnboardingSession(db, sessionID)
	if err != nil {
		ctx.Logger.Error("Failed to get onboarding session", slog.Any("error", err))
		flash.SetFlash(ctx.Ctx, "error", "Invalid or expired onboarding session")
		return ctx.Redirect("/setup", fiber.StatusFound)
	}

	// Validate current step
	if session.Step != onboarding.StepGeoLite {
		flash.SetFlash(ctx.Ctx, "error", "Invalid step")
		return ctx.Redirect("/setup", fiber.StatusFound)
	}

	// Get GeoLite credentials from form (optional)
	action := ctx.FormValue("action")
	if action != "skip" {
		accountID := strings.TrimSpace(ctx.FormValue("geolite_account_id"))
		licenseKey := strings.TrimSpace(ctx.FormValue("geolite_license_key"))

		// Only save if both fields are provided
		if accountID != "" && licenseKey != "" {
			if err := settings.SaveGeoLiteCredentials(db, accountID, licenseKey); err != nil {
				ctx.Logger.Error("Failed to save GeoLite credentials", slog.Any("error", err))
				// Don't fail onboarding for this - user can configure later
			} else {
				ctx.Logger.Info("GeoLite credentials saved during onboarding")
			}
		}
	}

	// Complete the onboarding by creating the user
	err = completeOnboarding(db, ctx.Logger, ctx.Ctx, ctx.Session, session)
	if err != nil {
		ctx.Logger.Error("Failed to complete onboarding", slog.Any("error", err))
		flash.SetFlash(ctx.Ctx, "error", "Failed to complete setup: "+err.Error())
		return ctx.Redirect("/setup", fiber.StatusFound)
	}

	flash.SetFlash(ctx.Ctx, "success", "Setup complete! You have been logged in.")
	return ctx.Redirect("/admin/websites/new", fiber.StatusFound)
}
