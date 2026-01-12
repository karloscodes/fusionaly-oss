package onboarding

import (
	"fmt"

	"log/slog"
	"gorm.io/gorm"

	"fusionaly/internal/users"
)

// CompletionData holds all the data needed to complete onboarding
type CompletionData struct {
	Email    string
	Password string
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
	if data.Password == "" {
		return nil, fmt.Errorf("password is required")
	}

	// Create the admin user
	err := users.CreateAdminUser(db, data.Email, data.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to create admin user: %w", err)
	}

	// Find the created user
	user, err := users.FindByEmail(db, data.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to find created user: %w", err)
	}

	return &CompletionResult{
		UserID:    user.ID,
		UserEmail: data.Email,
	}, nil
}

// Note: License validation, OpenAI configuration, and Gumroad integration are available
// in Fusionaly Pro. See https://fusionaly.com/#pricing
