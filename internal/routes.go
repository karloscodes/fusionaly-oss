package internal

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/karloscodes/cartridge"
	cartridgemiddleware "github.com/karloscodes/cartridge/middleware"

	v1 "fusionaly/api/v1"
	"fusionaly/internal/config"
	"fusionaly/internal/http"
	"fusionaly/internal/http/middleware"
)

// publicCORSConfig returns the standard CORS configuration for public endpoints.
// All public endpoints share this permissive CORS setup for cross-origin access.
var publicCORSConfig = &cors.Config{
	AllowOrigins: "*",
	AllowMethods: "POST,GET,OPTIONS",
	AllowHeaders: "Origin, Content-Type, Accept, Authorization, Referrer, User-Agent",
}

// SetupSession configures session management on the server.
// Exported for Pro to call before mounting its routes.
func SetupSession(srv *cartridge.Server) {
	cfg := config.GetConfig()
	sessionMgr := cartridge.NewSessionManager(cartridge.SessionConfig{
		CookieName: cfg.AppName + "_session",
		Secret:     cfg.GetSessionSecret(),
		TTL:        time.Duration(cfg.GetLoginSessionTimeout()) * time.Second,
		Secure:     cfg.IsProduction(),
		LoginPath:  "/login",
	})
	srv.SetSession(sessionMgr)
}

// MountAppRoutes mounts all application routes using cartridge's route API
func MountAppRoutes(srv *cartridge.Server) {
	// Create and set session manager
	SetupSession(srv)
	MountAppRoutesWithoutSession(srv)
}

