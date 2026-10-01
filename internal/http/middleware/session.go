package middleware

import (
	"log/slog"
	"net/http"

	"github.com/karloscodes/cartridge"
	"gorm.io/gorm"

	"fusionaly/internal/users"
)

// ValidSession ends a session that its user has ended since it was issued,
// for example by changing the password. A session cookie is signed, not
// stored, so the server checks its issue time on each request. It runs after
// the session manager's middleware.
func ValidSession(db *gorm.DB, sessions *cartridge.SessionManager, logger *slog.Logger) cartridge.HandlerFunc {
	return func(c *cartridge.Context) error {
		userID, ok := sessions.GetUserID(c)
		if !ok {
			return c.Redirect("/login", http.StatusFound)
		}
		issuedAt, ok := sessions.IssuedAt(c)
		valid := false
		if ok {
			var err error
			valid, err = users.SessionStillValid(db, userID, issuedAt)
			if err != nil {
				logger.Warn("Failed to check the session", slog.Uint64("user_id", uint64(userID)), slog.Any("error", err))
			}
		}
		if !valid {
			sessions.ClearSession(c)
			return c.Redirect("/login", http.StatusFound)
		}
		return c.Next()
	}
}
