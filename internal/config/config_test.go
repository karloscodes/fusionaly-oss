package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidatePrivateKey(t *testing.T) {
	valid := func(key string) *Config {
		return &Config{Environment: Production, DatabaseType: SQLiteDatabase, PrivateKey: key}
	}

	t.Run("accepts a key of 32 characters or more", func(t *testing.T) {
		err := valid(strings.Repeat("a", 32)).validate()

		assert.NoError(t, err)
	})

	t.Run("rejects a shorter key with a clear message", func(t *testing.T) {
		err := valid("short").validate()

		assert.ErrorContains(t, err, "FUSIONALY_PRIVATE_KEY must have at least 32 characters")
	})
}

func TestLoad(t *testing.T) {
	env := func(vars map[string]string) func(string) string {
		return func(name string) string { return vars[name] }
	}

	t.Run("uses the defaults without environment variables", func(t *testing.T) {
		c, err := load(env(nil))

		assert.NoError(t, err)
		assert.Equal(t, "fusionaly", c.AppName)
		assert.Equal(t, "3000", c.AppPort)
		assert.Equal(t, Development, c.Environment)
		assert.Equal(t, 1800, c.SessionTimeoutSeconds)
		assert.Equal(t, 7776000, c.LoginSessionTimeoutSeconds)
		assert.Equal(t, "storage", c.DatabasePath)
		assert.Equal(t, "storage/GeoLite2-City.mmdb", c.GeoDBPath)
		assert.Equal(t, "web/dist/assets", c.PublicDirectory)
		assert.Equal(t, "/", c.PublicAssetsUrlPrefix)
		assert.Equal(t, "logs", c.LogsDirectory)
		assert.Equal(t, 20, c.LogsMaxSizeInMb)
		assert.Equal(t, 10, c.LogsMaxBackups)
		assert.Equal(t, 30, c.LogsMaxAgeInDays)
		assert.Equal(t, SQLiteDatabase, c.DatabaseType)
		assert.Equal(t, 60, c.JobIntervalSeconds)
		assert.Equal(t, 90, c.IngestedEventsRetentionDays)
		assert.Equal(t, "csrf", c.CSRFContextKey)
		assert.Equal(t, LogLevel(""), c.LogLevel)
	})

	t.Run("reads FUSIONALY_HOST, empty by default", func(t *testing.T) {
		def, _ := load(env(nil))
		c, err := load(env(map[string]string{"FUSIONALY_HOST": "0.0.0.0"}))

		assert.NoError(t, err)
		assert.Equal(t, "", def.GetHost())
		assert.Equal(t, "0.0.0.0", c.GetHost())
	})

	t.Run("reads PRIVATE_KEY, the name Chasen gives the key", func(t *testing.T) {
		c, err := load(env(map[string]string{"PRIVATE_KEY": strings.Repeat("c", 64)}))

		assert.NoError(t, err)
		assert.Equal(t, strings.Repeat("c", 64), c.PrivateKey)
	})

	t.Run("FUSIONALY_PRIVATE_KEY wins over PRIVATE_KEY", func(t *testing.T) {
		c, err := load(env(map[string]string{"FUSIONALY_PRIVATE_KEY": strings.Repeat("f", 64), "PRIVATE_KEY": strings.Repeat("c", 64)}))

		assert.NoError(t, err)
		assert.Equal(t, strings.Repeat("f", 64), c.PrivateKey)
	})

	t.Run("reads every FUSIONALY_ variable", func(t *testing.T) {
		c, err := load(env(map[string]string{
			"FUSIONALY_APP_NAME": "acme", "FUSIONALY_APP_PORT": "8080", "FUSIONALY_ENV": Production,
			"FUSIONALY_LOG_LEVEL": "warn", "FUSIONALY_PRIVATE_KEY": strings.Repeat("k", 64),
			"FUSIONALY_SESSION_TIMEOUT_SECONDS": "600", "FUSIONALY_LOGIN_SESSION_TIMEOUT_SECONDS": "3600",
			"FUSIONALY_DOMAIN": "t.example.com",
			"FUSIONALY_STORAGE_PATH": "/data", "FUSIONALY_GEO_DB_PATH": "/geo.mmdb", "FUSIONALY_PUBLIC_DIR": "/pub",
			"FUSIONALY_PUBLIC_ASSETS_URL_PREFIX": "/static/", "FUSIONALY_LOGS_DIR": "/logs",
			"FUSIONALY_LOGS_MAX_SIZE_IN_MB": "5", "FUSIONALY_LOGS_MAX_BACKUPS": "2", "FUSIONALY_LOGS_MAX_AGE_IN_DAYS": "7",
			"FUSIONALY_DB_TYPE": SQLiteDatabase, "FUSIONALY_DB_MAX_OPEN_CONNS": "4", "FUSIONALY_DB_MAX_IDLE_CONNS": "2",
			"FUSIONALY_JOB_INTERVAL_SECONDS": "15", "FUSIONALY_INGESTED_EVENTS_RETENTION_DAYS": "30",
		}))

		assert.NoError(t, err)
		assert.Equal(t, "acme", c.AppName)
		assert.Equal(t, "8080", c.AppPort)
		assert.Equal(t, Production, c.Environment)
		assert.Equal(t, LogLevelWarn, c.LogLevel)
		assert.Equal(t, 600, c.SessionTimeoutSeconds)
		assert.Equal(t, 3600, c.LoginSessionTimeoutSeconds)
		assert.Equal(t, "t.example.com", c.Domain)
		assert.Equal(t, "/data", c.DatabasePath)
		assert.Equal(t, "/geo.mmdb", c.GeoDBPath)
		assert.Equal(t, "/pub", c.PublicDirectory)
		assert.Equal(t, "/static/", c.PublicAssetsUrlPrefix)
		assert.Equal(t, "/logs", c.LogsDirectory)
		assert.Equal(t, 5, c.LogsMaxSizeInMb)
		assert.Equal(t, 2, c.LogsMaxBackups)
		assert.Equal(t, 7, c.LogsMaxAgeInDays)
		assert.Equal(t, 4, c.DatabaseMaxOpenConns)
		assert.Equal(t, 2, c.DatabaseMaxIdleConns)
		assert.Equal(t, 15, c.JobIntervalSeconds)
		assert.Equal(t, 30, c.IngestedEventsRetentionDays)
	})

	t.Run("treats an empty variable as unset", func(t *testing.T) {
		c, err := load(env(map[string]string{"FUSIONALY_APP_PORT": "", "FUSIONALY_JOB_INTERVAL_SECONDS": ""}))

		assert.NoError(t, err)
		assert.Equal(t, "3000", c.AppPort)
		assert.Equal(t, 60, c.JobIntervalSeconds)
	})

	t.Run("rejects a number that does not parse", func(t *testing.T) {
		_, err := load(env(map[string]string{"FUSIONALY_JOB_INTERVAL_SECONDS": "soon"}))

		assert.ErrorContains(t, err, "FUSIONALY_JOB_INTERVAL_SECONDS")
	})
}
