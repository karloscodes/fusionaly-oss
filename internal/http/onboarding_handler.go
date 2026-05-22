package http

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/inertia"
	"gorm.io/gorm"

	"fusionaly/internal/config"
	"fusionaly/internal/jobs"
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

// GetOrCreateOnboardingSession gets existing session or creates a new one
func GetOrCreateOnboardingSession(ctx *cartridge.Context, db *gorm.DB) (*onboarding.OnboardingSession, error) {
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
		return ctx.FlashInfo("Setup is already complete. Please log in.").Redirect("/login", fiber.StatusFound)
	}

	// Get or create onboarding session
	session, err := GetOrCreateOnboardingSession(ctx, db)
	if err != nil {
		ctx.Logger.Error("Failed to get/create onboarding session", slog.Any("error", err))
		return ctx.FlashError("Failed to start onboarding session").Redirect("/login", fiber.StatusFound)
	}

	// Build props based on current step
	props := inertia.Props{
		"title": "Initial Setup - Fusionaly",
		"step":  string(session.Step),
		"email": session.Data.Email,
	}

	return ctx.Inertia("Onboarding", props)
}

// OnboardingUserFormAction handles user account form submission (PRG pattern)
func OnboardingUserFormAction(ctx *cartridge.Context) error {
	email := strings.TrimSpace(ctx.Input("email"))

	// Get session ID from cookie
	sessionID := ctx.Cookies(onboardingSessionCookieName)
	if sessionID == "" {
		return ctx.FlashError("No active onboarding session").Redirect("/setup", fiber.StatusFound)
	}

	db := ctx.DB()

	// Get onboarding session
	session, err := onboarding.GetOnboardingSession(db, sessionID)
	if err != nil {
		ctx.Logger.Error("Failed to get onboarding session", slog.Any("error", err))
		return ctx.FlashError("Invalid or expired onboarding session").Redirect("/setup", fiber.StatusFound)
	}

	// Validate current step
	if session.Step != onboarding.StepUserAccount {
		return ctx.FlashError("Invalid step").Redirect("/setup", fiber.StatusFound)
	}

	// Validate email
	if email == "" {
		return ctx.FlashError("Email is required").Redirect("/setup", fiber.StatusFound)
	}

	// Basic email validation
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return ctx.FlashError("Please enter a valid email address").Redirect("/setup", fiber.StatusFound)
	}

	// Update session with email and move to password step
	session.Data.Email = email
	err = onboarding.UpdateOnboardingSession(db, sessionID, onboarding.StepPassword, session.Data)
	if err != nil {
		ctx.Logger.Error("Failed to update onboarding session", slog.Any("error", err))
		return ctx.FlashError("Failed to save progress").Redirect("/setup", fiber.StatusFound)
	}

	return ctx.Redirect("/setup", fiber.StatusFound)
}

// OnboardingPasswordFormAction handles password form submission (PRG pattern)
func OnboardingPasswordFormAction(ctx *cartridge.Context) error {
	password := ctx.Input("password")
	confirmPassword := ctx.Input("confirm_password")

	// Get session ID from cookie
	sessionID := ctx.Cookies(onboardingSessionCookieName)
	if sessionID == "" {
		return ctx.FlashError("No active onboarding session").Redirect("/setup", fiber.StatusFound)
	}

	db := ctx.DB()

	// Get onboarding session
	session, err := onboarding.GetOnboardingSession(db, sessionID)
	if err != nil {
		ctx.Logger.Error("Failed to get onboarding session", slog.Any("error", err))
		return ctx.FlashError("Invalid or expired onboarding session").Redirect("/setup", fiber.StatusFound)
	}

	// Validate current step
	if session.Step != onboarding.StepPassword {
		return ctx.FlashError("Invalid step").Redirect("/setup", fiber.StatusFound)
	}

	// Validate password
	if strings.TrimSpace(password) == "" {
		return ctx.FlashError("Password is required").Redirect("/setup", fiber.StatusFound)
	}

	if password != confirmPassword {
		return ctx.FlashError("Passwords do not match").Redirect("/setup", fiber.StatusFound)
	}

	if len(password) < 8 {
		return ctx.FlashError("Password must be at least 8 characters long").Redirect("/setup", fiber.StatusFound)
	}

	// Update session with password and move to GeoLite step
	session.Data.Password = password
	err = onboarding.UpdateOnboardingSession(db, sessionID, onboarding.StepGeoLite, session.Data)
	if err != nil {
		ctx.Logger.Error("Failed to update onboarding session", slog.Any("error", err))
		return ctx.FlashError("Failed to save progress").Redirect("/setup", fiber.StatusFound)
	}

	return ctx.Redirect("/setup", fiber.StatusFound)
}

