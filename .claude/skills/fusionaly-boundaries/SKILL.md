---
name: fusionaly-boundaries
description: Use when working on Fusionaly OSS or Pro - enforces clean separation between repos, prevents Pro features in OSS, forbids modifying OSS from Pro
---

# Fusionaly Boundaries

## Overview

Fusionaly has two repos with strict boundaries. OSS is the open-source core. Pro extends OSS via git submodule without modifying it.

## Repository Structure

```
fusionaly-oss/              # Standalone open-source product
├── app/                    # Public API (Pro imports from here)
├── internal/               # OSS internals (Pro cannot import)
└── web/src/                # OSS React components

fusionaly-pro/              # Commercial extension
├── oss/                    # Git submodule → fusionaly-oss (READ-ONLY!)
├── internal/               # Pro-only code
└── web/src/                # Pro React components
```

## The Rules

### When Working in OSS

| ✅ Do | ❌ Don't |
|-------|---------|
| Keep it fully standalone | Add AI/LLM features |
| Export via `app/` package | Add license validation |
| Provide extension points | Reference Pro repo |
| Test with `make test-e2e` | Add "Pro" conditionals |

**OSS must work without Pro.** No AI, no licensing, no Pro feature flags.

### When Working in Pro

| ✅ Do | ❌ Don't |
|-------|---------|
| Import from `fusionaly/app` | Import from `fusionaly/internal/*` |
| Override routes (Pro first) | Modify files in `./oss/` |
| Extend via props/hooks | Add functions to OSS "for Pro" |
| Test with `make test-all` | Commit OSS changes from Pro |

**Pro extends OSS via public API only.** Never edit the submodule directly.

## Import Rules (Pro)

```go
// ✅ GOOD
import "fusionaly/app"

// ❌ FORBIDDEN
import "fusionaly/internal/analytics"
```

## Red Flags - STOP

**In OSS, stop if you're adding:**
- OpenAI or LLM integration
- License key handling
- AI insights or Lens features
- "Pro" in any name

**In Pro, stop if you're:**
- Editing any file in `./oss/`
- Importing from `fusionaly/internal/*`
- Adding exports to OSS for Pro's benefit

## Extension Pattern

OSS provides hooks, Pro uses them:

```go
// OSS: Provides extension point
func HandleDashboardWithExtension(ctx, component string, extender func(props) props) error {
    props := buildProps(ctx)
    if extender != nil {
        props = extender(props)  // Pro injects here
    }
    return render(component, props)
}

// Pro: Uses extension point
func proDashboardExtender(props map[string]any) map[string]any {
    props["insights"] = ai.GetInsights()  // Pro-only data
    return props
}
```

## If OSS Needs Changes

1. Work in fusionaly-oss repo directly (not the submodule)
2. Make changes, test: `make test && make test-e2e`
3. Commit and push to OSS
4. In Pro: `make update-oss` to pull changes

## Quick Reference

| Task | Location | Command |
|------|----------|---------|
| Add OSS feature | fusionaly-oss | `make test` |
| Add Pro feature | fusionaly-pro/internal | `make test` |
| Extend OSS behavior | Pro via `fusionaly/app` | `make test-all` |
| Update OSS in Pro | fusionaly-pro | `make update-oss` |
