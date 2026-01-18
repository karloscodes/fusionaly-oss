// Package config provides configuration management using Viper
package config

import (
	"fmt"
	"log"
	"path/filepath"
	"sync"

	"github.com/spf13/viper"
)

// Environment types
const (
	Development = "development"
	Production  = "production"
	Test        = "test"
)

// LogLevel represents the logging level for the application
type LogLevel string

// Available log levels
const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// Database types
const (
	SQLiteDatabase = "sqlite"
)

// Config holds all configuration parameters for the application
type Config struct {
	// Application settings
	AppName               string   `mapstructure:"appname"`
	AppPort               string   `mapstructure:"appport"`
	Environment           string   `mapstructure:"environment"`
	LogLevel              LogLevel `mapstructure:"loglevel"`
	PrivateKey            string `mapstructure:"privatekey"`
	SessionTimeoutSeconds      int `mapstructure:"sessiontimeoutseconds"`
	LoginSessionTimeoutSeconds int `mapstructure:"loginsessiontimeoutseconds"`
	CSRFContextKey        string   `mapstructure:"-"`
	AdminEmail            string   `mapstructure:"adminemail"`
	Domain                string   `mapstructure:"domain"`

	// File paths
	DatabasePath          string `mapstructure:"storagepath"`
	DatabaseName          string `mapstructure:"-"` // Derived from other settings
	GeoDBPath             string `mapstructure:"geodbpath"`
	PublicDirectory       string `mapstructure:"publicdir"`
	PublicAssetsUrlPrefix string `mapstructure:"publicassetsurlprefix"`

	// Logging settings
	LogsDirectory    string `mapstructure:"logsdir"`
	LogsMaxSizeInMb  int    `mapstructure:"logsmaxsizeinmb"`
	LogsMaxBackups   int    `mapstructure:"logsmaxbackups"`
	LogsMaxAgeInDays int    `mapstructure:"logsmaxageindays"`

	// Database settings
	DatabaseType         string `mapstructure:"dbtype"`
	DatabaseMaxOpenConns int    `mapstructure:"dbmaxopenconns"`
	DatabaseMaxIdleConns int    `mapstructure:"dbmaxidleconns"`

	// Pro feature placeholders (for compatibility with fusionaly-pro)
	LicenseKey   string `mapstructure:"licensekey"`
	OpenAIAPIKey string `mapstructure:"openaiapikey"`

	// Job scheduling settings
	JobIntervalSeconds int `mapstructure:"jobintervalseconds"`

	// Data retention settings
	IngestedEventsRetentionDays int `mapstructure:"ingestedeventsretentiondays"`
}

var (
	cfg  *Config
	once sync.Once
)

