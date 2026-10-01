package internal_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/testsupport"
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
}
