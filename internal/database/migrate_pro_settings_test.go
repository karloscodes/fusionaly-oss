package database_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"fusionaly/internal/database"
	"fusionaly/internal/settings"
	"fusionaly/internal/testsupport"
)

// seedProSettingsTable creates a Pro-shaped pro_settings key/value table and
// inserts the given key/value rows, simulating an existing Pro database.
func seedProSettingsTable(t *testing.T, db *gorm.DB, rows map[string]string) {
	t.Helper()

	require.NoError(t, db.Exec(`
		CREATE TABLE IF NOT EXISTS pro_settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)
	`).Error)

	for key, value := range rows {
		require.NoError(t, db.Exec(
			`INSERT INTO pro_settings (key, value, created_at, updated_at) VALUES (?, ?, ?, ?)`,
			key, value, time.Now().UTC(), time.Now().UTC(),
		).Error)
	}
}

func TestMigrateProSettings(t *testing.T) {
	t.Run("copies openai_api_key from pro_settings when OSS setting is empty", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		require.NoError(t, settings.SetupDefaultSettings(db))
		seedProSettingsTable(t, db, map[string]string{
			"openai_api_key": "sk-frompro",
			"license_key":    "lic-12345",
		})

		require.NoError(t, database.MigrateProSettings(db))

		got, err := settings.GetOpenAIKey(db)
		require.NoError(t, err)
		assert.Equal(t, "sk-frompro", got)
	})

	t.Run("does not overwrite an already-set OSS openai_api_key", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		require.NoError(t, settings.SetupDefaultSettings(db))
		require.NoError(t, settings.SaveOpenAIKey(db, "sk-existing-oss"))
		seedProSettingsTable(t, db, map[string]string{
			"openai_api_key": "sk-frompro",
		})

		require.NoError(t, database.MigrateProSettings(db))

		got, err := settings.GetOpenAIKey(db)
		require.NoError(t, err)
		assert.Equal(t, "sk-existing-oss", got)
	})

	t.Run("is a clean no-op when no pro_settings table exists", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		require.NoError(t, settings.SetupDefaultSettings(db))

		require.NoError(t, database.MigrateProSettings(db))

		got, err := settings.GetOpenAIKey(db)
		require.NoError(t, err)
		assert.Equal(t, "", got)
	})

	t.Run("is idempotent when run twice", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		require.NoError(t, settings.SetupDefaultSettings(db))
		seedProSettingsTable(t, db, map[string]string{
			"openai_api_key": "sk-frompro",
		})

		require.NoError(t, database.MigrateProSettings(db))
		require.NoError(t, database.MigrateProSettings(db))

		got, err := settings.GetOpenAIKey(db)
		require.NoError(t, err)
		assert.Equal(t, "sk-frompro", got)
	})
}
