---
name: fusionaly-qa
description: Use when testing Fusionaly features, checking for regressions, or verifying releases - supports OSS and Pro, install scripts, E2E tests, and feature testing
---

# Fusionaly QA Testing

## Overview

Quality assurance for Fusionaly across different testing scenarios. Supports both OSS and Pro editions with multiple testing flavors.

## Testing Flavors

| Flavor | Purpose | When to Use |
|--------|---------|-------------|
| **full-install** | VM install → onboarding → verification | Before releases, after install script changes |
| **e2e** | E2E tests against local server | After code changes, before commits |
| **feature** | Test specific feature manually | Debugging, exploring behavior |
| **build-verify** | Build + quick health check | After dependency updates |

## Quick Reference

### E2E Tests (Most Common)

```bash
# OSS
cd /path/to/fusionaly-oss
lsof -ti :3000 | xargs kill -9 2>/dev/null  # Kill any running dev server first!
make test-e2e

# Pro
cd /path/to/fusionaly-pro
lsof -ti :3000 | xargs kill -9 2>/dev/null  # Kill any running dev server first!
make test-e2e
```

**CRITICAL**: E2E tests will fail if a dev server is running (uses different database).

### Feature Testing

```bash
# Start dev server
make dev

# In browser:
# OSS: http://localhost:3000
# Pro: http://localhost:3000 (includes AI features)
```

---

## Flavor 1: Full Install (VM)

Test the actual installation script on a fresh Ubuntu VM.

### 1. Create Fresh VM

```bash
multipass delete fusionaly-test --purge 2>/dev/null || true
multipass launch 24.04 --name fusionaly-test --cpus 2 --memory 2G --disk 10G
```

### 2. Run Install Script

```bash
multipass exec fusionaly-test -- bash -c '
sudo apt-get update -qq && sudo apt-get install -y -qq expect

cat > /tmp/run_install.exp << '\''EXPECTSCRIPT'\''
#!/usr/bin/expect -f
set timeout 300
spawn sudo bash -c "curl -fsSL https://fusionaly.com/install | bash"
expect "Enter your domain name"
send "test.local\r"
expect "Proceed with this configuration"
send "Y\r"
expect eof
EXPECTSCRIPT

expect /tmp/run_install.exp
'
```

### 3. Verify Installation

```bash
multipass exec fusionaly-test -- bash -c '
echo "=== Containers ===" && sudo docker ps
echo "=== Version ===" && fusionaly version
echo "=== Health ===" && curl -s http://172.18.0.2:8080/_health
'
```

### 4. Browser Testing (via SSH Tunnel)

```bash
VM_IP=$(multipass info fusionaly-test | grep IPv4 | awk '{print $2}')
ssh -L 8080:172.18.0.2:8080 ubuntu@$VM_IP
# Then open: http://localhost:8080/setup
```

### 5. Cleanup

```bash
multipass delete fusionaly-test --purge
```

---

## Flavor 2: E2E Tests

Run automated E2E tests against local application.

### OSS E2E

```bash
cd /path/to/fusionaly-oss
lsof -ti :3000 | xargs kill -9 2>/dev/null  # Important!
make test-e2e
```

**Tests include:**
- Onboarding flow (creates test user)
- Website creation
- Dashboard loading
- Event ingestion

### Pro E2E

```bash
cd /path/to/fusionaly-pro
lsof -ti :3000 | xargs kill -9 2>/dev/null  # Important!
make test-e2e
```

**Additional Pro tests:**
- AI settings page
- License page
- Lens interface
- Pro sidebar links
- AI API endpoints

---

## Flavor 3: Feature Testing

Manual testing of specific features.

### Start Dev Server

```bash
# OSS
cd /path/to/fusionaly-oss && make dev

# Pro
cd /path/to/fusionaly-pro && make dev
```

### Key Pages to Test

| Page | URL | What to Verify |
|------|-----|----------------|
| Onboarding | `/setup` | Form submission, validation, step progression |
| Dashboard | `/admin/websites/:id/dashboard` | Charts load, date picker works |
| Settings | `/admin/administration/ingestion` | Form saves, flash messages |
| **Pro: Lens** | `/admin/websites/:id/lens` | AI textarea, example questions |
| **Pro: AI Settings** | `/admin/administration/ai` | API key saves |
| **Pro: License** | `/admin/administration/license` | License key saves |

---

## Flavor 4: Build Verification

Quick sanity check that the build works.

```bash
# OSS
cd /path/to/fusionaly-oss
make build
./tmp/fusionaly --help

# Pro
cd /path/to/fusionaly-pro
make build
./tmp/fusionaly-pro --help
```

---

## Common Issues

| Issue | Cause | Fix |
|-------|-------|-----|
| E2E tests fail with "Setup already complete" | Dev server running (wrong database) | Kill process on port 3000 first |
| VM SSH connection refused | VM not ready | Wait for cloud-init: `multipass exec fusionaly-test -- cloud-init status --wait` |
| Can't reach 172.18.0.2 | Docker internal network | Use SSH tunnel |
| Tests timeout | Server slow to start | Increase timeout in playwright.config.js |

## Release Checklist

Before announcing a release:

- [ ] E2E tests pass (both OSS and Pro if applicable)
- [ ] Fresh VM install works
- [ ] Onboarding completes successfully
- [ ] Dashboard loads with data
- [ ] **Pro:** AI features work (with API key configured)
- [ ] **Pro:** License validation works
- [ ] Build produces working binaries
