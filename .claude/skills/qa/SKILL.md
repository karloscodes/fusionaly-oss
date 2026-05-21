---
name: qa
description: Use after code changes, before releases, or when testing features - runs the right level of QA based on what changed
---

# QA Testing

## What did you change? Start here.

| Change | Test level | Command |
|--------|-----------|---------|
| Bug fix, refactor, small feature | **Unit** | `make test` |
| Feature, UI, big change | **Unit + E2E** | `make test` then `make test-e2e` |
| Visual / UI verification | **Agent-browser** | `make dev` + agent-browser |
| Install script, matcha, infra | **VM install** | multipass VM |
| Release | **All of the above** | See release checklist |

---

## Level 1: Unit Tests (~3 seconds)

```bash
make test
```

Run after every change. No excuses.

---

## Level 2: E2E Tests (~5 minutes)

```bash
# CRITICAL: Kill any running dev server first — E2E uses its own database
lsof -ti :3000 | xargs kill -9 2>/dev/null
make test-e2e
```

Playwright tests that run onboarding, create websites, check dashboards, ingest events. Run after features or big changes.

---

## Level 3: Visual QA with agent-browser

For verifying UI rendering, checking that data displays correctly, or testing flows that E2E doesn't cover.

### Setup (one-time)
```bash
npm i -g agent-browser
agent-browser install
```

### Start dev server
```bash
make dev    # MUST use make dev (Go + Vite together)
```

### Dev credentials
- Email: `admin@example.com`
- Password: `password`
- Created by `make db-seed`

### Login
```bash
agent-browser open "http://localhost:3000/login"
agent-browser snapshot -i
agent-browser fill @e1 "admin@example.com"
agent-browser fill @e3 "password"
agent-browser click @e4
```

### Navigate and verify
```bash
agent-browser snapshot -i           # list interactive elements with refs
agent-browser click @e11            # click element by ref
agent-browser scroll down 1000      # scroll to find sections
agent-browser screenshot            # capture page as PNG
agent-browser get url               # check current URL
```

### Generate real browser events
```bash
# The /_demo page includes the tracking script — fires real pageviews
agent-browser open "http://localhost:3000/_demo"

# Wait for event processing (~10s), then check dashboard
```

### Key pages to verify

| Page | How to reach | What to check |
|------|-------------|---------------|
| Admin home | `/admin` | Website list loads |
| Dashboard | Click arrow (→) on a website | Charts, stats, time range |
| Browsers tab | Scroll to Device Analytics, click "Browsers" | Browser names (Brave, Edge, etc.) |
| Settings | Click "Settings" in nav | Forms save, flash messages |
| Events | Click "Events" tab on dashboard | Event list, sessions view |

### Important
- **Always `make dev`** — `make watch-go` alone won't render Inertia pages
- **Click through the UI** — don't force navigation with `open` after login (breaks Inertia state)
- **`/_demo` for real events** — sends actual `Sec-CH-UA` headers from the browser
- Dev database: `storage/fusionaly-development.db`

---

## Level 4: VM Install Test

Only needed when changing: install script, matcha, Docker setup, or release infrastructure.

### Create fresh VM
```bash
multipass delete fusionaly-test --purge 2>/dev/null || true
multipass launch 24.04 --name fusionaly-test --cpus 2 --memory 2G --disk 10G
```

### Run install
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

### Verify
```bash
multipass exec fusionaly-test -- bash -c '
echo "=== Containers ===" && sudo docker ps
echo "=== Version ===" && fusionaly version
echo "=== Health ===" && curl -s http://172.18.0.2:8080/_health
'
```

### Browser test via tunnel
```bash
VM_IP=$(multipass info fusionaly-test | grep IPv4 | awk '{print $2}')
ssh -L 8080:172.18.0.2:8080 ubuntu@$VM_IP
# Open http://localhost:8080/setup
```

### Cleanup
```bash
multipass delete fusionaly-test --purge
```

---

## Release Checklist

Before tagging a release:

- [ ] `make test` passes
- [ ] `make test-e2e` passes (kill dev server first!)
- [ ] Visual QA: dashboard loads, browser stats correct, events ingesting
- [ ] VM install test (if install/infra changed)
- [ ] Pro: update OSS submodule, build, test

---

## Common Issues

| Issue | Cause | Fix |
|-------|-------|-----|
| E2E fails "Setup already complete" | Dev server running (wrong DB) | `lsof -ti :3000 \| xargs kill -9` |
| agent-browser login doesn't work | Vite not running | Use `make dev`, not `make watch-go` |
| Dashboard shows JSON error | Navigated directly after forced POST | Start fresh browser, click through UI |
| `/_demo` events not appearing | Processing job hasn't run yet | Wait ~10 seconds, check `ingested_events` table |
| VM can't reach app | Docker internal network | Use SSH tunnel to 172.18.0.2:8080 |
