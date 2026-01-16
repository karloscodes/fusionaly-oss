package jobs

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"fusionaly/internal/config"
	"fusionaly/internal/database"
	"fusionaly/internal/pkg/geoip"
	"fusionaly/internal/settings"
)

const (
	// GeoLite database is updated weekly by MaxMind
	GeoLiteUpdateInterval = 7 * 24 * time.Hour
	// MaxMind download URL template
	MaxMindDownloadURL = "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-City&license_key=%s&suffix=tar.gz"
	// Settings keys for GeoLite
	KeyGeoLiteLastUpdate = "geolite_last_update"
)

// GeoLiteUpdaterJob handles automatic GeoLite database updates
type GeoLiteUpdaterJob struct {
	dbManager *database.DBManager
	logger    *slog.Logger
	cfg       *config.Config
}

// NewGeoLiteUpdaterJob creates a new GeoLite updater job
func NewGeoLiteUpdaterJob(dbManager *database.DBManager, logger *slog.Logger, cfg *config.Config) *GeoLiteUpdaterJob {
	return &GeoLiteUpdaterJob{
		dbManager: dbManager,
		logger:    logger,
		cfg:       cfg,
	}
}

// Run executes the GeoLite update job
func (j *GeoLiteUpdaterJob) Run() error {
	db := j.dbManager.GetConnection()

	// Get credentials from settings
	accountID, licenseKey, err := settings.GetGeoLiteCredentials(db)
	if err != nil {
		j.logger.Debug("Failed to get GeoLite credentials", slog.Any("error", err))
		return nil // Not an error - just not configured
	}

	// Skip if credentials are not configured
	if accountID == "" || licenseKey == "" {
		j.logger.Debug("GeoLite credentials not configured, skipping update")
		return nil
	}

	// Check last update time
	lastUpdate := j.getLastUpdateTime()
	if time.Since(lastUpdate) < GeoLiteUpdateInterval {
		j.logger.Debug("GeoLite database is up to date",
			slog.Time("last_update", lastUpdate),
			slog.Duration("age", time.Since(lastUpdate)))
		return nil
	}

	j.logger.Info("Starting GeoLite database update",
		slog.Time("last_update", lastUpdate))

	// Download and update the database
	if err := j.downloadAndUpdate(licenseKey); err != nil {
		j.logger.Error("Failed to update GeoLite database", slog.Any("error", err))
		return err
	}

	// Reload the in-memory database so event processor can use it immediately
	geoip.ReloadGeoDB()

	// Update last update time
	if err := j.setLastUpdateTime(time.Now()); err != nil {
		j.logger.Error("Failed to update last update time", slog.Any("error", err))
	}

	j.logger.Info("GeoLite database updated successfully")
	return nil
}

// getLastUpdateTime returns the last time the GeoLite database was updated
func (j *GeoLiteUpdaterJob) getLastUpdateTime() time.Time {
	db := j.dbManager.GetConnection()
	lastUpdateStr, err := settings.GetSetting(db, KeyGeoLiteLastUpdate)
	if err != nil || lastUpdateStr == "" {
		return time.Time{} // Never updated
	}

	lastUpdate, err := time.Parse(time.RFC3339, lastUpdateStr)
	if err != nil {
		return time.Time{}
	}
	return lastUpdate
}

// setLastUpdateTime sets the last update time in settings
func (j *GeoLiteUpdaterJob) setLastUpdateTime(t time.Time) error {
	db := j.dbManager.GetConnection()
	return settings.CreateOrUpdateSetting(db, KeyGeoLiteLastUpdate, t.Format(time.RFC3339))
}

