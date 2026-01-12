# Developer Agent Guidelines

This is the **canonical source** for all AI coding assistants (Claude, Copilot, Codex) working on Fusionaly.

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
│   ├── routes.go        # Route registration
│   └── server.go        # Server configuration
│
├── analytics/           # Context: Analytics domain
├── events/              # Context: Events domain
├── websites/            # Context: Websites domain
├── users/               # Context: Users domain
├── visitors/            # Context: Visitors domain
├── onboarding/          # Context: Onboarding domain
├── settings/            # Context: Settings domain
├── queries/             # Context: Saved queries
├── insights/            # Context: AI insights
├── timeframe/           # Context: Time ranges
└── aggregates/          # Context: Aggregations
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
func DashboardAction(c *fiber.Ctx, db *gorm.DB, logger *zap.Logger, cfg *config.Config) error {
    websiteID := c.QueryInt("website_id", 0)
    timeframe := timeframe.ParseTimeframe(c.Query("range"))
    
    metrics := analytics.GetDashboardMetrics(db, websiteID, timeframe)
    return view.RenderSuccess(c, "Dashboard", map[string]interface{}{
        "metrics": metrics,
    })
}

// ❌ BAD: Business logic in handler
func DashboardAction(c *fiber.Ctx, db *gorm.DB, logger *zap.Logger, cfg *config.Config) error {
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
make release    # Tests + Build + Push + Deploy
```

---

## Project Snapshot

- **Stack**: Go 1.23+ (Fiber, GORM, SQLite) + React 19/TypeScript/Tailwind
- **Architecture**: Phoenix Contexts pattern  
- **Domain**: Privacy-first analytics
- **Database**: SQLite with WAL mode
- **Binaries**: `cmd/fusionaly` (server) + `cmd/fnctl` (CLI/migrations)

### Repo Landmarks

- **`internal/http/`**: HTTP handlers, middleware, routes
- **`internal/{domain}/`**: Domain contexts (analytics, events, users, etc.)
- **`api/v1/`**: Public ingestion + SDK endpoints
- **`web/`**: React SPA (shadcn/ui, Tailwind, TanStack Query)
- **`tests/` and `e2e/`**: Go tests and Playwright suites
- **`cmd/`**: Main binaries

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
make release      # Tests + Build + Docker + Deploy
```

---

## UI Architecture

Hybrid architecture optimized for solo maintainability:

1. **ALL UI**: React components + Tailwind (shadcn/ui)
   - Every page is a React component
   - Consistent component library

2. **INITIAL DATA**: window.__PAGE_DATA__
   - Server renders HTML shell once
   - React hydrates from embedded data
   - No client-side data fetching on page load

3. **FORMS**: PRG Pattern (POST → Redirect → GET)
   - Settings, Websites, Account, Login
   - Flash messages for feedback
   - Works without JavaScript
   - Use `view.RenderSuccess()` for rendering

4. **NAVIGATION**: Custom Link Component
   - Smooth transitions without full reload
   - Fetches new page via AJAX  
   - React remounts with new data
   - Use `<Link>` from `@/components/link`

5. **JSON APIs**: ONLY for complex features
   - Ask AI: Streaming responses (SSE/NDJSON)
   - Lens: Query CRUD and execution
   - Dashboard Insights: Async data loading
   - Use `c.JSON()` for these endpoints

**When adding features:**
- Admin pages → Use `view.RenderSuccess()` with `__PAGE_DATA__`
- Forms → Use PRG pattern with flash messages
- Complex interactions → JSON API only if truly needed

---

## Strong Requirements

- **Convention over configuration:** Minimize setup complexity
- **window.__PAGE_DATA__:** Server-side data hydration for React
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
- **CSRF protection**: Enabled for admin routes
- **Rate limiting**: Public API endpoints protected
- **User signatures**: Instead of identifiable data

---

Keep AGENTS.md synchronized when workflows change.
