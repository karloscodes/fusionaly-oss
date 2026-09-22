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
	if password == "" {
		return errors.New("password cannot be empty")
	}

	hashedPassword, err := crypto.GeneratePasswordHash(password)
	if err != nil {
		return err
	}

	return CreateAdminUserWithHash(dbConn, email, string(hashedPassword))
}

// CreateAdminUserWithHash creates a new admin user from an already hashed
// password. It returns ErrUserExists if the user already exists.
func CreateAdminUserWithHash(dbConn *gorm.DB, email, passwordHash string) error {
	if email == "" {
		return errors.New("email cannot be empty")
	}
	if passwordHash == "" {
		return errors.New("password cannot be empty")
	}
	if _, err := FindByEmail(dbConn, email); err == nil {
		return ErrUserExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	newUser := User{
		Email:             email,
		EncryptedPassword: passwordHash,
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
