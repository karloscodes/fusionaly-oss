<h1 align="center">Fusionaly</h1>

<p align="center"><strong>Self-hosted web analytics in one SQLite file.</strong></p>

<p align="center">
  See who visits, where they come from, and what they click.<br>
  No cookies, no fingerprinting, no third parties. Your data never leaves your server.
</p>

<p align="center">
  <a href="https://github.com/karloscodes/fusionaly-oss/releases/latest"><img src="https://img.shields.io/github/v/release/karloscodes/fusionaly-oss?color=00D1FF" alt="Latest release"></a>
  <a href="https://hub.docker.com/r/karloscodes/fusionaly"><img src="https://img.shields.io/docker/pulls/karloscodes/fusionaly?color=00D1FF" alt="Docker pulls"></a>
  <a href="https://github.com/karloscodes/fusionaly-oss/actions"><img src="https://github.com/karloscodes/fusionaly-oss/actions/workflows/pr.yml/badge.svg" alt="CI"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/github/license/karloscodes/fusionaly-oss?color=00D678" alt="License: MIT"></a>
  <a href="https://github.com/karloscodes/fusionaly-oss/stargazers"><img src="https://img.shields.io/github/stars/karloscodes/fusionaly-oss?style=flat&color=00D678" alt="GitHub stars"></a>
  <br>
  <img src="https://img.shields.io/github/go-mod/go-version/karloscodes/fusionaly-oss" alt="Go version">
  <img src="https://img.shields.io/badge/database-SQLite-003B57?logo=sqlite&logoColor=white" alt="SQLite">
  <img src="https://img.shields.io/badge/self--hosted-one%20container-000000" alt="Self-hosted in one container">
  <a href="#ask-from-your-ai-client"><img src="https://img.shields.io/badge/MCP-Claude%20%C2%B7%20Codex%20%C2%B7%20Cursor%20%C2%B7%20Gemini-00D1FF" alt="MCP: Claude, Codex, Cursor, Gemini"></a>
</p>

<p align="center">
  <a href="https://t.fusionaly.com/share/QdREgX5mZu5y">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset=".github/assets/dashboard-dark.png">
      <img src=".github/assets/dashboard-light.png" alt="The Fusionaly dashboard for fusionaly.com: visitors, page views, sessions, bounce rate, and a 30-day visitors chart" width="900">
    </picture>
  </a>
  <br>
  <sub>The real dashboard for fusionaly.com. <a href="https://t.fusionaly.com/share/QdREgX5mZu5y">Open it live</a>.</sub>
</p>

<p align="center">
  Runs on a Raspberry Pi or a $5 VPS, on your own domain. One command installs it, and it updates itself every night.<br>
  Ask it questions from Claude, Codex, Cursor, or Gemini.
</p>

<p align="center">
  <a href="https://fusionaly.com">Website</a> ·
  <a href="https://fusionaly.com/docs">Documentation</a> ·
  <a href="https://fusionaly.com/docs/installation/">Installation</a> ·
  <a href="https://t.fusionaly.com/share/QdREgX5mZu5y">Live dashboard</a>
</p>

---

## Install

```bash
curl -fsSL https://fusionaly.com/install | bash
```

One command. It installs Docker if needed, gets a TLS certificate for your domain, and sets up nightly auto-updates and backups. Or pull the image yourself:

```bash
docker pull karloscodes/fusionaly:latest
```

