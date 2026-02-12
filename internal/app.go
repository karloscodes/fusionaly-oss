// Package internal contains core application functionality
package internal

import (
	"fmt"
	"io/fs"

	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/inertia"

	"fusionaly/internal/config"
	"fusionaly/internal/database"
	"fusionaly/internal/jobs"
)

// Application wraps cartridge.Application with fusionaly-specific components
type Application struct {
	*cartridge.Application
	DBManager *database.DBManager // Fusionaly-specific DB manager with migration methods
}

// AppOption configures the application
type AppOption func(*appOptions)

type appOptions struct {
	staticFS fs.FS
}

// WithStaticFS sets embedded static assets for production builds.
// In development mode, assets are served from disk for hot-reload.
func WithStaticFS(staticFS fs.FS) AppOption {
	return func(o *appOptions) {
		o.staticFS = staticFS
	}
}

// NewApp creates a new application instance with default settings
func NewApp(opts ...AppOption) (*Application, error) {
	cfg := config.GetConfig()
	return NewAppWithConfig(cfg, opts...)
}

// NewAppWithRoutes creates a new application with custom route mounting function
func NewAppWithRoutes(cfg *config.Config, routeMount func(*cartridge.Server), opts ...AppOption) (*Application, error) {
	// Apply options
	options := &appOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Enable Inertia dev mode in development (re-reads manifest on every request)
	if cfg.IsDevelopment() {
		inertia.SetDevMode(true)
	}

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

	// Configure server with SecFetchSite for cross-origin analytics
	// Analytics SDK sends events from customer sites (cross-site) to our API
	serverConfig := cartridge.DefaultServerConfig()
	serverConfig.SecFetchSiteAllowedValues = []string{"cross-site", "same-site", "same-origin"}

	// Use embedded static assets in production, disk in development for hot-reload
	if !cfg.IsDevelopment() && options.staticFS != nil {
		serverConfig.StaticFS = options.staticFS
	}

	// Create the cartridge application with custom route mount
	app, err := cartridge.NewApplication(cartridge.ApplicationOptions{
		Config:            cfg,
		Logger:            logger,
		DBManager:         dbManager,
		ServerConfig:      serverConfig,
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
func NewAppWithConfig(cfg *config.Config, opts ...AppOption) (*Application, error) {
	// Apply options
	options := &appOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Enable Inertia dev mode in development (re-reads manifest on every request)
	if cfg.IsDevelopment() {
		inertia.SetDevMode(true)
	}

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

	// Configure server with SecFetchSite for cross-origin analytics
	// Analytics SDK sends events from customer sites (cross-site) to our API
	serverConfig := cartridge.DefaultServerConfig()
	serverConfig.SecFetchSiteAllowedValues = []string{"cross-site", "same-site", "same-origin"}

	// Use embedded static assets in production, disk in development for hot-reload
	if !cfg.IsDevelopment() && options.staticFS != nil {
		serverConfig.StaticFS = options.staticFS
	}

	// Create the cartridge application using NewApplication
	app, err := cartridge.NewApplication(cartridge.ApplicationOptions{
		Config:            cfg,
		Logger:            logger,
		DBManager:         dbManager,
		ServerConfig:      serverConfig,
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
