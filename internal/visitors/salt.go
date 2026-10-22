package visitors

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/karloscodes/cartridge"
	"gorm.io/gorm"
)

// keyDailySalt is the settings row that holds today's salt as "day:salt".
const keyDailySalt = "visitor_daily_salt"

// The daily salt makes visitor signatures anonymous. Each UTC day gets a new
// random salt, and the new salt overwrites the old one in the same row. After
// that, nobody can link an old signature to an IP address and a User-Agent,
// not even the owner of the server.
//
// The salt is kept in the database, so a restart keeps today's visitors.
var (
	saltMu    sync.Mutex
	saltDay   string
	saltValue string
)

// DailySalt returns the salt of the current UTC day.
func DailySalt(dbManager cartridge.DBManager) (string, error) {
	return dailySaltOn(dbManager, time.Now().UTC())
}

func dailySaltOn(dbManager cartridge.DBManager, now time.Time) (string, error) {
	day := now.UTC().Format("2006-01-02")

	saltMu.Lock()
	defer saltMu.Unlock()
	if saltDay == day {
		return saltValue, nil
	}

	var salt string
	err := cartridge.Write(context.Background(), dbManager, func(tx *gorm.DB) error {
		var stored string
		if err := tx.Raw("SELECT value FROM settings WHERE key = ?", keyDailySalt).Scan(&stored).Error; err != nil {
			return err
		}
		if storedDay, storedSalt, ok := strings.Cut(stored, ":"); ok && storedDay == day {
			salt = storedSalt
			return nil
		}

		salt = newSalt()
		return tx.Exec(`
			INSERT INTO settings (key, value, created_at, updated_at) VALUES (?, ?, ?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
		`, keyDailySalt, day+":"+salt, now, now).Error
	})
	if err != nil {
		return "", fmt.Errorf("failed to load the daily salt: %w", err)
	}

	saltDay, saltValue = day, salt
	return salt, nil
}

func newSalt() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // crypto/rand.Read never fails on supported platforms
	return hex.EncodeToString(b)
}