// downloadAndUpdate downloads and extracts the GeoLite database
func (j *GeoLiteUpdaterJob) downloadAndUpdate(licenseKey string) error {
	// Determine download path
	geoDBPath := j.cfg.GeoDBPath
	if geoDBPath == "" {
		geoDBPath = filepath.Join("storage", "GeoLite2-City.mmdb")
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(geoDBPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Download the database
	downloadURL := fmt.Sprintf(MaxMindDownloadURL, licenseKey)
	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download GeoLite database: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// Create a temp file for the download
	tempFile, err := os.CreateTemp("", "geolite-*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Copy to temp file
	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		return fmt.Errorf("failed to save download: %w", err)
	}

	// Seek to beginning for extraction
	if _, err := tempFile.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}

	// Extract the .mmdb file
	if err := j.extractMMDB(tempFile, geoDBPath); err != nil {
		return fmt.Errorf("failed to extract database: %w", err)
	}

	return nil
}

// extractMMDB extracts the .mmdb file from the tar.gz archive
func (j *GeoLiteUpdaterJob) extractMMDB(tarGzFile *os.File, destPath string) error {
	gzr, err := gzip.NewReader(tarGzFile)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}

		// Look for the .mmdb file
		if strings.HasSuffix(header.Name, ".mmdb") {
			// Create the destination file
			outFile, err := os.Create(destPath)
			if err != nil {
				return fmt.Errorf("failed to create output file: %w", err)
			}
			defer outFile.Close()

			// Copy the content
			if _, err := io.Copy(outFile, tr); err != nil {
				return fmt.Errorf("failed to extract file: %w", err)
			}

			return nil
		}
	}

	return fmt.Errorf("no .mmdb file found in archive")
}

// IsGeoLiteConfigured checks if GeoLite credentials are configured
func IsGeoLiteConfigured(dbManager *database.DBManager) bool {
	db := dbManager.GetConnection()
	accountID, licenseKey, _ := settings.GetGeoLiteCredentials(db)
	return accountID != "" && licenseKey != ""
}

// TriggerImmediateDownload triggers an immediate GeoLite database download.
// Use this after saving new credentials to avoid waiting for the scheduled job.
// It runs in a goroutine to avoid blocking the caller.
func TriggerImmediateDownload(db *gorm.DB, logger *slog.Logger, cfg *config.Config) {
	go func() {
		// Get credentials
		accountID, licenseKey, err := settings.GetGeoLiteCredentials(db)
		if err != nil {
			logger.Debug("Failed to get GeoLite credentials for immediate download", slog.Any("error", err))
			return
		}

		if accountID == "" || licenseKey == "" {
			logger.Debug("GeoLite credentials not configured, skipping immediate download")
			return
		}

		logger.Info("Starting immediate GeoLite database download")

		// Create a minimal job structure just for download
		job := &GeoLiteUpdaterJob{
			dbManager: nil, // not needed for download
			logger:    logger,
			cfg:       cfg,
		}

		if err := job.downloadAndUpdate(licenseKey); err != nil {
			logger.Error("Failed immediate GeoLite download", slog.Any("error", err))
			return
		}

		// Reload the in-memory database so event processor can use it immediately
		geoip.ReloadGeoDB()

		// Update last update time directly
		if err := settings.CreateOrUpdateSetting(db, KeyGeoLiteLastUpdate, time.Now().Format(time.RFC3339)); err != nil {
			logger.Error("Failed to update last update time", slog.Any("error", err))
		}

		logger.Info("Immediate GeoLite database download completed successfully")
	}()
}

// GetGeoLiteStatus returns the status of GeoLite configuration
func GetGeoLiteStatus(dbManager *database.DBManager) (configured bool, dbExists bool, lastUpdate time.Time) {
	db := dbManager.GetConnection()

	// Check if credentials are configured
	accountID, licenseKey, _ := settings.GetGeoLiteCredentials(db)
	configured = accountID != "" && licenseKey != ""

	// Check if database file exists
	cfg := config.GetConfig()
	geoDBPath := cfg.GeoDBPath
	if geoDBPath == "" {
		geoDBPath = filepath.Join("storage", "GeoLite2-City.mmdb")
	}
	_, err := os.Stat(geoDBPath)
	dbExists = err == nil

	// Get last update time
	lastUpdateStr, _ := settings.GetSetting(db, KeyGeoLiteLastUpdate)
	if lastUpdateStr != "" {
		lastUpdate, _ = time.Parse(time.RFC3339, lastUpdateStr)
	}

	return
}
