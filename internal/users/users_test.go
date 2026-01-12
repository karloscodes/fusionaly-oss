package users_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"fusionaly/internal/users"
	"fusionaly/internal/testsupport"
)

func TestFindByEmail(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()

	t.Run("finds existing user", func(t *testing.T) {
		// Create a test user
		testUser := testsupport.CreateTestUser(db, "test@example.com", "password123")

		// Find the user
		foundUser, err := users.FindByEmail(db, "test@example.com")

		require.NoError(t, err)
		assert.NotNil(t, foundUser)
		assert.Equal(t, testUser.Email, foundUser.Email)
		assert.Equal(t, testUser.ID, foundUser.ID)
	})

	t.Run("returns error for non-existent user", func(t *testing.T) {
		// Try to find a user that doesn't exist
		foundUser, err := users.FindByEmail(db, "nonexistent@example.com")

		assert.Error(t, err)
		assert.Nil(t, foundUser)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("returns error for empty email", func(t *testing.T) {
		foundUser, err := users.FindByEmail(db, "")

		assert.Error(t, err)
		assert.Nil(t, foundUser)
	})
}

func TestCreateAdminUser(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()

	t.Run("creates new admin user successfully", func(t *testing.T) {
		email := "newadmin@example.com"
		password := "securepassword123"

		err := users.CreateAdminUser(db, email, password)
		require.NoError(t, err)

		// Verify user was created
		foundUser, err := users.FindByEmail(db, email)
		require.NoError(t, err)
		assert.Equal(t, email, foundUser.Email)
		assert.NotEmpty(t, foundUser.EncryptedPassword)
	})

	t.Run("returns error when user already exists", func(t *testing.T) {
		email := "existing@example.com"
		password := "password123"

		// Create user first time
		err := users.CreateAdminUser(db, email, password)
		require.NoError(t, err)

		// Try to create same user again
		err = users.CreateAdminUser(db, email, password)
		assert.Error(t, err)
		assert.ErrorIs(t, err, users.ErrUserExists)
	})

	t.Run("returns error for empty email", func(t *testing.T) {
		err := users.CreateAdminUser(db, "", "password123")
		assert.Error(t, err)
	})

	t.Run("returns error for empty password", func(t *testing.T) {
		err := users.CreateAdminUser(db, "test@example.com", "")
		assert.Error(t, err)
	})
}

func TestChangePassword(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()

	t.Run("changes password successfully", func(t *testing.T) {
		email := "changepass@example.com"
		oldPassword := "oldpassword123"
		newPassword := "newpassword456"

		// Create user
		err := users.CreateAdminUser(db, email, oldPassword)
		require.NoError(t, err)

		// Get original encrypted password
		userBefore, err := users.FindByEmail(db, email)
		require.NoError(t, err)
		oldEncryptedPassword := userBefore.EncryptedPassword

		// Change password
		err = users.ChangePassword(db, email, newPassword)
		require.NoError(t, err)

		// Verify password was changed
		userAfter, err := users.FindByEmail(db, email)
		require.NoError(t, err)
		assert.NotEqual(t, oldEncryptedPassword, userAfter.EncryptedPassword)
		assert.NotEmpty(t, userAfter.EncryptedPassword)
	})

	t.Run("returns error for non-existent user", func(t *testing.T) {
		err := users.ChangePassword(db, "nonexistent@example.com", "newpassword")
		assert.Error(t, err)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("returns error for empty password", func(t *testing.T) {
		email := "testuser@example.com"

		// Create user first
		err := users.CreateAdminUser(db, email, "password123")
		require.NoError(t, err)

		// Try to change to empty password
		err = users.ChangePassword(db, email, "")
		assert.Error(t, err)
	})
}

func TestSetupAdminUserIfNotExists(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()

	t.Run("creates user if not exists", func(t *testing.T) {
		email := "setup@example.com"

		// Call setup function
		users.SetupAdminUserIfNotExists(db, email)

		// Verify user was created
		foundUser, err := users.FindByEmail(db, email)
		require.NoError(t, err)
		assert.Equal(t, email, foundUser.Email)
	})

	t.Run("does not error if user already exists", func(t *testing.T) {
		email := "existing-setup@example.com"

		// Create user first
		err := users.CreateAdminUser(db, email, "password123")
		require.NoError(t, err)

		// Call setup function - should not panic or error
		users.SetupAdminUserIfNotExists(db, email)

		// Verify user still exists
		foundUser, err := users.FindByEmail(db, email)
		require.NoError(t, err)
		assert.Equal(t, email, foundUser.Email)
	})
}

func TestErrUserExists(t *testing.T) {
	t.Run("ErrUserExists is defined", func(t *testing.T) {
		assert.NotNil(t, users.ErrUserExists)
		assert.Equal(t, "user already exists", users.ErrUserExists.Error())
	})
}

func TestErrUserNotFound(t *testing.T) {
	t.Run("ErrUserNotFound is gorm.ErrRecordNotFound", func(t *testing.T) {
		assert.Equal(t, gorm.ErrRecordNotFound, users.ErrUserNotFound)
	})
}
