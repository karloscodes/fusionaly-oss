package onboarding

import (
	"errors"
	"fmt"

	"log/slog"

	"github.com/karloscodes/cartridge/sqlite"
	"gorm.io/gorm"

	"fusionaly/internal/users"
)

// ErrSetupAlreadyComplete is returned when onboarding runs after an admin exists.
var ErrSetupAlreadyComplete = errors.New("setup is already complete")

// CompletionData holds all the data needed to complete onboarding
type CompletionData struct {
	Email        string
	PasswordHash string
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
	// route guard. The check and the insert are in one write transaction, so
	// two requests at the same time cannot both create an admin.
	err := sqlite.PerformWrite(logger, db, func(tx *gorm.DB) error {
		required, err := IsOnboardingRequired(tx)
		if err != nil {
			return err
		}
		if !required {
			return ErrSetupAlreadyComplete
		}
		return tx.Create(&users.User{Email: data.Email, EncryptedPassword: data.PasswordHash}).Error
	})
	if errors.Is(err, ErrSetupAlreadyComplete) {
		return nil, err
	}
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
