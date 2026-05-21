package database

import (
	"strings"

	"gorm.io/gorm"

	"fusionaly/internal/settings"
)

// MigrateProSettings is a one-time, idempotent migration that copies the OpenAI
// API key from a legacy Pro database into the OSS settings table.
//
// Pro stored its OpenAI key in a key/value `pro_settings` table (rows like
// key='openai_api_key', key='license_key'). Merged OSS reads the key from the
// OSS `settings` table via settings.GetOpenAIKey. The only data gap when a Pro
// install becomes a merged-OSS install is that the key lives in the wrong table.
//
// Behavior:
//   - No-op on a fresh OSS install (no pro_settings table).
//   - Copies pro_settings.openai_api_key into settings only when the OSS key is
//     currently empty, so re-running it never overwrites a user-set value.
//   - Leaves pro_settings untouched (a dropped table cannot be rolled back); the
//     license_key row is simply ignored.
func MigrateProSettings(db *gorm.DB) error {
	// Fresh OSS install: nothing to migrate.
	if !db.Migrator().HasTable("pro_settings") {
		return nil
	}

	// If the OSS key is already set, leave it alone (also makes this idempotent).
	currentKey, err := settings.GetOpenAIKey(db)
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	if strings.TrimSpace(currentKey) != "" {
		return nil
	}

	// Read the Pro-stored OpenAI key.
	var proKey string
	if err := db.Raw(
		"SELECT value FROM pro_settings WHERE key = ? LIMIT 1",
		settings.KeyOpenAIKey,
	).Scan(&proKey).Error; err != nil {
		return err
	}

	if strings.TrimSpace(proKey) == "" {
		return nil
	}

	return settings.SaveOpenAIKey(db, proKey)
}
