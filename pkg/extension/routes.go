package extension

import (
	"sync"

	"github.com/gofiber/fiber/v2"
)

// RouteRegistrar is a function that registers routes on a Fiber app
type RouteRegistrar func(app *fiber.App)

var (
	routesMu   sync.Mutex
	proRoutes  []RouteRegistrar
)

// RegisterProRoutes adds a route registrar to be called during app setup
func RegisterProRoutes(r RouteRegistrar) {
	routesMu.Lock()
	defer routesMu.Unlock()
	proRoutes = append(proRoutes, r)
}

// ApplyProRoutes calls all registered route registrars
func ApplyProRoutes(app *fiber.App) {
	routesMu.Lock()
	defer routesMu.Unlock()
	for _, r := range proRoutes {
		r(app)
	}
}

// MiddlewareRegistrar is a function that registers middleware
type MiddlewareRegistrar func(app *fiber.App)

var (
	middlewareMu   sync.Mutex
	proMiddleware  []MiddlewareRegistrar
)

// RegisterProMiddleware adds a middleware registrar
func RegisterProMiddleware(m MiddlewareRegistrar) {
	middlewareMu.Lock()
	defer middlewareMu.Unlock()
	proMiddleware = append(proMiddleware, m)
}

// ApplyProMiddleware calls all registered middleware registrars
func ApplyProMiddleware(app *fiber.App) {
	middlewareMu.Lock()
	defer middlewareMu.Unlock()
	for _, m := range proMiddleware {
		m(app)
	}
}
