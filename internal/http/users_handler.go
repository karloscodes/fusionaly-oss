package http

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/crypto"
	"github.com/karloscodes/cartridge/inertia"
	"github.com/karloscodes/cartridge/flash"
	"log/slog"

	"fusionaly/internal/onboarding"
	"fusionaly/internal/users"
)

// RenderLoginAction renders the login page
func RenderLoginAction(ctx *cartridge.Context) error {
	ctx.Logger.Debug("is authenticated", slog.Bool("isAuthenticated", ctx.Session.IsAuthenticated(ctx.Ctx)))

	db := ctx.DB()

	// Check if onboarding is required first
	required, err := onboarding.IsOnboardingRequired(db)
	if err != nil {
		ctx.Logger.Error("Failed to check if onboarding is required on login", slog.Any("error", err))
	} else if required {
		ctx.Logger.Info("Login page accessed but onboarding required, redirecting to setup")
		return ctx.Redirect("/setup", fiber.StatusFound)
	}

	if ctx.Session.IsAuthenticated(ctx.Ctx) {
		return ctx.Redirect("/admin")
	}

	// Render the login page using Inertia (csrfToken and flash auto-injected)
	return inertia.RenderPage(ctx.Ctx, "Login", inertia.Props{})
}

// ProcessLoginAction handles the login form submission
func ProcessLoginAction(ctx *cartridge.Context) error {
	// Parse login form - try both form value and JSON body (for Inertia.js)
	email := ctx.FormValue("email")
	password := ctx.FormValue("password")
	tz := ctx.FormValue("_tz")

	// Try parsing as JSON for Inertia.js requests
	if email == "" && password == "" {
		var jsonBody struct {
			Email    string `json:"email"`
			Password string `json:"password"`
			Tz       string `json:"_tz"`
		}
		if err := ctx.BodyParser(&jsonBody); err == nil {
			if jsonBody.Email != "" {
				email = jsonBody.Email
			}
			if jsonBody.Password != "" {
				password = jsonBody.Password
			}
			if jsonBody.Tz != "" {
				tz = jsonBody.Tz
			}
		}
	}

	if email == "" || password == "" {
		// Set flash error message and redirect to login page
		flash.SetFlash(ctx.Ctx, "error", "Email and password are required")
		return ctx.Redirect("/login", fiber.StatusFound)
	}

	db := ctx.DB()

	// Find user by email
	user, result := users.FindByEmail(db, email)

	// Always verify password to prevent timing attacks
	// This ensures constant time regardless of whether user exists
	var passwordValid bool
	if result != nil {
		// User not found - verify against dummy hash to maintain constant time
		// This prevents attackers from determining if email exists based on response time
		ctx.Logger.Debug("User not found during login",
			slog.String("email", email))
		dummyHash := "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy" // bcrypt hash of "dummy"
		crypto.VerifyPassword(dummyHash, password)
		passwordValid = false
	} else {
		// User found - verify actual password
		passwordValid = crypto.VerifyPassword(user.EncryptedPassword, password)
		if !passwordValid {
			ctx.Logger.Debug("Invalid password attempt",
				slog.String("email", email))
		}
	}

	// Check if authentication failed (either user not found or wrong password)
	if !passwordValid {
		// Generic error message - don't reveal whether email exists
		flash.SetFlash(ctx.Ctx, "error", "Invalid email or password")
		return ctx.Redirect("/login", fiber.StatusFound)
	}

	// Set session cookie
	if err := ctx.Session.SetSession(ctx.Ctx, user.ID); err != nil {
		ctx.Logger.Error("Failed to set session", slog.Any("error", err))
		flash.SetFlash(ctx.Ctx, "error", "Login failed")
		return ctx.Redirect("/login", fiber.StatusFound)
	}
	ctx.Logger.Debug("Login successful",
		slog.String("email", email),
		slog.Int("userId", int(user.ID)))

	// Set timezone cookie with robust configuration (10 years expiration)
	tzExpiration := time.Now().Add(10 * 365 * 24 * time.Hour)
	ctx.Cookie(&fiber.Cookie{
		Name:     "_tz",
		Value:    tz,
		Path:     "/",                                        // Explicit path
		MaxAge:   int((10 * 365 * 24 * time.Hour).Seconds()), // 10 years in seconds
		Expires:  tzExpiration,                               // 10 years from now
		Secure:   ctx.Config.IsProduction(),                  // Match production setting
		HTTPOnly: true,
		SameSite: "Lax", // Less strict than "Strict"
		Domain:   "",    // Let browser determine
	})

	// Redirect to websites list (admin home)
	return ctx.Redirect("/admin", fiber.StatusFound)
}

// LogoutAction handles user logout
func LogoutAction(ctx *cartridge.Context) error {
	ctx.Logger.Debug("LogoutAction: Starting logout process",
		slog.String("path", ctx.Path()),
		slog.String("method", ctx.Method()))

	userID, isAuthenticated := ctx.Session.GetUserID(ctx.Ctx)
	ctx.Logger.Debug("LogoutAction: Current auth state",
		slog.Uint64("userID", uint64(userID)),
		slog.Bool("isAuthenticated", isAuthenticated))

	// Clear the session
	ctx.Session.ClearSession(ctx.Ctx)

	// Also clear the timezone cookie for clean logout
	ctx.ClearCookie("_tz")
	ctx.Cookie(&fiber.Cookie{
		Name:     "_tz",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Now().Add(-24 * time.Hour),
		Secure:   ctx.Config.IsProduction(),
		HTTPOnly: true,
		SameSite: "Lax",
	})

	// Set a flash message
	flash.SetFlash(ctx.Ctx, "success", "You have been successfully logged out")

	ctx.Logger.Debug("LogoutAction: User logged out, redirecting to login page")

	// Return a redirect to /login
	return ctx.Redirect("/login", fiber.StatusFound)
}