Full setup in the [Installation Guide](https://fusionaly.com/docs/installation/).

> Don't host it on an `analytics.*` subdomain — ad blockers (uBlock Origin, EasyPrivacy) block hostnames like that and drop your tracking requests. Use a neutral subdomain such as `data.example.com`.

## How it works

Add the script to your site:

```html
<script defer src="https://your-domain.com/y/api/v1/sdk.js"></script>
```

Page views and clicks are tracked automatically. For named events, add one attribute — it works on any element:

```html
<button data-fusionaly-event-name="signup_clicked">Sign up</button>
<a href="/pricing" data-fusionaly-event-name="pricing_viewed">Pricing</a>
<form data-fusionaly-event-name="contact_submitted">...</form>
<section data-fusionaly-event-name="testimonials_seen">...</section>
```

The SDK picks the right trigger per element — click, submit, `sendBeacon`, or scroll-into-view. [Read the docs](https://fusionaly.com/docs/automated-tracking/).

## What you get

- **Tracking** — page views, SPA navigation, clicks, forms, sections, revenue, and custom events. Automatic where it can be, one attribute where it can't.
- **Dashboard** — visitors, sources, top pages, countries, devices, goals, and user flows. Daily charts follow your timezone, daylight saving included.
- **What's new** — a home feed across all your sites: traffic spikes and drops, new referrers, goal spikes, milestones. Each day is compared with the same weekday in past weeks, so a normal Monday peak is not news, and one viral day doesn't hide the next. Small sites stay quiet until something real happens.
- **Ask from your AI client** — Claude, Codex, Cursor, or Gemini answer questions about your traffic in plain English. See [Ask from your AI client](#ask-from-your-ai-client).
- **Annotations** — mark deployments, campaigns, and incidents on the timeline.
- **Shareable dashboards** — public read-only links.
- **Bot filtering & spam protection** — clean data by default.

## Privacy

- No cookies, no fingerprinting, no personal data stored.
- Visitors are counted with a daily-rotating hash, not a stable identifier.
- Everything stays on your server. No third parties, and Fusionaly never calls an AI provider.

## Ask from your AI client

Ask "where did my traffic come from this week?" in the AI client you already use. Fusionaly answers the Model Context Protocol at `/mcp` with four read-only tools, and the plugin adds a skill that turns questions into the right queries.

```
/plugin marketplace add karloscodes/fusionaly-oss
/plugin install fusionaly
/fusionaly:connect
```

That is Claude Code. Codex, Gemini CLI, Cursor, Claude Desktop, and others are one command each: see the [plugin README](plugin/README.md). Get the agent key from **Administration > Agents**.

Your client asks your server, and only the answer reaches your AI provider. The tools can't write, and they can't read accounts, settings, or keys.

## Self-hosting

- One SQLite file. No external database, no Redis, no queue.
- Runs on a Raspberry Pi or a $5 VPS.
- The installer sets up nightly auto-updates and local backups, no SSH chores. For off-server backups (VPS snapshots, Litestream, rsync), see [Backups](https://fusionaly.com/docs/server-administration/#backups).

If an update doesn't land, force one on the server — it pulls the latest image, redeploys, and repairs the auto-update cron:

```bash
sudo fusionaly update
```

## Tech stack

- **Backend**: Go (Fiber, GORM, SQLite, cartridge)
- **Frontend**: React, Inertia.js, Tailwind CSS, shadcn/ui
- **Tests**: Go testing, Playwright E2E

## Development

**Requirements:** Go 1.25+, Node.js 22+, SQLite

```bash
make install      # Install dependencies
make db-migrate   # Apply migrations
make dev          # Start dev server (hot reload)
```

Access at `http://localhost:3000`.

```bash
make test         # Unit tests (~3s)
make test-e2e     # Playwright E2E (~5min)
make lint         # Run linters
make build        # Production binaries
```

## Project structure

```
fusionaly/
├── cmd/
│   ├── fusionaly/     # Main server binary
│   ├── fnctl/         # CLI tool (migrations, admin tasks)
│   └── manager/       # Install, updates, and backups in production
├── internal/          # Core logic (Phoenix Contexts pattern)
├── api/v1/            # Public tracking API + SDK
├── web/               # React frontend (Inertia.js + Tailwind)
├── e2e/               # Playwright E2E tests
└── storage/           # Runtime data (SQLite database, GeoLite2)
```

## Configuration

Defaults work out of the box for development. For production, set:

- `FUSIONALY_DOMAIN` — your domain name
- `FUSIONALY_PRIVATE_KEY` — generate with `openssl rand -hex 32`

See the [Installation Guide](https://fusionaly.com/docs/installation/) for Docker setup and [SDK Configuration](https://fusionaly.com/docs/configuration/) for tracking options.

## Contributing

1. **Open an issue first** — discuss features or significant changes before starting.
2. **Bug fixes** — PRs welcome with a clear problem/solution description.
3. Run `make lint`, `make test`, and `make test-e2e` before submitting.

## License

[MIT](./LICENSE)

---

[Website](https://fusionaly.com) · [Docs](https://fusionaly.com/docs) · [Issues](https://github.com/karloscodes/fusionaly-oss/issues) · [Docker Hub](https://hub.docker.com/r/karloscodes/fusionaly)
