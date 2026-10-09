// Package config reads the application configuration from environment variables.
package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	AppName                    string
	AppPort                    string
	Host                       string
	Environment                string
	LogLevel                   LogLevel
	PrivateKey                 string
	SessionTimeoutSeconds      int
	LoginSessionTimeoutSeconds int
	CSRFContextKey             string
	Domain                     string

	// File paths
	DatabasePath          string
	DatabaseName          string // Derived from other settings
	GeoDBPath             string
	PublicDirectory       string
	PublicAssetsUrlPrefix string

	// Logging settings
	LogsDirectory    string
	LogsMaxSizeInMb  int
	LogsMaxBackups   int
	LogsMaxAgeInDays int

	// Database settings
	DatabaseType         string
	DatabaseMaxOpenConns int
	DatabaseMaxIdleConns int

	// Job scheduling settings
	JobIntervalSeconds int

	// Data retention settings
	IngestedEventsRetentionDays int
}

var (
	cfg  *Config
	once sync.Once
)

// GetConfig returns the application configuration
func GetConfig() *Config {
	once.Do(func() {
		var err error
		cfg, err = load(os.Getenv)
		if err != nil {
			log.Fatalf("config: %v", err)
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

// load builds the configuration from the FUSIONALY_ environment variables
// that getenv returns, with defaults for the ones that are unset. An empty
// variable counts as unset.
func load(getenv func(string) string) (*Config, error) {
	str := func(name, def string) string {
		if v := getenv(name); v != "" {
			return v
		}
		return def
	}
	var bad error
	num := func(name string, def int) int {
		v := getenv(name)
		if v == "" {
			return def
		}
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil && bad == nil {
			bad = fmt.Errorf("%s must be a whole number, got %q", name, v)
		}
		return n
	}

	c := &Config{
		AppName:                     str("FUSIONALY_APP_NAME", "fusionaly"),
		AppPort:                     str("FUSIONALY_APP_PORT", "3000"),
		Host:                        str("FUSIONALY_HOST", ""),
		Environment:                 str("FUSIONALY_ENV", Development),
		LogLevel:                    LogLevel(str("FUSIONALY_LOG_LEVEL", "")),                                             // empty: cartridge picks one per environment
		PrivateKey:                  str("FUSIONALY_PRIVATE_KEY", str("PRIVATE_KEY", "88888888888888888888888888888888")), // PRIVATE_KEY: the name Chasen gives it
		SessionTimeoutSeconds:       num("FUSIONALY_SESSION_TIMEOUT_SECONDS", 1800),
		LoginSessionTimeoutSeconds:  num("FUSIONALY_LOGIN_SESSION_TIMEOUT_SECONDS", 7776000), // 90 days
		CSRFContextKey:              "csrf",
		Domain:                      str("FUSIONALY_DOMAIN", ""),
		DatabasePath:                str("FUSIONALY_STORAGE_PATH", "storage"),
		GeoDBPath:                   str("FUSIONALY_GEO_DB_PATH", "storage/GeoLite2-City.mmdb"),
		PublicDirectory:             str("FUSIONALY_PUBLIC_DIR", "web/dist/assets"),
		PublicAssetsUrlPrefix:       str("FUSIONALY_PUBLIC_ASSETS_URL_PREFIX", "/"),
		LogsDirectory:               str("FUSIONALY_LOGS_DIR", "logs"),
		LogsMaxSizeInMb:             num("FUSIONALY_LOGS_MAX_SIZE_IN_MB", 20),
		LogsMaxBackups:              num("FUSIONALY_LOGS_MAX_BACKUPS", 10),
		LogsMaxAgeInDays:            num("FUSIONALY_LOGS_MAX_AGE_IN_DAYS", 30),
		DatabaseType:                str("FUSIONALY_DB_TYPE", SQLiteDatabase),
		DatabaseMaxOpenConns:        num("FUSIONALY_DB_MAX_OPEN_CONNS", 0),
		DatabaseMaxIdleConns:        num("FUSIONALY_DB_MAX_IDLE_CONNS", 0),
		JobIntervalSeconds:          num("FUSIONALY_JOB_INTERVAL_SECONDS", 60),
		IngestedEventsRetentionDays: num("FUSIONALY_INGESTED_EVENTS_RETENTION_DAYS", 90),
	}
	return c, bad
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

	// The key signs login sessions; cartridge refuses a shorter one. Fail at
	// startup with a clear message, not with a panic in the route setup.
	if len(c.PrivateKey) < 32 {
		return fmt.Errorf("FUSIONALY_PRIVATE_KEY must have at least 32 characters; generate one with: openssl rand -hex 32")
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
// GetHost returns the address to listen on. Empty lets cartridge choose:
// every interface in production, loopback only in development and test. A
// test-mode container behind a proxy, like the local demo, sets 0.0.0.0.
func (c *Config) GetHost() string {
	return c.Host
}

func (c *Config) GetPort() string {
	return c.AppPort
}

// GetPublicDirectory returns the path to public/static assets (implements cartridge.Config interface).
func (c *Config) GetPublicDirectory() string {
	return c.PublicDirectory
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
// If not explicitly set, defaults based on environment:
// - Production: warn (errors and warnings only, no query noise)
// - Development: debug (verbose, shows GORM queries)
// - Test: error (minimal output)
func (c *Config) GetLogLevel() string {
	if c.LogLevel != "" {
		return string(c.LogLevel)
	}
	// Default based on environment
	switch c.Environment {
	case Production:
		return string(LogLevelWarn)
	case Test:
		return string(LogLevelError)
	default:
		return string(LogLevelDebug)
	}
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
