package users

import (
	"database/sql"
	"errors"
	"time"

	"log/slog"

	"github.com/karloscodes/cartridge/crypto"
	"github.com/karloscodes/cartridge/sqlite"
	"gorm.io/gorm"
)

type User struct {
	ID                  uint   `gorm:"primaryKey"`
	Email               string `gorm:"uniqueIndex"`
	EncryptedPassword   string
	ResetPasswordToken  sql.NullString
	ResetPasswordSentAt sql.NullTime
	RememberCreatedAt   sql.NullTime
	CreatedAt           time.Time `gorm:"autoCreateTime"`
	UpdatedAt           time.Time `gorm:"autoUpdateTime"`
}

// ErrUserExists is returned when attempting to create a user that already exists.
var ErrUserExists = errors.New("user already exists")

// ErrUserNotFound is returned when a user lookup fails.
var ErrUserNotFound = gorm.ErrRecordNotFound

// FindByEmail retrieves a user by email.
func FindByEmail(db *gorm.DB, email string) (*User, error) {
	var user User
	if err := db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByID retrieves a user by ID.
func FindByID(db *gorm.DB, id uint) (*User, error) {
	var user User
	if err := db.Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// CreateAdminUser creates a new admin user with the supplied credentials. It returns ErrUserExists if the user already exists.
func CreateAdminUser(dbConn *gorm.DB, email, password string) error {
	// Check existence first
	if _, err := FindByEmail(dbConn, email); err == nil {
		return ErrUserExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if email == "" {
		return errors.New("email cannot be empty")
	}
	if password == "" {
		return errors.New("password cannot be empty")
	}

	hashedPassword, err := crypto.GeneratePasswordHash(password)
	if err != nil {
		return err
	}

	newUser := User{
		Email:             email,
		EncryptedPassword: string(hashedPassword),
	}

	logger := slog.Default()
	return sqlite.PerformWrite(logger, dbConn, func(tx *gorm.DB) error {
		return tx.Create(&newUser).Error
	})
}

// ChangePassword updates a user's password given their email.
func ChangePassword(dbConn *gorm.DB, email, password string) error {
	if password == "" {
		return errors.New("password cannot be empty")
	}

	user, err := FindByEmail(dbConn, email)
	if err != nil {
		return err
	}

	hashedPassword, err := crypto.GeneratePasswordHash(password)
	if err != nil {
		return err
	}

	logger := slog.Default()
	return sqlite.PerformWrite(logger, dbConn, func(tx *gorm.DB) error {
		return tx.Model(user).Update("encrypted_password", string(hashedPassword)).Error
	})
}

// SetupAdminUserIfNotExists creates a default user in the database if it doesn't already exist
func SetupAdminUserIfNotExists(dbConn *gorm.DB, email string) {
	logger := slog.Default()
	hashedPassword, err := crypto.GeneratePasswordHash("password")
	if err != nil {
		logger.Error("Failed to generate password hash", slog.Any("error", err))
		return
	}
	err = sqlite.PerformWrite(logger, dbConn, func(tx *gorm.DB) error {
		return tx.Exec(`
            INSERT INTO users (email, encrypted_password, created_at, updated_at)
            VALUES (?, ?, ?, ?)
            ON CONFLICT(email) DO NOTHING
        `, email, hashedPassword, time.Now().UTC(), time.Now().UTC()).Error
	})
	if err != nil {
		logger.Error("Failed to upsert admin user", slog.String("email", email), slog.Any("error", err))
		return
	}
	logger.Info("Ensured admin user exists", slog.String("email", email))
}
