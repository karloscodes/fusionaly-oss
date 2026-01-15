// Package internal contains core application functionality
package internal

import (
	"fmt"

	"github.com/karloscodes/cartridge"

	"fusionaly/internal/config"
	"fusionaly/internal/database"
	"fusionaly/internal/jobs"
)

// Application wraps cartridge.Application with fusionaly-specific components
type Application struct {
	*cartridge.Application
	DBManager *database.DBManager // Fusionaly-specific DB manager with migration methods
}

// NewApp creates a new application instance with default settings
func NewApp() (*Application, error) {
	cfg := config.GetConfig()
	return NewAppWithConfig(cfg)
}

// NewAppWithRoutes creates a new application with custom route mounting function
func NewAppWithRoutes(cfg *config.Config, routeMount func(*cartridge.Server)) (*Application, error) {
	// Create logger
	logger := cartridge.NewLogger(cfg, nil)

	// Initialize database manager (fusionaly-specific with migration methods)
	dbManager := database.NewDBManager(cfg, logger)
	if err := dbManager.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize jobs system
	jobsManager, err := jobs.NewJobs(dbManager, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize jobs: %w", err)
	}

	// Create the cartridge application with custom route mount
	app, err := cartridge.NewApplication(cartridge.ApplicationOptions{
		Config:            cfg,
		Logger:            logger,
		DBManager:         dbManager,
		RouteMountFunc:    routeMount,
		BackgroundWorkers: []cartridge.BackgroundWorker{jobsManager},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create application: %w", err)
	}

	return &Application{
		Application: app,
		DBManager:   dbManager,
	}, nil
}

// NewAppWithConfig creates a new application with the provided config
func NewAppWithConfig(cfg *config.Config) (*Application, error) {
	// Create logger
	logger := cartridge.NewLogger(cfg, nil)

	// Initialize database manager (fusionaly-specific with migration methods)
	dbManager := database.NewDBManager(cfg, logger)
	if err := dbManager.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize jobs system
	jobsManager, err := jobs.NewJobs(dbManager, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize jobs: %w", err)
	}

	// Create the cartridge application using NewApplication
	app, err := cartridge.NewApplication(cartridge.ApplicationOptions{
		Config:            cfg,
		Logger:            logger,
		DBManager:         dbManager,
		RouteMountFunc:    MountAppRoutes,
		BackgroundWorkers: []cartridge.BackgroundWorker{jobsManager},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create application: %w", err)
	}

	return &Application{
		Application: app,
		DBManager:   dbManager,
	}, nil
}