// GetConfig returns the application configuration
func GetConfig() *Config {
	once.Do(func() {
		v := viper.New()

		// Set defaults (matching envconfig defaults)
		v.SetDefault("appname", "fusionaly")
		v.SetDefault("appport", "3000")
		v.SetDefault("environment", Development)
		v.SetDefault("loglevel", string(LogLevelDebug))
		v.SetDefault("privatekey", "88888888888888888888888888888888")
		v.SetDefault("sessiontimeoutseconds", 1800)
		v.SetDefault("loginsessiontimeoutseconds", 604800) // 1 week
		v.SetDefault("storagepath", "storage")
		v.SetDefault("geodbpath", "storage/GeoLite2-City.mmdb")
		v.SetDefault("publicdir", "web/dist/assets")
		v.SetDefault("publicassetsurlprefix", "/")
		v.SetDefault("logsdir", "logs")
		v.SetDefault("logsmaxsizeinmb", 20)
		v.SetDefault("logsmaxbackups", 10)
		v.SetDefault("logsmaxageindays", 30)
		v.SetDefault("dbtype", SQLiteDatabase)
		v.SetDefault("dbmaxopenconns", 0)
		v.SetDefault("dbmaxidleconns", 0)
		v.SetDefault("jobintervalseconds", 60)
		v.SetDefault("ingestedeventsretentiondays", 90)

		// Bind environment variables (same names as envconfig)
		v.BindEnv("appname", "FUSIONALY_APP_NAME")
		v.BindEnv("appport", "FUSIONALY_APP_PORT")
		v.BindEnv("environment", "FUSIONALY_ENV")
		v.BindEnv("loglevel", "FUSIONALY_LOG_LEVEL")
		v.BindEnv("privatekey", "FUSIONALY_PRIVATE_KEY")
		v.BindEnv("sessiontimeoutseconds", "FUSIONALY_SESSION_TIMEOUT_SECONDS")
		v.BindEnv("loginsessiontimeoutseconds", "FUSIONALY_LOGIN_SESSION_TIMEOUT_SECONDS")
		v.BindEnv("adminemail", "FUSIONALY_ADMIN_EMAIL")
		v.BindEnv("domain", "FUSIONALY_DOMAIN")
		v.BindEnv("storagepath", "FUSIONALY_STORAGE_PATH")
		v.BindEnv("geodbpath", "FUSIONALY_GEO_DB_PATH")
		v.BindEnv("publicdir", "FUSIONALY_PUBLIC_DIR")
		v.BindEnv("publicassetsurlprefix", "FUSIONALY_PUBLIC_ASSETS_URL_PREFIX")
		v.BindEnv("logsdir", "FUSIONALY_LOGS_DIR")
		v.BindEnv("logsmaxsizeinmb", "FUSIONALY_LOGS_MAX_SIZE_IN_MB")
		v.BindEnv("logsmaxbackups", "FUSIONALY_LOGS_MAX_BACKUPS")
		v.BindEnv("logsmaxageindays", "FUSIONALY_LOGS_MAX_AGE_IN_DAYS")
		v.BindEnv("dbtype", "FUSIONALY_DB_TYPE")
		v.BindEnv("dbmaxopenconns", "FUSIONALY_DB_MAX_OPEN_CONNS")
		v.BindEnv("dbmaxidleconns", "FUSIONALY_DB_MAX_IDLE_CONNS")
		v.BindEnv("licensekey", "FUSIONALY_LICENSE_KEY")
		v.BindEnv("openaiapikey", "OPENAI_API_KEY")
		v.BindEnv("jobintervalseconds", "FUSIONALY_JOB_INTERVAL_SECONDS")
		v.BindEnv("ingestedeventsretentiondays", "FUSIONALY_INGESTED_EVENTS_RETENTION_DAYS")

		cfg = &Config{
			CSRFContextKey: "csrf",
		}
		if err := v.Unmarshal(cfg); err != nil {
			log.Fatalf("config: failed to unmarshal configuration: %v", err)
		}

		// Validate
		if err := cfg.validate(); err != nil {
			log.Fatalf("config: invalid configuration: %v", err)
		}

		// Set derived values
		cfg.DatabaseName = cfg.GetDatabasePath()

		// Validate private key - in production, must be explicitly set (not empty, not default)
		defaultKey := "88888888888888888888888888888888"
		if cfg.PrivateKey == "" {
			log.Fatal("Private key is required")
		}
		if cfg.IsProduction() && cfg.PrivateKey == defaultKey {
			log.Fatal("Production requires a unique FUSIONALY_PRIVATE_KEY (cannot use default)")
		}
	})
	return cfg
}

// validate checks the configuration for errors
func (c *Config) validate() error {
	validEnvs := map[string]bool{
		Development: true,
		Production:  true,
		Test:        true,
	}
	if !validEnvs[c.Environment] {
		return fmt.Errorf("invalid environment: %s", c.Environment)
	}

	validDBTypes := map[string]bool{
		SQLiteDatabase: true,
	}
	if !validDBTypes[c.DatabaseType] {
		return fmt.Errorf("invalid database type: %s", c.DatabaseType)
	}

	return nil
}

// GetDatabasePath returns the appropriate database path based on environment
func (c *Config) GetDatabasePath() string {
	if c.DatabaseName == "" {
		c.DatabaseName = filepath.Join(c.DatabasePath,
			fmt.Sprintf("%s-%s.db", c.AppName, c.Environment))
	}
	return c.DatabaseName
}

