# Developer Agent Guidelines

This is the **canonical source** for all AI coding assistants (Claude, Copilot, Codex) working on Fusionaly OSS.

---

## Core Philosophy

- **Simple over clever:** readable, debuggable, maintainable code
- **Pragmatic, not perfect:** clarity over architectural purity
- **Sustainability:** one person should be able to maintain this for years
- **Convention over configuration:** minimize setup, maximize productivity
- **No fluff:** avoid unnecessary dependencies, abstractions, or buzzwords

> **When in doubt, choose the simplest path that works — and that you can maintain alone.**

---

## Architecture: Phoenix Contexts Pattern

Fusionaly follows **Phoenix Contexts** architecture for clean separation of concerns:

```
internal/
├── http/                 # HTTP Transport Layer
│   ├── *_handler.go     # Thin handlers (HTTP concerns only)
│   ├── middleware/      # HTTP middleware
│   └── routes.go        # Route registration (in internal/)
│
├── analytics/           # Context: Analytics domain (metrics, dashboards)
├── events/              # Context: Events domain (tracking, ingestion)
├── websites/            # Context: Websites domain
├── users/               # Context: Users domain
├── visitors/            # Context: Visitors domain
├── onboarding/          # Context: Onboarding domain
├── settings/            # Context: Settings domain
├── timeframe/           # Context: Time ranges
├── annotations/         # Context: Dashboard annotations
├── jobs/                # Context: Background jobs (event processing, cleanup)
├── config/              # App configuration
├── database/            # Database setup
└── pkg/                 # Shared utilities
```

### Context Rules

1. **Domain-focused:** Each context owns its business logic
2. **Top-level functions:** Export functions, not service objects
3. **Accept `*gorm.DB`:** All functions take DB as parameter
4. **No HTTP dependencies:** Contexts never import `internal/http`
5. **Clean boundaries:** Contexts can call other contexts, but minimize coupling

### Handler Pattern

```go
// ✅ GOOD: Thin handler delegating to context
func DashboardAction(ctx *cartridge.Context) error {
    db := ctx.DB()
    websiteID := ctx.QueryInt("website_id", 0)
    timeframe := timeframe.ParseTimeframe(ctx.Query("range"))

    metrics := analytics.GetDashboardMetrics(db, websiteID, timeframe)
    return inertia.RenderPage(ctx.Ctx, "Dashboard", inertia.Props{
        "metrics": metrics,
    })
}

// ❌ BAD: Business logic in handler
func DashboardAction(ctx *cartridge.Context) error {
    db := ctx.DB()
    var visitors int64
    db.Model(&visitors.Visitor{}).Count(&visitors) // Direct DB access
    // ... complex calculations ...
}
```

---

## Development Workflow

### Low-Risk Changes
For refactors, small features, or bug fixes:

```bash
# 1. Make changes
vim internal/analytics/metrics.go

# 2. Run unit tests frequently
make test                    # All tests (~3 seconds)
make test t="TestMetrics"   # Specific test

# 3. After several changes, run E2E
make test-e2e               # Full suite (~5 minutes)
```

### Test-Driven Development
```bash
# Write failing test first
vim internal/users/users_test.go

# Run test watch loop
make test t="TestCreateUser"

# Implement until green
vim internal/users/users.go

# Refactor with confidence
```

### Deployment
```bash
make release v=1.0.0    # Tag + Push + GoReleaser builds
```

---

## Project Snapshot

- **Stack**: Go 1.23+ (Fiber, GORM, SQLite, Cartridge) + React 19/TypeScript/Tailwind
- **Architecture**: Phoenix Contexts pattern
- **Domain**: Privacy-first analytics
- **Database**: SQLite with WAL mode
- **Binaries**: `cmd/fusionaly` (server) + `cmd/fnctl` (CLI/migrations)

### Repo Landmarks

- **`internal/http/`**: HTTP handlers, middleware
- **`internal/routes.go`**: Route registration
- **`internal/{domain}/`**: Domain contexts (analytics, events, users, etc.)
- **`api/v1/`**: Public ingestion + SDK endpoints
- **`web/`**: React SPA (shadcn/ui, Tailwind, Inertia.js)
- **`e2e/`**: Playwright E2E test suites
- **`cmd/`**: Main binaries (fusionaly, fnctl, manager)
- **`storage/`**: Runtime data (GeoLite2 database, uploads)

---

## Everyday Commands