// MountAppRoutesWithoutSession mounts routes without setting up session.
// Used by Pro which sets up session separately.
func MountAppRoutesWithoutSession(srv *cartridge.Server) {
	cfg := config.GetConfig()
	sessionMgr := srv.Session()

	// ============================================
	// PUBLIC ENDPOINT PROTECTION
	// All public endpoints get the following protection:
	// - Rate limiting (70 req/min for events, production only)
	// - CORS (permissive for cross-origin tracking)
	// - Sec-Fetch-Site validation where applicable
	// ============================================

	// Helper to conditionally apply rate limiting (only in production)
	// In development/test, rate limiting would interfere with testing
	conditionalRateLimiter := func(limiter fiber.Handler) fiber.Handler {
		return func(c *fiber.Ctx) error {
			if cfg.IsProduction() {
				return limiter(c)
			}
			return c.Next()
		}
	}

	// Rate limiter for public event ingestion API (70 requests per minute per IP)
	// 70/min = ~1.2 req/sec - handles legitimate analytics traffic while preventing abuse
	publicRateLimiter := conditionalRateLimiter(cartridgemiddleware.RateLimiter(
		cartridgemiddleware.WithMax(70),
		cartridgemiddleware.WithDuration(time.Minute),
	))

	// Stricter rate limiter for auth endpoints (10 requests per minute)
	// Prevents brute force login attempts
	authRateLimiter := conditionalRateLimiter(cartridgemiddleware.RateLimiter(
		cartridgemiddleware.WithMax(10),
		cartridgemiddleware.WithDuration(time.Minute),
	))

	// ============================================
	// ROUTE CONFIGURATIONS
	// ============================================

	// Public API config (event ingestion)
	// Rate limiting + CORS + Sec-Fetch-Site (global middleware handles validation)
	// CORS runs first ensuring 403 responses have CORS headers
	// Global SecFetchSite middleware allows: cross-site, same-site, same-origin
	publicAPIConfig := &cartridge.RouteConfig{
		EnableCORS:       true,
		WriteConcurrency: false,
		CustomMiddleware: []fiber.Handler{publicRateLimiter},
		CORSConfig:       publicCORSConfig,
	}

	// SDK delivery config
	// Rate limiting + CORS (no Sec-Fetch-Site needed for GET-only)
	sdkConfig := &cartridge.RouteConfig{
		EnableCORS:       true,
		CustomMiddleware: []fiber.Handler{publicRateLimiter},
		CORSConfig:       publicCORSConfig,
	}

	// Onboarding config
	// No rate limiting - one-time setup flow, not sensitive auth
	// No Sec-Fetch-Site - internal page navigation
	onboardingConfig := &cartridge.RouteConfig{
		EnableSecFetchSite: cartridge.Bool(false),
	}

	// Get dependencies for middleware
	db := srv.GetDBManager().GetConnection()
	logger := srv.GetLogger()

	adminConfig := &cartridge.RouteConfig{
		CustomMiddleware: []fiber.Handler{
			middleware.OnboardingCheck(db, logger),
			sessionMgr.Middleware(),
			middleware.WebsiteFilter(db, logger),
		},
	}

	adminAPIConfig := &cartridge.RouteConfig{
		CustomMiddleware: []fiber.Handler{
			middleware.OnboardingCheck(db, logger),
			sessionMgr.Middleware(),
			middleware.WebsiteFilter(db, logger),
		},
	}

	// === ROOT ROUTES ===
	srv.Get("/", http.HomeIndexAction)

	// Health check endpoint
	srv.Get("/_health", http.HealthIndexAction)
	srv.Head("/_health", http.HealthIndexAction)

	srv.Get("/_demo", http.DemoIndexAction)

	// === PUBLIC DASHBOARD SHARING ===
	// Rate limited to prevent abuse (same as public API)
	publicDashboardConfig := &cartridge.RouteConfig{
		CustomMiddleware: []fiber.Handler{publicRateLimiter},
	}
	srv.Get("/share/:token", http.PublicDashboardAction, publicDashboardConfig)

	// === PUBLIC API ROUTES ===
	srv.Post("/x/api/v1/events", v1.CreateEventPublicAPIHandler, publicAPIConfig)
	srv.Options("/x/api/v1/events", func(ctx *cartridge.Context) error {
		return ctx.SendStatus(fiber.StatusNoContent)
	}, publicAPIConfig)
	srv.Post("/x/api/v1/events/beacon", v1.CreateEventBeaconHandler, publicAPIConfig)
	srv.Options("/x/api/v1/events/beacon", func(ctx *cartridge.Context) error {
		return ctx.SendStatus(fiber.StatusNoContent)
	}, publicAPIConfig)
	srv.Get("/x/api/v1/me", v1.GetVisitorInfoHandler, publicAPIConfig)
	srv.Options("/x/api/v1/me", func(ctx *cartridge.Context) error {
		return ctx.SendStatus(fiber.StatusNoContent)
	}, publicAPIConfig)
	srv.Get("/x/api/v1/you", v1.GetVisitorInfoHandler, publicAPIConfig)
	srv.Options("/x/api/v1/you", func(ctx *cartridge.Context) error {
		return ctx.SendStatus(fiber.StatusNoContent)
	}, publicAPIConfig)

	// === SDK ROUTES ===
	srv.Get("/y/api/v1/sdk.js", v1.GetSDKAction, sdkConfig)

	// === ONBOARDING ROUTES (PRG pattern) ===
	srv.Get("/setup", http.OnboardingPageAction, onboardingConfig)
	srv.Get("/api/onboarding/check", http.OnboardingCheckAction, onboardingConfig)
	srv.Post("/setup/user", http.OnboardingUserFormAction, onboardingConfig)
	srv.Post("/setup/password", http.OnboardingPasswordFormAction, onboardingConfig)
	srv.Post("/setup/geolite", http.OnboardingGeoLiteFormAction, onboardingConfig)

	// === AUTHENTICATION ROUTES ===
	// Login needs rate limiting to prevent brute force attacks
	loginConfig := &cartridge.RouteConfig{
		CustomMiddleware: []fiber.Handler{authRateLimiter},
	}
	srv.Get("/login", http.RenderLoginAction)
	srv.Post("/login", http.ProcessLoginAction, loginConfig)
	srv.Post("/logout", http.LogoutAction)

	// === PROTECTED ADMIN ROUTES ===
	srv.Get("/admin", http.WebsitesIndexAction, adminConfig)

	srv.Get("/admin/websites/new", http.WebsiteNewPageAction, adminConfig)
	srv.Post("/admin/websites", http.WebsiteCreateAction, adminConfig)

	srv.Get("/admin/websites/:id/setup", http.WebsiteSetupPageAction, adminConfig)
	srv.Get("/admin/websites/:id/dashboard", http.WebsiteDashboardAction, adminConfig)
	srv.Get("/admin/websites/:id/events", http.WebsiteEventsAction, adminConfig)
	srv.Get("/admin/websites/:id/lens", http.WebsiteLensAction, adminConfig)
	srv.Get("/admin/websites/:id/edit", http.WebsiteEditPageAction, adminConfig)
	srv.Post("/admin/websites/:id", http.WebsiteUpdateAction, adminConfig)
	srv.Delete("/admin/websites/:id", http.WebsiteDeleteAction, adminConfig)
	srv.Post("/admin/websites/:id/delete", http.WebsiteDeleteAction, adminConfig)

	srv.Post("/admin/websites/:id/annotations", http.AnnotationCreateAction, adminConfig)
	srv.Post("/admin/websites/:id/annotations/:annotationId", http.AnnotationUpdateAction, adminConfig)
	srv.Post("/admin/websites/:id/annotations/:annotationId/delete", http.AnnotationDeleteAction, adminConfig)

	// Dashboard sharing
	srv.Post("/admin/websites/:id/share/enable", http.EnableShareAction, adminConfig)
	srv.Post("/admin/websites/:id/share/disable", http.DisableShareAction, adminConfig)

	// === ADMINISTRATION ROUTES ===
	srv.Get("/admin/administration", http.AdministrationIndexAction, adminConfig)
	srv.Get("/admin/administration/ingestion", http.AdministrationIngestionPageAction, adminConfig)
	srv.Get("/admin/administration/account", http.AdministrationAccountPageAction, adminConfig)
	srv.Get("/admin/administration/system", http.AdministrationSystemPageAction, adminConfig)

	srv.Post("/admin/account/change-password", http.AccountChangePasswordFormAction, adminConfig)

	// === SYSTEM API ROUTES ===
	srv.Get("/admin/api/system/export-database", http.SystemExportDatabaseAction, adminAPIConfig)
	srv.Get("/admin/api/system/health", http.SystemHealthAction, adminAPIConfig)
	srv.Post("/admin/system/purge-cache", http.SystemPurgeCacheFormAction, adminConfig)
	srv.Post("/admin/system/geolite", http.SystemGeoLiteFormAction, adminConfig)
	srv.Post("/admin/system/geolite/download", http.SystemGeoLiteDownloadAction, adminConfig)
	srv.Post("/admin/ingestion/settings", http.IngestionSettingsFormAction, adminConfig)
}