// IsDevelopment returns true if the environment is development
func (c *Config) IsDevelopment() bool {
	return c.Environment == Development
}

// IsProduction returns true if the environment is production
func (c *Config) IsProduction() bool {
	return c.Environment == Production
}

// IsTest returns true if the environment is test
func (c *Config) IsTest() bool {
	return c.Environment == Test
}

// GetPort returns the HTTP server port (implements cartridge.Config interface).
func (c *Config) GetPort() string {
	return c.AppPort
}

// GetPublicDirectory returns the path to public/static assets (implements cartridge.Config interface).
func (c *Config) GetPublicDirectory() string {
	return c.PublicDirectory
}

// GetAssetsPrefix returns the URL prefix for static assets (implements cartridge.Config interface).
func (c *Config) GetAssetsPrefix() string {
	return c.PublicAssetsUrlPrefix
}

// GetAppName returns the application name (implements cartridge.FactoryConfig interface).
func (c *Config) GetAppName() string {
	return c.AppName
}

// DatabaseDSN returns the database connection string (implements cartridge.FactoryConfig interface).
func (c *Config) DatabaseDSN() string {
	return c.GetDatabasePath()
}

// GetSessionSecret returns the session encryption key (implements cartridge.FactoryConfig interface).
func (c *Config) GetSessionSecret() string {
	return c.PrivateKey
}

// GetSessionTimeout returns the analytics session timeout in seconds.
// Used for visitor session tracking (when a visitor's session expires after inactivity).
func (c *Config) GetSessionTimeout() int {
	return c.SessionTimeoutSeconds
}

// GetLoginSessionTimeout returns the login session timeout in seconds.
// Used for admin login cookie duration.
func (c *Config) GetLoginSessionTimeout() int {
	return c.LoginSessionTimeoutSeconds
}

// GetMaxOpenConns returns the appropriate MaxOpenConns value based on environment
// If explicitly set via env var, uses that value. Otherwise:
// - Test: 1 (required for E2E test stability)
// - Development/Production: 10 (allows concurrent reads for parallel dashboard queries)
func (c *Config) GetMaxOpenConns() int {
	if c.DatabaseMaxOpenConns > 0 {
		return c.DatabaseMaxOpenConns
	}

	if c.Environment == Test {
		return 1 // Required for E2E test stability
	}

	return 10 // Higher concurrency for development and production
}

// GetMaxIdleConns returns the appropriate MaxIdleConns value based on environment
// If explicitly set via env var, uses that value. Otherwise:
// - Test: 1 (matches MaxOpenConns for test stability)
// - Development/Production: 5 (keep half the connections warm for reuse)
func (c *Config) GetMaxIdleConns() int {
	if c.DatabaseMaxIdleConns > 0 {
		return c.DatabaseMaxIdleConns
	}

	if c.Environment == Test {
		return 1 // Matches MaxOpenConns for test stability
	}

	return 5 // Keep half the pool warm for development and production
}

// GetLogLevel returns the log level as a string (implements cartridge.LogConfigProvider).
func (c *Config) GetLogLevel() string {
	return string(c.LogLevel)
}

// GetLogDirectory returns the logs directory (implements cartridge.LogConfigProvider).
func (c *Config) GetLogDirectory() string {
	return c.LogsDirectory
}

// GetLogMaxSizeMB returns the max log file size in MB (implements cartridge.LogConfigProvider).
func (c *Config) GetLogMaxSizeMB() int {
	return c.LogsMaxSizeInMb
}

// GetLogMaxBackups returns the max number of log backups (implements cartridge.LogConfigProvider).
func (c *Config) GetLogMaxBackups() int {
	return c.LogsMaxBackups
}

// GetLogMaxAgeDays returns the max age in days for log files (implements cartridge.LogConfigProvider).
func (c *Config) GetLogMaxAgeDays() int {
	return c.LogsMaxAgeInDays
}

// Reset clears the cached configuration; intended for tests.
func Reset() {
	once = sync.Once{}
	cfg = nil
}
