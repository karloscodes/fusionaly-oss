# Fusionaly

[![Latest Release](https://img.shields.io/github/v/release/karloscodes/fusionaly-oss)](https://github.com/karloscodes/fusionaly-oss/releases/latest)
[![License: AGPL-3.0](https://img.shields.io/github/license/karloscodes/fusionaly-oss)](./LICENSE)
[![CI](https://github.com/karloscodes/fusionaly-oss/actions/workflows/pr.yml/badge.svg)](https://github.com/karloscodes/fusionaly-oss/actions)
[![Docker](https://img.shields.io/docker/pulls/karloscodes/fusionaly)](https://hub.docker.com/r/karloscodes/fusionaly)

Privacy-first, self-hosted web analytics. No cookies, no fingerprinting, no personal data stored.

**One script tag. One attribute. You own everything.**

[Website](https://fusionaly.com) · [Documentation](https://fusionaly.com/docs) · [Installation](https://fusionaly.com/docs/installation/) · [Free vs Pro](https://fusionaly.com/docs/editions/)

---

## Install

```bash
curl -fsSL https://fusionaly.com/install | bash
```

Or with Docker:

```bash
docker pull karloscodes/fusionaly:latest
```

See the [Installation Guide](https://fusionaly.com/docs/installation/) for full setup instructions.

## How It Works

Add the tracking script to your site:

```html
<script defer src="https://your-domain.com/y/api/v1/sdk.js"></script>
```

Page views and button clicks are tracked automatically. Want more? One attribute works on any element:

```html
<button data-fusionaly-event-name="signup_clicked">Sign Up</button>
<a href="/pricing" data-fusionaly-event-name="pricing_viewed">Pricing</a>
<form data-fusionaly-event-name="contact_submitted">...</form>
<section data-fusionaly-event-name="testimonials_seen">...</section>
```

The SDK does the right thing based on element type — click, submit, sendBeacon, or scroll into view. [Read the docs](https://fusionaly.com/docs/automated-tracking/).

## Features

- **Page views & SPA navigation** — automatic, zero config
- **Button & link tracking** — automatic or named with one attribute
- **Form tracking** — tracks on submit, suppresses button double-events
- **Section tracking** — fires when scrolled into view (50% visible)
- **Revenue tracking** — purchases with price, currency, metadata
- **Custom events** — JavaScript API for dynamic data
- **Goal conversions** — track signups, purchases, any event as a goal
- **User flows** — see how visitors navigate entry to exit
- **Annotations** — mark deployments, campaigns, incidents on your timeline
- **Shareable dashboards** — public links to your analytics
- **Bot filtering & spam protection** — clean data by default
- **Subdomain tracking** — first-party, ad-block proof

## Tech Stack

- **Backend**: Go, Fiber, GORM, SQLite
- **Frontend**: React, Inertia.js, Tailwind CSS, shadcn/ui
- **Testing**: Go testing, Playwright E2E

## Development

**Requirements:** Go 1.25+, Node.js 22+, SQLite

```bash
make install      # Install dependencies
make db-migrate   # Apply migrations
make dev          # Start dev server (hot reload)
```

Access at `http://localhost:3000`

```bash
make test         # Unit tests (~3s)
make test-e2e     # Playwright E2E (~5min)
make lint         # Run linters
make build        # Production binaries
```

## Project Structure

```
fusionaly/
├── cmd/
│   ├── fusionaly/     # Main server binary
│   ├── fnctl/         # CLI tool (migrations, admin tasks)
│   └── manager/       # Production manager (health checks, upgrades)
├── internal/          # Core business logic (Phoenix Contexts pattern)
├── api/v1/            # Public tracking API + SDK
├── web/               # React frontend (Inertia.js + Tailwind)
├── e2e/               # Playwright E2E tests
└── storage/           # Runtime data (database, GeoLite2)
```

## Configuration

Defaults work out of the box for development.

For production, set:
- `FUSIONALY_DOMAIN` — your domain name
- `FUSIONALY_PRIVATE_KEY` — generate with `openssl rand -hex 32`

See [Installation Guide](https://fusionaly.com/docs/installation/) for Docker setup and [SDK Configuration](https://fusionaly.com/docs/configuration/) for tracking options.

## Contributing

1. **Open an issue first** — discuss features or significant changes before starting
2. **Bug fixes** — PRs welcome with clear problem/solution description
3. **Run `make lint`** before submitting
4. **Run `make test` and `make test-e2e`** to verify changes

## License

[AGPL-3.0](./LICENSE)

---

[Website](https://fusionaly.com) · [Docs](https://fusionaly.com/docs) · [Issues](https://github.com/karloscodes/fusionaly-oss/issues) · [Docker Hub](https://hub.docker.com/r/karloscodes/fusionaly)
