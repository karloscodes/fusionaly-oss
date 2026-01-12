package http

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/inertia"
	"github.com/karloscodes/cartridge/flash"
	"gorm.io/gorm"

	"fusionaly/internal/onboarding"
)

const (
	onboardingSessionCookieName = "fusionaly_onboarding_session"
)

// OnboardingStartRequest represents the start onboarding request
type OnboardingStartRequest struct {
	Force bool `json:"force"`
}

// UserAccountRequest represents the user account creation step
type UserAccountRequest struct {
	Email string `json:"email"`
}

// PasswordSetupRequest represents the password setup step
type PasswordSetupRequest struct {
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

// generateSessionID generates a secure random session ID
func generateSessionID() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// OnboardingCheckAction checks if onboarding is required
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

// OnboardingStartAction starts the onboarding process
func OnboardingStartAction(ctx *cartridge.Context) error {
	var req OnboardingStartRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request format",
		})
	}

	// Clear any existing authentication cookies to ensure clean onboarding
	if ctx.Session != nil {
		ctx.Session.ClearSession(ctx.Ctx)
	}

	db := ctx.DB()

	// Check if onboarding is actually required
	if !req.Force {
		required, err := onboarding.IsOnboardingRequired(db)
		if err != nil {
			ctx.Logger.Error("Failed to check if onboarding is required", slog.Any("error", err))
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to check onboarding status",
			})
		}
		if !required {
			return ctx.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "Onboarding is not required - users already exist",
			})
		}
	}

	// Generate session ID
	sessionID, err := generateSessionID()
	if err != nil {
		ctx.Logger.Error("Failed to generate session ID", slog.Any("error", err))
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to start onboarding session",
		})
	}

	// Create onboarding session
	session, err := onboarding.CreateOnboardingSession(db, sessionID)
	if err != nil {
		ctx.Logger.Error("Failed to create onboarding session", slog.Any("error", err))
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to start onboarding session",
		})
	}

	// Store session ID in cookie
	ctx.Cookie(&fiber.Cookie{
		Name:     onboardingSessionCookieName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   3600, // 1 hour
		HTTPOnly: true,
		Secure:   ctx.Config.IsProduction(),
		SameSite: "Lax",
	})

	return ctx.JSON(fiber.Map{
		"success":    true,
		"session_id": sessionID,
		"step":       session.Step,
		"expires_at": session.ExpiresAt,
	})
}

// OnboardingUserAccountAction handles user account creation step
func OnboardingUserAccountAction(ctx *cartridge.Context) error {
	var req UserAccountRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request format",
		})
	}

	// Get session ID from cookie
	sessionID := ctx.Cookies(onboardingSessionCookieName)
	if sessionID == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No active onboarding session",
		})
	}

	db := ctx.DB()

	// Get onboarding session
	onboardingSession, err := onboarding.GetOnboardingSession(db, sessionID)
	if err != nil {
		ctx.Logger.Error("Failed to get onboarding session", slog.Any("error", err))
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid or expired onboarding session",
		})
	}

	// Validate current step
	if onboardingSession.Step != onboarding.StepUserAccount {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Expected step %s but current step is %s", onboarding.StepUserAccount, onboardingSession.Step),
		})
	}

	// Validate email
	emailTrimmed := strings.TrimSpace(req.Email)
	if emailTrimmed == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Email is required",
		})
	}

	// Basic email validation
	if !strings.Contains(emailTrimmed, "@") || !strings.Contains(emailTrimmed, ".") {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Please enter a valid email address",
		})
	}

	onboardingSession.Data.Email = emailTrimmed

	// Move to password step
	err = onboarding.UpdateOnboardingSession(db, sessionID, onboarding.StepPassword, onboardingSession.Data)
	if err != nil {
		ctx.Logger.Error("Failed to update onboarding session", slog.Any("error", err))
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save progress",
		})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"step":    onboarding.StepPassword,
	})
}

// OnboardingPasswordAction handles password setup step and completes onboarding
func OnboardingPasswordAction(ctx *cartridge.Context) error {
	var req PasswordSetupRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request format",
		})
	}

	// Get session ID from cookie
	sessionID := ctx.Cookies(onboardingSessionCookieName)
	if sessionID == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No active onboarding session",
		})
	}

	db := ctx.DB()

	// Get onboarding session
	onboardingSession, err := onboarding.GetOnboardingSession(db, sessionID)
	if err != nil {
		ctx.Logger.Error("Failed to get onboarding session", slog.Any("error", err))
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid or expired onboarding session",
		})
	}

	// Validate current step
	if onboardingSession.Step != onboarding.StepPassword {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Expected step %s but current step is %s", onboarding.StepPassword, onboardingSession.Step),
		})
	}

	// Validate password
	if strings.TrimSpace(req.Password) == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Password is required",
		})
	}

	if req.Password != req.ConfirmPassword {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Passwords do not match",
		})
	}

	if len(req.Password) < 8 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Password must be at least 8 characters long",
		})
	}

	// Store password
	onboardingSession.Data.Password = req.Password

	// Complete the onboarding by creating the user
	err = completeOnboarding(db, ctx.Logger, ctx.Ctx, ctx.Session, onboardingSession)
	if err != nil {
		ctx.Logger.Error("Failed to complete onboarding", slog.Any("error", err))
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to complete onboarding: %v", err),
		})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"step":    onboarding.StepCompleted,
		"message": "Onboarding completed successfully! You have been logged in.",
	})
}

// completeOnboarding finishes the onboarding process by creating the user
func completeOnboarding(db *gorm.DB, logger *slog.Logger, c *fiber.Ctx, session *cartridge.SessionManager, onboardingSession *onboarding.OnboardingSession) error {
	// Prepare completion data
	completionData := onboarding.CompletionData{
		Email:    onboardingSession.Data.Email,
		Password: onboardingSession.Data.Password,
	}

	// Use onboarding context function to complete
	result, err := onboarding.CompleteOnboarding(db, logger, completionData)
	if err != nil {
		return err
	}

	// Log in the user by setting their session
	if session != nil {
		if err := session.SetSession(c, result.UserID); err != nil {
			logger.Error("Failed to set user session", slog.Any("error", err))
			// Don't fail the process for this, user can log in manually
		}
	}

	// Mark onboarding as completed
	err = onboarding.CompleteOnboardingSession(db, onboardingSession.ID)
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

// OnboardingStatusAction returns the current onboarding status
func OnboardingStatusAction(ctx *cartridge.Context) error {
	// Get session ID from cookie
	sessionID := ctx.Cookies(onboardingSessionCookieName)
	if sessionID == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No active onboarding session",
		})
	}

	db := ctx.DB()

	// Get onboarding session
	onboardingSession, err := onboarding.GetOnboardingSession(db, sessionID)
	if err != nil {
		ctx.Logger.Error("Failed to get onboarding session", slog.Any("error", err))
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid or expired onboarding session",
		})
	}

	return ctx.JSON(fiber.Map{
		"success":    true,
		"session_id": onboardingSession.ID,
		"step":       onboardingSession.Step,
		"completed":  onboardingSession.Completed,
		"expires_at": onboardingSession.ExpiresAt,
	})
}

// OnboardingPageAction serves the onboarding wizard page
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

	// Render the onboarding page using Inertia
	return inertia.RenderPage(ctx.Ctx, "Onboarding", inertia.Props{
		"title": "Initial Setup - Fusionaly",
	})
}

// Note: License validation, OpenAI configuration, and Gumroad email handlers are available
// in Fusionaly Pro. See https://fusionaly.com/#pricing
