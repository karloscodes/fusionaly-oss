---
name: fusionaly-deploy
description: Use ONLY when explicitly invoked. Deploys Fusionaly to a new Hetzner server or an existing server, with optional hardening and Cloudflare DNS.
---

# Deploy Fusionaly

Deploy Fusionaly to a new Hetzner server or an existing server you already have. Optionally harden the server and configure Cloudflare DNS.

**Principle:** Scan first, report status, ask before every invasive action.

## Flow

1. Scan environment (silent)
2. Show status report
3. Fix missing tools (ask for each)
4. Choose: new Hetzner server or existing server
5. Gather details (domain, location, SSH key)
6. Cloudflare DNS? (optional)
7. Confirm plan
8. Create server (Hetzner only)
9. Verify install URLs are reachable
10. Harden server (ask first)
11. Install Fusionaly (ask first)
12. Verify install worked
13. Create DNS record (ask first)
14. Summary + reminders

## Phase 1: Preflight

### Step 1 — Scan Environment (silent, no questions)

Run all checks, collect results:

```bash
# hcloud CLI
which hcloud && hcloud version
hcloud context active 2>/dev/null

# flarectl
which flarectl

# Local SSH keys
ls ~/.ssh/*.pub 2>/dev/null

# Hetzner SSH keys (only if hcloud configured)
hcloud ssh-key list 2>/dev/null
```

### Step 2 — Present Status Report

Show everything at once so the user sees the full picture:

```
Environment check:
  hcloud CLI:       ✓ installed (v1.x.x) / ✗ not installed
  Hetzner token:    ✓ configured (context: xxx) / ✗ not configured
  flarectl:         ✓ installed / ✗ not installed
  Local SSH keys:   ✓ N keys found / ✗ none found
  Hetzner SSH keys: ✓ N keys / — (can't check without token)
```

### Step 3 — Fix Missing Tools (ask for each)

For each missing item, ask whether to install/configure. One at a time.

**hcloud CLI** (only needed for new server creation):
```bash
brew install hcloud
```

**Hetzner API token** (only needed for new server creation). Walk the user through:

```
You need a Hetzner Cloud API token with read/write permissions.

  1. Go to https://console.hetzner.cloud
  2. Select your project (or create one)
  3. Go to Security → API Tokens
  4. Click "Generate API Token"
  5. Name it (e.g. "fusionaly-deploy"), select Read & Write
  6. Copy the token (you won't see it again)
```

Then store it:
```bash
hcloud context create fusionaly --token <token-user-provided>
# Verify it works
hcloud server-type list > /dev/null && echo "Hetzner token works"
```

**SSH keys** — if none exist:
```bash
ssh-keygen -t ed25519
```

**flarectl** — handled later in Cloudflare section if user opts in.

### Step 3b — Choose Mode

```
Do you want to:
  1. Create a new server on Hetzner
  2. Use an existing server (any provider)
```

## Phase 2: Gather Choices

Ask one question at a time. Wait for answer before asking next.

### Path A: New Hetzner Server

**If user declined hcloud install in step 3:** explain it's required for this path and stop gracefully.

#### Domain

```
What domain will Fusionaly run on? (e.g. analytics.example.com)
```

#### Server Location

Fetch from `hcloud location list` and present the list. Let user pick.

#### Server Type

Fetch from `hcloud server-type list`, filter to shared CPU types (cx/cax), and present with specs and prices. Let user pick.

Fusionaly minimum: 1 vCPU, 512MB RAM, 10GB SSD.

#### SSH Key

```bash
hcloud ssh-key list
```

If keys exist in Hetzner: let user pick from list.
If no Hetzner keys but local keys exist: offer to upload one.

Upload with: `hcloud ssh-key create --name <name> --public-key-from-file <path>`

### Path B: Existing Server

Ask:
1. **Server IP:** "What's your server's IP address?"
2. **SSH user:** "What SSH user should I use? (e.g. root, ubuntu, admin)"
3. **SSH port:** "What SSH port? (default: 22)"
4. **Domain:** "What domain will Fusionaly run on?"

