// Package upgrader handles upgrading from Fusionaly OSS to Pro
package upgrader

import (
	"fmt"
	"path/filepath"

	"fusionaly/internal/manager/config"
	"fusionaly/internal/manager/database"
	"fusionaly/internal/manager/docker"
	"fusionaly/internal/manager/logging"
)

const (
	// ProAppImage is the Docker image for Fusionaly Pro
	ProAppImage = "karloscodes/fusionaly-pro:latest"
)

// Upgrader handles the OSS to Pro upgrade process
type Upgrader struct {
	logger   *logging.Logger
	config   *config.Config
	docker   *docker.Docker
	database *database.Database
}

// NewUpgrader creates a new Upgrader instance
func NewUpgrader(logger *logging.Logger) *Upgrader {
	fileLogger := logging.NewFileLogger(logging.Config{
		Level:   logger.Level.String(),
		Verbose: logger.GetVerbose(),
		Quiet:   logger.GetQuiet(),
		LogDir:  "/opt/fusionaly/logs",
		LogFile: "fusionaly-upgrader.log",
	})

	db := database.NewDatabase(fileLogger)
	return &Upgrader{
		logger:   fileLogger,
		config:   config.NewConfig(fileLogger),
		docker:   docker.NewDocker(fileLogger, db),
		database: db,
	}
}

// Run performs the upgrade from OSS to Pro
func (u *Upgrader) Run() error {
	data := u.config.GetData()
	envFile := filepath.Join(data.InstallDir, ".env")

	// Step 1: Load current configuration
	u.logger.Info("Step 1/4: Loading configuration")
	if err := u.config.LoadFromFile(envFile); err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Check if already running Pro
	data = u.config.GetData()
	if data.AppImage == ProAppImage {
		u.logger.Info("Already running Fusionaly Pro")
		return nil
	}

	// Step 2: Backup database before upgrade
	u.logger.Info("Step 2/4: Backing up database")
	mainDBPath := u.config.GetMainDBPath()
	backupDir := data.BackupPath
	if _, err := u.database.BackupDatabase(mainDBPath, backupDir); err != nil {
		u.logger.Warn("Failed to backup database: %v", err)
		u.logger.Warn("Proceeding with upgrade without backup")
	} else {
		u.logger.Success("Database backup created")
	}

	// Step 3: Switch to Pro image
	u.logger.Info("Step 3/4: Switching to Fusionaly Pro")
	data.AppImage = ProAppImage
	u.config.SetData(data)

	// Step 4: Update Docker containers
	u.logger.Info("Step 4/4: Updating containers")
	if err := u.docker.Update(u.config); err != nil {
		return fmt.Errorf("failed to update containers: %w", err)
	}

	// Save updated configuration
	if err := u.config.SaveToFile(envFile); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	u.logger.Success("Upgrade to Fusionaly Pro completed")
	return nil
}

// GetDomain returns the configured domain for display
func (u *Upgrader) GetDomain() string {
	return u.config.GetData().Domain
}
