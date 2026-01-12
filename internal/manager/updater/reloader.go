package updater

import (
	"fmt"
	"path/filepath"

	"fusionaly/internal/manager/config"
	"fusionaly/internal/manager/database"
	"fusionaly/internal/manager/docker"
	"fusionaly/internal/manager/logging"
)

// Reloader handles container reload operations without database backups or other update steps
type Reloader struct {
	logger *logging.Logger
	config *config.Config
	docker *docker.Docker
}

// NewReloader creates a Reloader instance
func NewReloader(logger *logging.Logger) *Reloader {
	fileLogger := logging.NewFileLogger(logging.Config{
		Level:   logger.Level.String(),
		Verbose: logger.GetVerbose(),
		Quiet:   logger.GetQuiet(),
		LogDir:  "/opt/fusionaly/logs",
		LogFile: "fusionaly-reloader.log",
	})

	db := database.NewDatabase(fileLogger) // Need database for Docker constructor
	return &Reloader{
		logger: fileLogger,
		config: config.NewConfig(fileLogger),
		docker: docker.NewDocker(fileLogger, db),
	}
}

// Run executes the reload operation
func (r *Reloader) Run() error {
	r.logger.Info("Starting container reload with latest config")

	// Load configuration
	data := r.config.GetData()
	envFile := filepath.Join(data.InstallDir, ".env")
	r.logger.Info("Loading configuration from %s", envFile)
	if err := r.config.LoadFromFile(envFile); err != nil {
		return fmt.Errorf("failed to load config from %s: %w", envFile, err)
	}

	// Skip server fetch intentionally to just use local config

	// Reload containers with our simpler method
	r.logger.Info("Reloading Docker containers with latest config")
	if err := r.docker.Reload(r.config); err != nil {
		return fmt.Errorf("failed to reload Docker containers: %w", err)
	}

	r.logger.Success("Container reload completed successfully")
	return nil
}
