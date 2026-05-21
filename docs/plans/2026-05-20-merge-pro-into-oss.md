# Merge Fusionaly Pro into OSS — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Discontinue the Pro/OSS split. Fold the activity feed and the full AI Lens into the OSS repo as native features, delete all licensing and the extension-hook architecture, leaving one self-hosted product.

**Architecture:** OSS absorbs Pro's `internal/feed`, `internal/ai`, the FeedJob, the OpenAI settings key, an optional onboarding step, and the home feed page — all wired the native OSS way (routes in `internal/routes.go`, jobs in the `jobs.Scheduler`, migrations in `database.MigrateDatabase`). Licensing (Polar/Gumroad), the `RequireLicense` middleware, and the `app/` + `web/src/extensions.ts` seam are deleted, not ported. AI features gate on "OpenAI key configured" instead of a license.

**Tech Stack:** Go 1.23 (Fiber, GORM, SQLite, cartridge) + React 19/TS/Tailwind/Inertia. OpenAI HTTP API for the Lens.

**Source repos:**
- Target: `/Users/karloscodes/Code/fusionaly/fusionaly-oss` (branch `merge-pro-into-oss`)
- Port from: `/Users/karloscodes/Code/fusionaly/fusionaly-pro` (NOT its `./oss` submodule)

---

## Decisions baked in (from the DHH/Jason review)

1. **No licensing, ever.** Do not port `internal/licensing`, `internal/polar`, `internal/gumroad`, `RequireLicense`, the license handlers, the license onboarding step, or the license admin page. Their tests (`licensing_test.go`, `polar_test.go`, `license_handler_test.go`) are dropped with them.
2. **AI gates on OpenAI key, not license.** If no `openai_api_key` setting is present, the Lens renders an "add your key" state. No paywall.
3. **Settings reuse.** Add `openai_api_key` to the existing OSS settings KV (`internal/settings`). Do not port the `pro_settings` table. Drop `license_key` entirely.
4. **Feed is the home.** The merged `/admin` home = "Your sites" + "What's new" feed + conversion calendar (ProHome), replacing the bare Websites list as the landing.
5. **Onboarding 3 → 4 steps.** Add an optional OpenAI-key step after GeoLite. No license step.
6. **Tune feed signal thresholds** before the feed becomes the home (Phase 3, explicit task).
7. **Delete the seam last** so earlier phases keep building.
8. **Retire the boundaries doc.** The `fusionaly-boundaries` skill and the "Pro Extension Points" section of `CLAUDE.md` describe an architecture we're removing — update them in Phase 8 or future-us will be misled.

**Pre-release gate (NOT a code task):** the email to existing Pro license holders must go out before this is released. Branch work is safe; do not run `make release` until Carlos confirms the email is sent.

---

## Test strategy

- **Bring:** `internal/feed/detector_test.go`, `internal/feed/spc_test.go` (port alongside the feed code).
- **Write new:** the AI Lens has **zero** tests in Pro. As we port it (Phase 4), write tests for the SQL read-only validator, the cache key/TTL, and saved-query CRUD before wiring handlers.
- **E2E:** add Playwright specs for the home feed render and the Lens "no key" → "ask" flow (Phase 8). Pro had no e2e specs.
- **Gate each phase** on `make test` (~3s). Run `make test-e2e` (~5m) at the ends of Phase 3, 5, and 8.

---

## Phase 0: Baseline safety net

### Task 0.1: Confirm green baseline
- **Step 1:** Run `make test`. Expected: PASS. (If red on a clean `main` branch, stop and fix before merging anything.)
- **Step 2:** Run `make build`. Expected: builds clean.
- **Step 3:** Commit nothing; this is a checkpoint. Record the passing test count in the phase notes.

---

## Phase 1: OpenAI key setting (foundation for AI)

**Files:**
- Modify: `internal/settings/setting.go`
- Test: `internal/settings/setting_test.go` (create if absent)

**Step 1: Failing test** — add `TestOpenAIKeySetting` in `internal/settings/setting_test.go`:
```go
func TestOpenAIKeySetting(t *testing.T) {
    db := setupTestDB(t)

    err := SaveOpenAIKey(db, "  sk-test123  ")

    require.NoError(t, err)
    got, err := GetOpenAIKey(db)
    require.NoError(t, err)
    assert.Equal(t, "sk-test123", got) // trimmed
}
```
**Step 2:** `make test t="TestOpenAIKeySetting"` → FAIL (undefined SaveOpenAIKey).
**Step 3:** Implement in `setting.go`:
```go
const KeyOpenAIKey = "openai_api_key"

func SaveOpenAIKey(db *gorm.DB, key string) error {
    return CreateOrUpdateSetting(db, KeyOpenAIKey, strings.TrimSpace(key))
}
func GetOpenAIKey(db *gorm.DB) (string, error) {
    return GetSetting(db, KeyOpenAIKey)
}
```
Add `{Key: KeyOpenAIKey, Value: ""}` to `SetupDefaultSettings`.
**Step 4:** `make test t="TestOpenAIKeySetting"` → PASS.
**Step 5:** Commit `feat: add openai_api_key setting`.