// completeOnboarding finishes the onboarding process by creating the user
func completeOnboarding(db *gorm.DB, logger *slog.Logger, c *fiber.Ctx, sessionMgr *cartridge.SessionManager, session *onboarding.OnboardingSession) error {
	// Prepare completion data
	completionData := onboarding.CompletionData{
		Email:     session.Data.Email,
		Password:  session.Data.Password,
		OpenAIKey: session.Data.OpenAIKey,
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
		return ctx.FlashError("No active onboarding session").Redirect("/setup", fiber.StatusFound)
	}

	db := ctx.DB()

	// Get onboarding session
	session, err := onboarding.GetOnboardingSession(db, sessionID)
	if err != nil {
		ctx.Logger.Error("Failed to get onboarding session", slog.Any("error", err))
		return ctx.FlashError("Invalid or expired onboarding session").Redirect("/setup", fiber.StatusFound)
	}

	// Validate current step
	if session.Step != onboarding.StepGeoLite {
		return ctx.FlashError("Invalid step").Redirect("/setup", fiber.StatusFound)
	}

	// Get GeoLite credentials from form (optional)
	var in struct {
		Action     string `json:"action" form:"action"`
		AccountID  string `json:"geolite_account_id" form:"geolite_account_id"`
		LicenseKey string `json:"geolite_license_key" form:"geolite_license_key"`
	}
	_ = ctx.Bind(&in)

	action := in.Action
	if action != "skip" {
		accountID := strings.TrimSpace(in.AccountID)
		licenseKey := strings.TrimSpace(in.LicenseKey)

		// Only save if both fields are provided
		if accountID != "" && licenseKey != "" {
			if err := settings.SaveGeoLiteCredentials(db, accountID, licenseKey); err != nil {
				ctx.Logger.Error("Failed to save GeoLite credentials", slog.Any("error", err))
				// Don't fail onboarding for this - user can configure later
			} else {
				ctx.Logger.Info("GeoLite credentials saved during onboarding")
				// Trigger immediate download in background
				cfg := ctx.Config.(*config.Config)
				jobs.TriggerImmediateDownload(db, ctx.Logger, cfg)
			}
		}
	}

	// Move to the optional OpenAI step (instead of completing here)
	err = onboarding.UpdateOnboardingSession(db, sessionID, onboarding.StepOpenAI, session.Data)
	if err != nil {
		ctx.Logger.Error("Failed to update onboarding session", slog.Any("error", err))
		return ctx.FlashError("Failed to save progress").Redirect("/setup", fiber.StatusFound)
	}

	return ctx.Redirect("/setup", fiber.StatusFound)
}

// OnboardingOpenAIFormAction handles the optional OpenAI key form submission (PRG pattern)
func OnboardingOpenAIFormAction(ctx *cartridge.Context) error {
	// Get session ID from cookie
	sessionID := ctx.Cookies(onboardingSessionCookieName)
	if sessionID == "" {
		return ctx.FlashError("No active onboarding session").Redirect("/setup", fiber.StatusFound)
	}

	db := ctx.DB()

	// Get onboarding session
	session, err := onboarding.GetOnboardingSession(db, sessionID)
	if err != nil {
		ctx.Logger.Error("Failed to get onboarding session", slog.Any("error", err))
		return ctx.FlashError("Invalid or expired onboarding session").Redirect("/setup", fiber.StatusFound)
	}

	// Validate current step
	if session.Step != onboarding.StepOpenAI {
		return ctx.FlashError("Invalid step").Redirect("/setup", fiber.StatusFound)
	}

	// OpenAI key is optional - save it only if the user provided one and did not skip
	var in struct {
		Action    string `json:"action" form:"action"`
		OpenAIKey string `json:"openai_key" form:"openai_key"`
	}
	_ = ctx.Bind(&in)

	action := in.Action
	openAIKey := strings.TrimSpace(in.OpenAIKey)
	if action != "skip" && openAIKey != "" {
		session.Data.OpenAIKey = openAIKey
	}

	// Complete the onboarding by creating the user (saves the key if present)
	err = completeOnboarding(db, ctx.Logger, ctx.Ctx, ctx.Session, session)
	if err != nil {
		ctx.Logger.Error("Failed to complete onboarding", slog.Any("error", err))
		return ctx.FlashError("Failed to complete setup: "+err.Error()).Redirect("/setup", fiber.StatusFound)
	}

	return ctx.FlashSuccess("Setup complete! You have been logged in.").Redirect("/admin/websites/new", fiber.StatusFound)
}
