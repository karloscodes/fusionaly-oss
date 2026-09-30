package internal

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/karloscodes/cartridge/testsupport"
	"github.com/stretchr/testify/assert"

	"fusionaly/internal/config"
	"fusionaly/internal/users"
)

func TestPublicEventsRouteRateLimited(t *testing.T) {
	srv := testsupport.NewTestServer(t, testsupport.TestServerOptions{
		RouteMountFunc: MountAppRoutes,
	})
	// The public rate limiter only runs in production.
	cfg := config.GetConfig()
	env := cfg.Environment
	cfg.Environment = config.Production
	t.Cleanup(func() { cfg.Environment = env })
	post := func() int {
		resp, _ := srv.Server.Test(httptest.NewRequest("POST", "/x/api/v1/events", nil))
		return resp.StatusCode
	}
	for range 70 {
		post()
	}

	status := post()

	assert.Equal(t, http.StatusTooManyRequests, status, "the 71st request in a minute is limited")
}

func TestLensRoutesRegistered(t *testing.T) {
	srv := testsupport.NewTestServer(t, testsupport.TestServerOptions{
		Models:         []any{&users.User{}},
		RouteMountFunc: MountAppRoutes,
	})

	resp, _ := srv.Server.Test(httptest.NewRequest("GET", "/admin/websites/1/lens", nil))

	// The admin chain runs: with no user yet, it sends the visitor to setup.
	assert.Equal(t, http.StatusFound, resp.StatusCode)
	assert.Equal(t, "/setup", resp.Header.Get("Location"))
}
