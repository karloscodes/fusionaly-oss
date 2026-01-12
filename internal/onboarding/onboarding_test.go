package onboarding_test

import (
	"testing"
	"time"

	"fusionaly/internal/onboarding"
	"fusionaly/internal/users"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB creates an in-memory SQLite database and migrates necessary schemas
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Migrate necessary schemas
	err = db.AutoMigrate(&onboarding.OnboardingSession{}, &users.User{})
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	return db
}

func TestOnboardingData_Scan_Value(t *testing.T) {
	data := onboarding.OnboardingData{
		Email:    "test@example.com",
		Password: "password123",
	}

	// Test Value()
	value, err := data.Value()
	assert.NoError(t, err)
	assert.NotNil(t, value)

	// Test Scan() with []byte
	jsonBytes, ok := value.([]byte)
	if !ok {
		// Driver might return string
		jsonString, ok := value.(string)
		assert.True(t, ok, "Value should be []byte or string")
		jsonBytes = []byte(jsonString)
	}

	var scannedData onboarding.OnboardingData
	err = scannedData.Scan(jsonBytes)
	assert.NoError(t, err)
	assert.Equal(t, data, scannedData)

	// Test Scan() with string
	var scannedDataStr onboarding.OnboardingData
	err = scannedDataStr.Scan(string(jsonBytes))
	assert.NoError(t, err)
	assert.Equal(t, data, scannedDataStr)

	// Test Scan() with nil
	var emptyData onboarding.OnboardingData
	err = emptyData.Scan(nil)
	assert.NoError(t, err)
	assert.Equal(t, onboarding.OnboardingData{}, emptyData)
}

func TestCreateOnboardingSession(t *testing.T) {
	db := setupTestDB(t)
	sessionID := "test-session-id"

	session, err := onboarding.CreateOnboardingSession(db, sessionID)
	assert.NoError(t, err)
	assert.NotNil(t, session)
	assert.Equal(t, sessionID, session.ID)
	assert.Equal(t, onboarding.StepUserAccount, session.Step) // OSS starts at user account
	assert.False(t, session.Completed)
	assert.True(t, session.ExpiresAt.After(time.Now()))
}

func TestGetOnboardingSession(t *testing.T) {
	db := setupTestDB(t)
	sessionID := "test-session-id"

	// Create a session
	_, err := onboarding.CreateOnboardingSession(db, sessionID)
	assert.NoError(t, err)

	// Retrieve it
	session, err := onboarding.GetOnboardingSession(db, sessionID)
	assert.NoError(t, err)
	assert.NotNil(t, session)
	assert.Equal(t, sessionID, session.ID)

	// Test not found
	_, err = onboarding.GetOnboardingSession(db, "non-existent")
	assert.Error(t, err)
}

func TestUpdateOnboardingSession(t *testing.T) {
	db := setupTestDB(t)
	sessionID := "test-session-id"

	_, err := onboarding.CreateOnboardingSession(db, sessionID)
	assert.NoError(t, err)

	newData := onboarding.OnboardingData{
		Email: "updated@example.com",
	}
	newStep := onboarding.StepPassword

	err = onboarding.UpdateOnboardingSession(db, sessionID, newStep, newData)
	assert.NoError(t, err)

	// Verify update
	session, err := onboarding.GetOnboardingSession(db, sessionID)
	assert.NoError(t, err)
	assert.Equal(t, newStep, session.Step)
	assert.Equal(t, newData.Email, session.Data.Email)
}

func TestCompleteOnboardingSession(t *testing.T) {
	db := setupTestDB(t)
	sessionID := "test-session-id"

	_, err := onboarding.CreateOnboardingSession(db, sessionID)
	assert.NoError(t, err)

	err = onboarding.CompleteOnboardingSession(db, sessionID)
	assert.NoError(t, err)

	// Verify completed
	var session onboarding.OnboardingSession
	err = db.First(&session, "id = ?", sessionID).Error
	assert.NoError(t, err)
	assert.True(t, session.Completed)

	// Should not be retrievable via GetOnboardingSession (which filters out completed)
	_, err = onboarding.GetOnboardingSession(db, sessionID)
	assert.Error(t, err)
}

func TestIsExpired(t *testing.T) {
	session := onboarding.OnboardingSession{
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	assert.False(t, session.IsExpired())

	sessionExpired := onboarding.OnboardingSession{
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	assert.True(t, sessionExpired.IsExpired())
}

func TestCleanupExpiredOnboardingSessions(t *testing.T) {
	db := setupTestDB(t)

	// Create active session
	activeSession := onboarding.OnboardingSession{
		ID:        "active",
		Step:      onboarding.StepUserAccount,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	db.Create(&activeSession)

	// Create expired session
	expiredSession := onboarding.OnboardingSession{
		ID:        "expired",
		Step:      onboarding.StepUserAccount,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	db.Create(&expiredSession)

	err := onboarding.CleanupExpiredOnboardingSessions(db)
	assert.NoError(t, err)

	// Verify active exists
	var count int64
	db.Model(&onboarding.OnboardingSession{}).Where("id = ?", "active").Count(&count)
	assert.Equal(t, int64(1), count)

	// Verify expired gone
	db.Model(&onboarding.OnboardingSession{}).Where("id = ?", "expired").Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestIsOnboardingRequired(t *testing.T) {
	db := setupTestDB(t)

	// Initially required (no users)
	required, err := onboarding.IsOnboardingRequired(db)
	assert.NoError(t, err)
	assert.True(t, required)

	// Create a user
	user := users.User{
		Email: "admin@example.com",
	}
	db.Create(&user)

	// Now not required
	required, err = onboarding.IsOnboardingRequired(db)
	assert.NoError(t, err)
	assert.False(t, required)
}
