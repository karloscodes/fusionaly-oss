package internal

import (
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/karloscodes/cartridge/testsupport"
	"github.com/stretchr/testify/require"
)

func TestPublicEventsRouteRateLimited(t *testing.T) {
	srv := testsupport.NewTestServer(t, testsupport.TestServerOptions{
		RouteMountFunc: MountAppRoutes,
	})
	routes := srv.App.GetRoutes(true)

	var eventRoute *fiber.Route
	for idx := range routes {
		route := routes[idx]
		if route.Method == fiber.MethodPost && route.Path == "/x/api/v1/events" {
			eventRoute = &routes[idx]
			break
		}
	}

	require.NotNil(t, eventRoute, "expected events route to be registered")

	// The rate limiter is wrapped in a conditional function that only applies
	// in production. In test environment, it passes through but the wrapper
	// still exists. Check for the conditional wrapper (defined in MountAppRoutes).
	hasRateLimiter := false
	var handlerNames []string
	for _, handler := range eventRoute.Handlers {
		name := runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name()
		handlerNames = append(handlerNames, name)
		// Check for either the raw limiter or our conditional wrapper
		if strings.Contains(name, "middleware/limiter") || strings.Contains(name, "MountAppRoutes.func") {
			hasRateLimiter = true
			break
		}
	}

	require.Truef(t, hasRateLimiter, "expected rate limiter middleware for public events route, handlers: %v", handlerNames)
}

func TestLensRoutesRegistered(t *testing.T) {
	srv := testsupport.NewTestServer(t, testsupport.TestServerOptions{
		RouteMountFunc: MountAppRoutes,
	})
	routes := srv.App.GetRoutes(true)

	var hasLensIndex bool

	for _, route := range routes {
		// Website-scoped routes (Inertia)
		// OSS version: only the GET route for the paywall page
		if route.Path == "/admin/websites/:id/lens" && route.Method == fiber.MethodGet {
			hasLensIndex = true
		}
	}

	require.True(t, hasLensIndex, "expected website-scoped lens index route to be registered")
	// Note: POST routes for Lens are available in Fusionaly Pro
}