```bash
make dev          # Run Go API + React dev server with hot reload
make watch-go     # Only rebuild/run Go backend on change
make watch-web    # Only run React dev server
make db-migrate   # Apply SQLite migrations via fnctl
make db-seed      # Run migrations and load sample data
make test         # Reset test DB and run Go tests (~3s)
make test-e2e     # Build web app and run Playwright E2E tests (~5m)
make lint         # Run staticcheck and golangci-lint
make build        # Produce production binaries + bundled frontend
make check        # Run all CI checks locally
make release v=X  # Create tagged release (triggers GoReleaser)
```

---

## UI Architecture

**STRICT REQUIREMENT: Inertia.js Everywhere**

All pages MUST use Inertia.js patterns. No exceptions. No fetch() API calls for page data.

### Architecture Overview

1. **ALL PAGES**: Inertia.js + React + Tailwind (shadcn/ui)
   - Every page is a React component receiving props via Inertia
   - Server uses `inertia.RenderPage()` to pass data
   - React components use `usePage()` to access props
   - Consistent component library (shadcn/ui)

2. **FORMS**: PRG Pattern (POST → Redirect → GET)
   - All forms use HTML `<form action="..." method="POST">`
   - Server validates, sets flash message, redirects
   - Flash messages displayed via `props.flash`
   - NO fetch() or JSON API calls for form submissions
   - Examples: Settings, Websites, Account, Login, Onboarding

3. **DATA FLOW**:
   - Server prepares ALL data needed for render
   - Pass data as Inertia props
   - React receives via `usePage<Props>()`
   - NO client-side data fetching on page load

4. **NAVIGATION**: Inertia Link or HTML forms
   - Use Inertia's `<Link>` for navigation
   - Full page transitions handled by Inertia
   - Progress bar shown automatically

### What NOT to do

```tsx
// ❌ BAD: Using fetch() for data
const handleSubmit = async () => {
  const response = await fetch('/api/some-endpoint', {
    method: 'POST',
    body: JSON.stringify(data)
  });
  const result = await response.json();
};

// ✅ GOOD: Using HTML form with PRG pattern
<form action="/setup/user" method="POST">
  <input name="email" type="email" />
  <Button type="submit">Continue</Button>
</form>
```

### Server Handler Pattern

```go
// ✅ GOOD: Inertia page render
func OnboardingPageAction(ctx *cartridge.Context) error {
    props := inertia.Props{
        "step":  session.Step,
        "email": session.Data.Email,
    }
    return inertia.RenderPage(ctx.Ctx, "Onboarding", props)
}

// ✅ GOOD: PRG form handler
func OnboardingUserFormAction(ctx *cartridge.Context) error {
    email := ctx.FormValue("email")
    // ... validation ...
    if err != nil {
        flash.SetFlash(ctx.Ctx, "error", "Invalid email")
        return ctx.Redirect("/setup", fiber.StatusFound)
    }
    // ... save data ...
    return ctx.Redirect("/setup", fiber.StatusFound)
}
```

### Exception: Streaming/Real-time Only

JSON APIs (`ctx.JSON()`) are ONLY allowed for:
- SSE/NDJSON streaming responses
- WebSocket-like real-time updates

---

## Route Protection

### Public Endpoints (event ingestion, SDK)
- Rate limiting: 70 req/min per IP (production only)
- CORS: Permissive for cross-origin tracking
- Sec-Fetch-Site validation: Only allows browser-initiated requests (cross-site, same-site, same-origin)
- Rejects direct requests (curl, Postman, scripts without browser context)

### Auth Endpoints (login)
- Rate limiting: 10 req/min per IP (brute force protection)

### Admin Endpoints
- Session-based authentication
- Onboarding check middleware
- Website filter middleware

---

## Strong Requirements

- **Inertia.js everywhere:** No fetch() for page data, use PRG pattern
- **No arbitrary docs:** Don't create .md files unless explicitly requested
- **Test integrity:** Never skip tests or mark as pending to pass CI
- **Privacy first:** No IP storage, cookie-less tracking

---

## Testing

- **Always** run `make test` after changes
- **Always** run `make test-e2e` after features or big changes
- **A failure is a failure:** Never cheat on tests - fix the actual issue
  - No warnings instead of failures
  - No pending/skip marks to bypass failing tests
  - No catching errors and logging instead of asserting
  - No increasing timeouts to hide race conditions
  - Fix the root cause, not the symptom
- E2E specs assume onboarding runs first (creates test data)

---

## Privacy & Security

- **GDPR compliant**: No IP storage, cookie-less tracking
- **CSRF protection**: Enabled via Cartridge middleware
- **Rate limiting**: Public API endpoints protected (70 req/min)
- **Auth rate limiting**: Login endpoints (10 req/min)
- **Sec-Fetch-Site**: Validates browser requests on event ingestion (rejects curl/scripts)
- **User signatures**: Hash-based visitor identification instead of cookies

---

Keep AGENTS.md synchronized when workflows change.
