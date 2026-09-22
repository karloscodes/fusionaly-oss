package onboarding

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"log/slog"

	"fusionaly/internal/settings"
	"fusionaly/internal/users"
)

// ErrSetupAlreadyComplete is returned when onboarding runs after an admin exists.
var ErrSetupAlreadyComplete = errors.New("setup is already complete")

// CompletionData holds all the data needed to complete onboarding
type CompletionData struct {
	Email        string
	PasswordHash string
	OpenAIKey    string
}

// CompletionResult contains the results of completing onboarding
type CompletionResult struct {
	UserID    uint
	UserEmail string
}

// CompleteOnboarding finishes the onboarding process by creating the admin user
func CompleteOnboarding(db *gorm.DB, logger *slog.Logger, data CompletionData) (*CompletionResult, error) {
	// Validate email
	if data.Email == "" {
		return nil, fmt.Errorf("email is required")
	}

	// Validate password
	if data.PasswordHash == "" {
		return nil, fmt.Errorf("password is required")
	}

	// Setup runs once. Refuse a second admin even if a request gets past the
	// route guard.
	required, err := IsOnboardingRequired(db)
	if err != nil {
		return nil, err
	}
	if !required {
		return nil, ErrSetupAlreadyComplete
	}

	// Create the admin user
	err = users.CreateAdminUserWithHash(db, data.Email, data.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("failed to create admin user: %w", err)
	}

	// Find the created user
	user, err := users.FindByEmail(db, data.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to find created user: %w", err)
	}

	// Save the OpenAI API key if provided (optional onboarding step)
	if strings.TrimSpace(data.OpenAIKey) != "" {
		if err := settings.SaveOpenAIKey(db, data.OpenAIKey); err != nil {
			logger.Error("Failed to save OpenAI API key during onboarding", "error", err)
			// Don't fail onboarding for this - user can configure it later
		} else {
			logger.Info("OpenAI API key saved during onboarding")
		}
	}

	return &CompletionResult{
		UserID:    user.ID,
		UserEmail: data.Email,
	}, nil
}
