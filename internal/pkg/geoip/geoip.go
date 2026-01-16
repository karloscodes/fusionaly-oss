package geoip

import (
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/oschwald/geoip2-golang"

	"fusionaly/internal/config"
)

var (
	geoDB  *geoip2.Reader
	once   sync.Once
	mu     sync.RWMutex
	logger *slog.Logger
)

// InitLogger sets the logger for the geoip package.
func InitLogger(l *slog.Logger) {
	logger = l
}

// InitGeoDB initializes the GeoLite2 database.
// Returns nil if the database is not configured or not found (GeoIP is optional).
func InitGeoDB() *geoip2.Reader {
	config := config.GetConfig()
	if config.GeoDBPath == "" {
		if logger != nil {
			logger.Debug("GeoIP database path not configured - GeoIP features disabled")
		}
		return nil
	}

	if logger != nil {
		logger.Debug("GeoIP database path from config",
			slog.String("path", config.GeoDBPath),
			slog.String("environment", config.Environment))
	}

	// Get absolute path to help with debugging
	absPath, err := filepath.Abs(config.GeoDBPath)
	if err != nil {
		if logger != nil {
			logger.Error("Failed to get absolute path for GeoDB",
				slog.String("path", config.GeoDBPath),
				slog.Any("error", err))
		}
	} else if logger != nil {
		logger.Debug("GeoIP database absolute path", slog.String("abs_path", absPath))
	}

	// Check if the file exists (GeoIP is optional)
	fileInfo, err := os.Stat(config.GeoDBPath)
	if os.IsNotExist(err) {
		if logger != nil {
			logger.Info("GeoLite2 database not found - GeoIP features disabled",
				slog.String("path", config.GeoDBPath),
				slog.String("hint", "Download from https://www.maxmind.com/en/geolite2/signup"))
		}
		return nil
	} else if err != nil {
		if logger != nil {
			logger.Warn("Error checking GeoLite2 database file",
				slog.String("path", config.GeoDBPath),
				slog.Any("error", err))
		}
		return nil
	}

	// Log file metadata to help debug permissions issues
	if logger != nil {
		logger.Debug("GeoIP database file details",
			slog.String("path", config.GeoDBPath),
			slog.Int64("size_bytes", fileInfo.Size()),
			slog.String("mode", fileInfo.Mode().String()),
			slog.Time("mod_time", fileInfo.ModTime()))
	}

	// Try to open the database
	db, err := geoip2.Open(config.GeoDBPath)
	if err != nil {
		if logger != nil {
			logger.Error("Failed to open GeoLite2 database",
				slog.String("path", config.GeoDBPath),
				slog.Any("error", err))
		}
		return nil
	}

	// Verify database has data by trying a test lookup
	if logger != nil {
		testIP := net.ParseIP("8.8.8.8") // Google DNS as a test
		if testIP != nil {
			country, err := db.Country(testIP)
			if err != nil {
				logger.Warn("Database opened but test lookup failed",
					slog.String("test_ip", "8.8.8.8"),
					slog.Any("error", err))
			} else {
				logger.Debug("Test lookup successful",
					slog.String("test_ip", "8.8.8.8"),
					slog.String("country", country.Country.IsoCode))
			}
		}

		logger.Info("GeoLite2 database initialized successfully",
			slog.String("path", config.GeoDBPath),
			slog.String("db_type", "GeoIP2-Country"))
	}
	return db
}

// GetGeoDB returns the GeoLite2 database reader, initializing it if necessary.
func GetGeoDB() *geoip2.Reader {
	once.Do(func() {
		mu.Lock()
		geoDB = InitGeoDB()
		mu.Unlock()
	})
	mu.RLock()
	defer mu.RUnlock()
	return geoDB
}

// ReloadGeoDB reloads the GeoLite2 database from disk.
// Call this after downloading a new database file.
func ReloadGeoDB() {
	mu.Lock()
	defer mu.Unlock()

	// Close existing database if open
	if geoDB != nil {
		geoDB.Close()
	}

	// Reinitialize
	geoDB = InitGeoDB()

	if geoDB != nil && logger != nil {
		logger.Info("GeoLite2 database reloaded successfully")
	}
}