---

## Phase 2: Feed backend + job

**Files (port from Pro, fix package paths, remove all `licensing.*` calls):**
- Create: `internal/feed/feed.go`, `baseline.go`, `detector.go`, `spc.go`
- Create (port tests): `internal/feed/detector_test.go`, `internal/feed/spc_test.go`
- Create: `internal/jobs/feed.go` (adapt Pro's `feed_job.go` to the OSS `Scheduler` `Run() error` pattern)
- Modify: `internal/database/database.go` (AutoMigrate `&feed.FeedItem{}`, `&feed.Baseline{}`)
- Modify: `internal/jobs/scheduler.go` (register FeedJob)

### Task 2.1: Port feed domain + tests
- **Step 1:** Copy the four feed source files + two test files. Rewrite imports (`fusionaly/internal/...`). **Delete** the `licensing.IsLicenseValid` guard inside detection — feed always runs now.
- **Step 2:** `make test t="TestDetector|TestSPC|TestSpc"` → expect PASS once imports resolve.
- **Step 3:** Add `feed.FeedItem` and `feed.Baseline` to `MigrateDatabase`'s `AutoMigrate` list.
- **Step 4:** `make test` → PASS.
- **Step 5:** Commit `feat: port activity feed detection from pro`.

### Task 2.2: Register FeedJob in the scheduler
- **Step 1:** Add `internal/jobs/feed.go` with `FeedJob{dbManager, logger}` and `Run() error` that calls `feed.DetectAll`, `feed.LearnFromYesterday`, `feed.CleanupOldItems`, `feed.CleanupStaleBaselines`. (Pro started its own ticker; OSS owns the schedule — drop Pro's internal ticker.)
- **Step 2:** In `scheduler.go`: add `feedJob` field + `feedTicker`, init in `NewScheduler`, add `startFeedJob()` using `executeJobSafely("feed", s.feedJob.Run)` on `JobIntervalSeconds`, stop the ticker in `Stop()`.
- **Step 3:** `make test` → PASS. `make build` → clean.
- **Step 4:** Commit `feat: schedule feed detection job`.

---

## Phase 3: Feed home page (the new landing)

**Files:**
- Modify: `internal/http/` — add `HomeFeedAction` (port `ProHomeAction` from Pro's `feed_handler.go`; sites + event counts + `feed.GetUserFeed` + conversion calendar).
- Modify: `internal/routes.go` — point `/admin` at `HomeFeedAction` (replacing the Websites-list landing), keep `/admin/websites` reachable.
- Create: `web/src/pages/Home.tsx` content from Pro's `ProHome.tsx` (sites cards, "What's new" grouped by date, site filter, conversion heatmap). Reuse OSS shadcn components.
- Modify: Inertia page name → render `"Home"` (or keep `"ProHome"` renamed to `"Home"`).

### Task 3.1: Backend home feed
- TDD a handler test if feasible (`internal/http/...test.go`): seed sites + feed items, assert props include `feedItems`, `websites`, `calendarData`. Otherwise cover via e2e in Phase 8.
- Commit `feat: home feed handler`.

### Task 3.2: Frontend home feed
- Port `ProHome.tsx` → `Home.tsx`, drop any Pro-only imports, wire to the new props.
- Commit `feat: feed home page`.

### Task 3.3: **Tune feed signal thresholds**
- Review `spc.go` / `detector.go` thresholds (spike sigma, min sample count, "new referrer" floor, milestone steps) against real seeded data: `make db-seed` then inspect generated `feed_items`.
- Goal: quiet-until-real. Adjust constants; the feed must not fire on noise for a small site.
- Add/extend a detector test asserting no items fire below threshold.
- Commit `tune: feed signal thresholds for low-noise`.
- **Run `make test-e2e`** at end of phase.

---

## Phase 4: AI Lens backend

**Files (port from Pro `internal/ai/`, fix paths):**
- Create: `internal/ai/ai.go` (+ OpenAI client, `SavedQuery`, `AIQueryCache`, read-only SQL validator, single-query + investigation generation).
- Create (NEW — Pro had none): `internal/ai/ai_test.go`.
- Modify: `internal/database/database.go` (AutoMigrate `&ai.SavedQuery{}`, `&ai.AIQueryCache{}`).
- Create: `internal/http/ai_handler.go` (ask, investigate, save, get saved, delete, status). Gate on `settings.GetOpenAIKey` present, **not** license.

### Task 4.1: Read-only SQL validator (TDD)
- **Step 1:** `TestValidateReadOnlyQuery` — rejects `INSERT/UPDATE/DELETE/DROP/ATTACH/PRAGMA write`, accepts `SELECT`.
- **Step 2:** FAIL. **Step 3:** Port the validator. **Step 4:** PASS. **Step 5:** Commit.

### Task 4.2: Query cache key + TTL (TDD)
- Test cache key determinism (same question+site+model → same key) and 6h expiry. Port `AIQueryCache`. Commit.

### Task 4.3: Saved-query CRUD (TDD)
- Test create/list-by-site/delete/reorder. Port `SavedQuery`. Commit.

### Task 4.4: OpenAI client + handlers
- Port the OpenAI call (keep BYO-key, JSON mode, retry-on-validation-failure). **Keep model selection** (full Lens per decision). Wire handlers; gate on key presence with a clean 400/empty-state contract. `make test` → PASS. Commit `feat: ai lens backend`.

---

## Phase 5: AI Lens frontend

**Files:**
- Replace: `web/src/pages/Lens.tsx` (the OSS placeholder) with the real Lens ported from Pro's `ProLens.tsx`.
- Create: `web/src/components/VegaChart.tsx`, `AskAIChat.tsx`, `ModelSelector.tsx`, `InsightsCard.tsx` (port).
- Modify: `internal/routes.go` — register lens + AI API routes (no license middleware; session + website-filter only).
- Modify: dashboard — port the Pro insights card injection directly into the OSS `Dashboard` component (the old `insightsSlot`), fed by an AI insights call when a key exists.

### Task 5.1: Lens page + components
- Port, wire to Inertia props + the AI API routes. When no key: render the "add your OpenAI key" state linking to onboarding/settings. Commit `feat: ai lens page`.

### Task 5.2: Dashboard insights (inline the old extender)
- Render insights card directly in `web/src/components/dashboard` (remove the `insightsSlot` indirection), populated server-side when a key is set. Commit `feat: inline dashboard insights`.
- **Run `make test-e2e`** at end of phase.

---

## Phase 6: Onboarding — optional OpenAI step

**Files:**
- Modify: `internal/onboarding/onboarding.go` (`StepOpenAI` constant, `OnboardingData.OpenAIKey`), `completion.go` (`CompletionData.OpenAIKey` → `settings.SaveOpenAIKey`).
- Modify: `internal/http/onboarding_handler.go` (geolite step now advances to `StepOpenAI`; add `OnboardingOpenAIFormAction` with skip support → then complete).
- Modify: `internal/routes.go` (`POST /setup/openai`).
- Modify: `web/src/pages/Onboarding.tsx` (render the optional OpenAI step with a "Skip" action).

### Task 6.1: Step state machine (TDD)
- Test: geolite submit → session step becomes `openai`; openai submit (or skip) → completed; saved key lands in settings. Implement. Commit `feat: optional openai onboarding step`.

---

## Phase 7: Delete the seam

**Files:**
- Delete: `app/extensions.go`, `web/src/extensions.ts`.
- Modify: `internal/http/dashboard_handler.go` — collapse `WebsiteDashboardActionWithExtension` + `DashboardPropsExtender` into the plain `WebsiteDashboardAction` (insights wired inline from Phase 5).
- Modify: `internal/http/` lens handler — collapse `WebsiteLensActionWithComponent` into a single `LensAction` rendering `"Lens"`.
- Modify: `internal/routes.go` — remove `MountAppRoutesWithoutSession`/custom-mounter split; one `MountAppRoutes`.
- Modify: `cmd/fusionaly/main.go` — confirm single entry point (it already is; remove any Pro-shaped seams).

### Task 7.1: Inline + delete
- Remove the exports, fix all references (compiler will list them), delete the two files. `make build` → clean, `make test` → PASS. Commit `refactor: remove pro extension seam`.

---

## Phase 8: Migrate existing Pro installations to free

Existing Pro customers run the `karloscodes/fusionaly-pro:latest` Docker image; OSS is `karloscodes/fusionaly:latest`. `matcha` cron-auto-updates whatever image is pinned in their `.env`. Three things must happen so a running Pro install becomes a running merged-OSS install with no data loss.

### Task 8.1: One-time settings data migration (the only real DB gap)
- **Schema parity (verify, don't migrate):** the four ported tables are byte-compatible because we port the structs verbatim — `feed_items`, `feed_baselines` (explicit `TableName()`), `saved_queries`, `ai_query_caches` (GORM-inferred from `SavedQuery`/`AIQueryCache` — keep the struct names identical so the inferred names match). GORM AutoMigrate is additive-only, so an existing Pro DB needs no schema change. **Keep the struct names and `TableName()` returns identical to Pro.**
- **The gap:** Pro stores the OpenAI key in the `pro_settings` table (`key='openai_api_key'`). Merged OSS reads it from the `settings` table. The `license_key` row is abandoned (harmless).
- **Step 1: Failing test** — `internal/database/migrate_pro_settings_test.go`: seed a DB with a `pro_settings` row `openai_api_key='sk-x'` and empty OSS `settings`; call the migration; assert `settings.GetOpenAIKey == 'sk-x'`. Add a second case: when OSS `settings.openai_api_key` is already set, do NOT overwrite.
- **Step 2:** FAIL. **Step 3:** implement `migrateProSettings(db)` — if a `pro_settings` table exists and has `openai_api_key`, and OSS `settings.openai_api_key` is empty, copy it. Idempotent. Call it from `MigrateDatabase` after AutoMigrate. Leave `pro_settings` in place (harmless orphan; do not drop — a dropped table can't be rolled back).
- **Step 4:** PASS. **Step 5:** Commit `feat: migrate openai key from pro_settings on upgrade`.

### Task 8.2: Remove the OSS→Pro upgrade path
- `cmd/manager/main.go`: delete the `case "upgrade"`, `runUpgrade`, the `proImage` const, and the usage line. There is no Pro to upgrade to.
- The `Lens.tsx` "Upgrade to Pro" CTA is already replaced by the real Lens in Phase 5 — confirm no other "Upgrade to Pro" / `fusionaly.com/#pricing` strings remain (`grep -rin "upgrade to pro" web/src cmd internal`).
- Commit `chore: remove oss→pro upgrade command`.

### Task 8.3: Image transition strategy (decision for Carlos, then execute)
- **Primary (zero customer action):** for a deprecation window, publish the merged OSS build to BOTH `karloscodes/fusionaly:latest` and `karloscodes/fusionaly-pro:latest`. Pro installs' cron auto-update pulls it, AutoMigrate + Task 8.1 run, and they're on the merged product with no SSH. Update `.goreleaser`/CI to dual-push the Pro tag during the window.
  - Caveat: the manager binary on Pro installs updates from the Pro manager repo (`ManagerRepo`). Verify whether Pro's manager needs a matching release, or whether the manager is unchanged enough to leave alone.
- **Follow-up (move them to the canonical tag):** add a `manager` command `migrate-to-oss` = `runUpgrade` reversed — backs up DB, `m.SetImage("karloscodes/fusionaly:latest")`, `SaveImage()`, `Deploy()`. Ship it so customers (or the email) can move off the `-pro` tag onto the canonical OSS tag at leisure. Then retire the `-pro` tag.
- Commit the chosen mechanism. Document the window length in the email.

### Task 8.4: Upgrade-path test
- Boot the OSS binary against a Pro-shaped DB fixture (feed_items + saved_queries + pro_settings populated). Assert: app starts, AutoMigrate is clean, OpenAI key carried over, feed + saved queries intact. Cover via an integration test or a scripted `make` target. Commit.

---

## Phase 9: Cleanup, docs, full QA

### Task 9.1: E2E specs
- Add `e2e/` specs: home feed renders sites + feed; Lens shows no-key state then accepts a question (mock or skip live OpenAI). Commit.

### Task 9.2: Docs + skill retirement
- Update `CLAUDE.md`: remove the "Pro Extension Points" section; note the product is now single-edition with feed + AI.
- Flag the `fusionaly-boundaries` skill for retirement/rewrite (it forbids exactly what we just did). Tell Carlos — it lives in dotfiles, not this repo.
- Update `README.md` positioning: feed + AI are now core, self-hosted, BYO OpenAI key. Commit `docs: single-edition product`.

### Task 9.3: Full QA gate
- `make test` → PASS. `make test-e2e` → PASS. `make lint` → clean. `make build` → clean.
- Use the `/qa` skill for the visual/VM pass.

### Task 9.4: Finish
- Per superpowers:finishing-a-development-branch — open PR or merge. **Do not release** until the Pro-customer email is confirmed sent.

---

## What is explicitly NOT ported (deleted with Pro)

`internal/licensing/`, `internal/polar/`, `internal/gumroad/`, `RequireLicense` middleware, `license_handler.go` (+ test), `licensing_test.go`, `polar_test.go`, the license onboarding step, the license admin page, the `pro_settings` table (we keep only the OpenAI key, in OSS settings). The Pro `cmd/fusionaly-pro` entry point and `ProApplication` wrapper are not needed — OSS's `cmd/fusionaly` is the single entry point.
