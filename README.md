# Fusionaly

[![Latest Release](https://img.shields.io/github/v/release/karloscodes/fusionaly-oss)](https://github.com/karloscodes/fusionaly-oss/releases/latest)
[![License: MIT](https://img.shields.io/github/license/karloscodes/fusionaly-oss)](./LICENSE)
[![CI](https://github.com/karloscodes/fusionaly-oss/actions/workflows/pr.yml/badge.svg)](https://github.com/karloscodes/fusionaly-oss/actions)
[![Docker](https://img.shields.io/docker/pulls/karloscodes/fusionaly)](https://hub.docker.com/r/karloscodes/fusionaly)

Self-hosted web analytics with SQLite. See who visits, where they come from, and what they click — without cookies, fingerprinting, or handing your data to anyone else.

Runs on hardware you already have: a Raspberry Pi or a $5 VPS, on your own domain. One script tag to track, one command to install.

[Website](https://fusionaly.com) · [Documentation](https://fusionaly.com/docs) · [Installation](https://fusionaly.com/docs/installation/)

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
- **Dashboard** — visitors, sources, top pages, countries, devices, goals, and user flows.
- **What's new** — a home feed across all your sites: traffic spikes, new referrers, milestones. Stays quiet until something real happens.
- **Ask** — ask a question in plain English and get a chart and the SQL back. Optional; see [Ask (AI)](#ask-ai) for what it does and doesn't send.
- **Annotations** — mark deployments, campaigns, and incidents on the timeline.
- **Shareable dashboards** — public read-only links.
- **Bot filtering & spam protection** — clean data by default.

## Privacy

- No cookies, no fingerprinting, no personal data stored.
- Visitors are counted with a daily-rotating hash, not a stable identifier.
- Everything stays on your server. No third parties — unless you turn on Ask.

## Ask (AI)

Ask is optional and stays off until you add a key. It connects to [OpenRouter](https://openrouter.ai) (bring your own key), so you can pick any model. When you ask a question, only your **database schema** and **the question you type** are sent to OpenRouter — never your visitors' data. The generated SQL is read-only.

## Self-hosting

- One SQLite file. No external database, no Redis, no queue.
- Runs on a Raspberry Pi or a $5 VPS.
- The installer sets up nightly auto-updates and backups — no SSH chores.

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