Verify SSH connectivity:
```bash
ssh -o ConnectTimeout=5 -p <port> <user>@<ip> echo ok
```

If it fails, help troubleshoot (wrong key, port, user).

### Cloudflare (optional, both paths)

```
Do you use Cloudflare for DNS on this domain? (y/n)
```

If yes:

**1. Install flarectl** (if not already installed):
```bash
brew install cloudflare/cloudflare/flarectl
```

**2. Get a Cloudflare API token.** Walk the user through this:

```
You need a Cloudflare API token with DNS edit permissions.

  1. Go to https://dash.cloudflare.com/profile/api-tokens
  2. Click "Create Token"
  3. Use the "Edit zone DNS" template
  4. Under Zone Resources: select the zone for your domain
  5. Click "Continue to summary" → "Create Token"
  6. Copy the token (you won't see it again)
```

**3. Store and verify the token.** Ask the user to paste it, then:

```bash
# Export for this session
export CF_API_TOKEN="<token-user-provided>"

# Verify it works — this MUST succeed before proceeding
flarectl zone list
```

If `flarectl zone list` fails (401, no zones, etc.) — stop and troubleshoot. Do NOT continue to DNS setup with a broken token.

## Phase 3: Confirm & Create

### Confirmation

Show full plan. Adapt based on path:

**New Hetzner server:**
```
Ready to provision:
  Server:     <type> (<specs>)
  Location:   <location-name> (<location-id>)
  Image:      Ubuntu 24.04
  SSH key:    <key-name>
  Domain:     <domain>
  Cloudflare: yes / no

Proceed? This will create a billable server.
```

**Existing server:**
```
Ready to deploy:
  Server:     <ip> (via <user>@<ip>:<port>)
  Domain:     <domain>
  Cloudflare: yes / no

Proceed?
```

**Only proceed on explicit yes.**

### Create Server (Hetzner path only)

Sanitize domain for server name: replace dots with dashes, lowercase (e.g. `analytics.example.com` → `fusionaly-analytics-example-com`).

```bash
hcloud server create \
  --name fusionaly-<sanitized-domain> \
  --type <type> \
  --location <location> \
  --image ubuntu-24.04 \
  --ssh-key <key-name>
```

Capture the public IP from output. Wait for SSH to become available:

```bash
# Poll until SSH is ready (max ~60 seconds)
ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no root@<ip> echo ok
```

Tell the user: "Server created at `<ip>`. SSH is ready."

## Phase 4: Server Setup

### Verify Install URLs Are Reachable

Before telling the user to SSH in, check the URLs work:

```bash
curl --head --silent --fail https://raw.githubusercontent.com/karloscodes/server-hardener/main/harden.sh > /dev/null && echo "server-hardener: reachable" || echo "server-hardener: UNREACHABLE"
curl --head --silent --fail https://fusionaly.com/install > /dev/null && echo "fusionaly installer: reachable" || echo "fusionaly installer: UNREACHABLE"
```

If either URL is unreachable, tell the user before proceeding.

### Harden Server (ask first)

```
Should I run the server-hardener? This will:
  - Create admin user with SSH key
  - Disable root login and password auth
  - Configure firewall (80/443 open)
  - Option for Tailscale (locks SSH to Tailscale only)
  - Enable unattended security upgrades
```

If yes, the hardener is interactive — the user must run it themselves:

```
The server-hardener is interactive (4 questions). Run this in your terminal:

  ssh root@<ip> 'curl -fsSL https://raw.githubusercontent.com/karloscodes/server-hardener/main/harden.sh -o /tmp/harden.sh && sudo bash /tmp/harden.sh'
```

**Wait for user to confirm hardening is complete before proceeding.**

After hardening, ask what SSH access method they now have:
- Tailscale mode: `ssh admin@<tailscale-ip>`
- No Tailscale: `ssh -p 2222 admin@<ip>`

