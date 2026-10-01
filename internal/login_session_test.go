package internal_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/testsupport"
	"fusionaly/internal/users"
)

func TestLoginSession(t *testing.T) {
	t.Run("lasts 90 days", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)
		testsupport.CreateTestUserForAuth(t, db, "qa@example.com", "QaPassword123!")
		app := testsupport.CreateMinimalTestApp(t, db)
		req := httptest.NewRequest("POST", "/login", strings.NewReader(`{"email":"qa@example.com","password":"QaPassword123!"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Sec-Fetch-Site", "same-origin")

		resp, err := app.Test(req)

		require.NoError(t, err)
		var session *http.Cookie
		for _, c := range resp.Cookies() {
			if c.Name == "fusionaly_session" {
				session = c
			}
		}
		require.NotNil(t, session, "login must set the session cookie")
		assert.Equal(t, 90*24*60*60, session.MaxAge)
	})

	t.Run("ends every earlier session when the password changes", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)
		db := dbManager.GetConnection()
		testsupport.CleanAllTables(db)
		testsupport.CreateTestUserForAuth(t, db, "qa@example.com", "QaPassword123!")
		app := testsupport.CreateMinimalTestApp(t, db)
		stolen := login(t, app, "qa@example.com", "QaPassword123!")
		require.Equal(t, http.StatusOK, adminStatus(t, app, stolen), "the session works before the change")

		require.NoError(t, users.ChangePassword(db, "qa@example.com", "NewPassword456!"))

		assert.Equal(t, http.StatusFound, adminStatus(t, app, stolen), "the old cookie goes back to the login page")
		fresh := login(t, app, "qa@example.com", "NewPassword456!")
		assert.Equal(t, http.StatusOK, adminStatus(t, app, fresh), "a new login works")
	})
}

func login(t *testing.T, app http.Handler, email, password string) *http.Cookie {
	t.Helper()
	req := httptest.NewRequest("POST", "/login", strings.NewReader(`{"email":"`+email+`","password":"`+password+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	for _, c := range rec.Result().Cookies() {
		if c.Name == "fusionaly_session" && c.Value != "" {
			return c
		}
	}
	t.Fatalf("login set no session cookie (status %d)", rec.Code)
	return nil
}

func adminStatus(t *testing.T, app http.Handler, session *http.Cookie) int {
	t.Helper()
	req := httptest.NewRequest("GET", "/admin/administration/account", nil)
	req.AddCookie(session)
	req.AddCookie(&http.Cookie{Name: "_tz", Value: "UTC"})
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	return rec.Code
}
