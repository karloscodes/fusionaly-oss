// Package app provides the public API for Fusionaly OSS.
// This package exports types and functions for Pro to extend.
// DO NOT add Pro-specific code here - this is OSS only.
package app

import (
	"fusionaly/internal"
	"fusionaly/internal/config"
	"fusionaly/internal/database"
	"fusionaly/internal/http"
	"fusionaly/internal/onboarding"
	"fusionaly/internal/settings"
	"fusionaly/internal/websites"

	"github.com/karloscodes/cartridge"
	"gorm.io/gorm"
)

// Re-export core types
type (
	Application = internal.Application
	Config      = config.Config
	DBManager   = database.DBManager
)

// Re-export onboarding types
type (
	OnboardingStep    = onboarding.OnboardingStep
	OnboardingData    = onboarding.OnboardingData
	OnboardingSession = onboarding.OnboardingSession
	CompletionData    = onboarding.CompletionData
	CompletionResult  = onboarding.CompletionResult
)

// Re-export onboarding step constants
const (
	StepUserAccount = onboarding.StepUserAccount
	StepPassword    = onboarding.StepPassword
	StepGeoLite     = onboarding.StepGeoLite
	StepCompleted   = onboarding.StepCompleted
)

// GetConfig returns the application configuration
func GetConfig() *Config {
	return config.GetConfig()
}

// NewApp creates a new application with default routes
func NewApp() (*Application, error) {
	return internal.NewApp()
}

// NewAppWithRoutes creates a new application with custom route mounting
func NewAppWithRoutes(cfg *Config, routeMount func(*cartridge.Server)) (*Application, error) {
	return internal.NewAppWithRoutes(cfg, routeMount)
}

// SetupSession configures session management on the server
func SetupSession(srv *cartridge.Server) {
	internal.SetupSession(srv)
}

// MountAppRoutes mounts OSS routes (for Pro to call after its routes)
func MountAppRoutes(srv *cartridge.Server) {
	internal.MountAppRoutesWithoutSession(srv)
}

// DashboardPropsExtender is a function that can modify dashboard props before rendering.
type DashboardPropsExtender = http.DashboardPropsExtender

// HandleDashboard renders the dashboard page with custom component
func HandleDashboard(ctx *cartridge.Context, component string) error {
	return http.WebsiteDashboardActionWithExtension(ctx, component, nil)
}

// HandleDashboardWithExtension renders the dashboard with a props extender function.
// Used by Pro to inject additional props like insights.
func HandleDashboardWithExtension(ctx *cartridge.Context, component string, extender DashboardPropsExtender) error {
	return http.WebsiteDashboardActionWithExtension(ctx, component, extender)
}

// HandleLens renders the lens page with custom component
func HandleLens(ctx *cartridge.Context, component string) error {
	return http.WebsiteLensActionWithComponent(ctx, component)
}

// Onboarding functions
var (
	IsOnboardingRequired         = onboarding.IsOnboardingRequired
	GetOnboardingSession         = onboarding.GetOnboardingSession
	UpdateOnboardingSession      = onboarding.UpdateOnboardingSession
	CompleteOnboardingSession    = onboarding.CompleteOnboardingSession
	CompleteOnboarding           = onboarding.CompleteOnboarding
	GetOrCreateOnboardingSession = http.GetOrCreateOnboardingSession
)

// Settings functions
var (
	SaveGeoLiteCredentials = settings.SaveGeoLiteCredentials
)

// Websites functions

// GetWebsitesForSelector returns a list of websites formatted for the frontend selector
func GetWebsitesForSelector(db *gorm.DB) ([]map[string]interface{}, error) {
	return websites.GetWebsitesForSelector(db)
}