### Install Fusionaly (ask first)

```
Should I install Fusionaly now? This will:
  - Install Docker
  - Set up Caddy reverse proxy with SSL
  - Configure automatic backups and updates
```

If yes, the installer is interactive — the user must run it themselves:

```
The Fusionaly installer is interactive (asks for domain). Run this in your terminal:

  ssh <user>@<host> 'curl -fsSL https://fusionaly.com/install | sudo bash'
```

Use the SSH access from the hardening step (admin user, correct port).

**Wait for user to confirm installation is complete.**

### Verify Install Worked

After the user confirms installation is complete, verify it's actually running:

```bash
# Wait a moment for SSL to provision, then check
curl --silent --max-time 10 -o /dev/null -w "%{http_code}" https://<domain>
```

If you get a 200 or 302: install confirmed.
If connection refused or timeout: SSL may still be provisioning (Let's Encrypt needs DNS to resolve first). Tell the user to wait a few minutes and check `https://<domain>` in their browser.

### Cloudflare DNS (if opted in, ask first)

```
Should I create the A record now?
  <domain> → <server-ip>
```

If yes — `CF_API_TOKEN` should already be exported from the Cloudflare setup step earlier:

```bash
# CF_API_TOKEN was exported during Cloudflare setup in Phase 2
flarectl dns create \
  --zone <base-domain> \
  --name <subdomain> \
  --type A \
  --content <server-ip>
```

Where `<base-domain>` is extracted from the domain (e.g. `example.com` from `analytics.example.com`) and `<subdomain>` is the prefix (e.g. `analytics`).

If `CF_API_TOKEN` is not set (e.g. new shell session), re-export it:
```bash
export CF_API_TOKEN="<token-from-earlier>"
```

## Phase 5: Summary

Adapt based on what was actually done. Only show completed steps.

**New Hetzner server:**
```
Done! Here's what was set up:

  Server:      fusionaly-<domain> (<ip>)
  Location:    <location-name> (<location-id>)
  Type:        <type> (<specs>)
  OS:          Ubuntu 24.04
  Hardened:    ✓ / ✗ skipped
  Tailscale:   ✓ / ✗ / — (skipped hardening)
  Fusionaly:   ✓ installed at <domain> / ✗ skipped
  DNS:         ✓ A record via Cloudflare / manual setup needed
  SSH access:  ssh admin@<tailscale-ip> / ssh -p 2222 admin@<ip>
  Dashboard:   https://<domain>/admin
```

**Existing server:**
```
Done! Here's what was set up:

  Server:      <ip> (via <user>@<ip>:<port>)
  Hardened:    ✓ / ✗ skipped
  Fusionaly:   ✓ installed at <domain> / ✗ skipped
  DNS:         ✓ A record via Cloudflare / manual setup needed
  SSH access:  <the access method from hardening>
  Dashboard:   https://<domain>/admin
```

### Conditional Reminders

Only show what's relevant:

- **If no Cloudflare:** "Create an A record pointing `<domain>` to `<ip>`. SSL via Let's Encrypt won't activate until DNS propagates."
- **If Tailscale chosen:** "Approve this server in your Tailscale admin console (https://login.tailscale.com/admin)."
- **If hardening skipped:** "Your server is NOT hardened. Root SSH is still open. Strongly recommend running the server-hardener."
- **If Fusionaly skipped:** "Fusionaly is not installed yet. SSH in and run the installer when ready."
- **Always (if Fusionaly installed):** "Auto-updates run daily at 3 AM. Backups stored at `/opt/fusionaly/storage/backups/`."
- **Always (if Fusionaly installed):** "Consider off-server backups: VPS snapshots, rsync, or Litestream to S3."
- **If Cloudflare was used:** "Your Cloudflare token is set for this session only. To persist it, add to `~/.zshrc`: `export CF_API_TOKEN=\"your-token\"`"
