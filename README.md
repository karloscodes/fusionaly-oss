# Fusionaly

Privacy-first, self-hosted web analytics. Simple, fast, no cookies required.

## Features

- **Privacy-First**: No cookies, no IP logging, GDPR-compliant by design
- **Self-Hosted**: Full control over your data
- **Fast**: Go backend + SQLite (WAL mode) + Inertia.js frontend
- **Lightweight Tracking**: ~1KB SDK, minimal performance impact
- **Event Tracking**: Custom events, goals, revenue tracking
- **Real-time Dashboard**: Live visitor counts, page views, referrers

## Project Structure

```
fusionaly/
├── cmd/
│   ├── fusionaly/     # Main server binary
│   ├── fnctl/         # CLI tool (migrations, admin tasks)
│   └── manager/       # Production manager (health checks, upgrades)
├── internal/          # Core business logic (Phoenix Contexts pattern)
├── api/v1/            # Public tracking API endpoints
├── web/               # React frontend (Inertia.js + Tailwind)
├── e2e/               # Playwright E2E tests
├── demo/              # Docker demo environment
└── scripts/           # Development and deployment scripts
```

## Quick Start

### Docker (Recommended)

```bash
docker compose up -d
```

Access at `http://localhost:3000`

### Development

**Requirements:** Go 1.25+, Node.js 22+, SQLite

```bash
# Install dependencies
make install

# Run database migrations
make db-migrate

# Start dev server (hot reload)
make dev
```

## Binaries

| Binary | Purpose |
|--------|---------|
| `fusionaly` | Main web server |
| `fnctl` | CLI: migrations, create-admin-user, export |
| `fusionaly-manager` | Production: health checks, graceful upgrades |

## Tracking Code

Add to your website's `<head>`:

```html
<script defer src="https://your-fusionaly.com/y/api/v1/sdk.js" data-domain="your-site.com"></script>
```

### Custom Events

```javascript
fusionaly.track('signup', { plan: 'pro' });
fusionaly.track('purchase', { revenue: 99.99, currency: 'USD' });
```

## Development Commands

```bash
make dev          # Run with hot reload
make test         # Run unit tests (~3s)
make test-e2e     # Run Playwright tests (~5min)
make build        # Build production binaries
make lint         # Run linters
```

## Tech Stack

- **Backend**: Go 1.25, Fiber, GORM, SQLite
- **Frontend**: React 19, Inertia.js, Tailwind CSS, shadcn/ui
- **Testing**: Go testing, Playwright
- **Deployment**: Docker, Caddy (reverse proxy)

## Free vs Pro

This is the **Free** edition. Everything you need for web analytics.

**[Fusionaly Pro](https://fusionaly.com/#pricing)** ($100 one-time) adds:
- Ask AI - query data in plain English
- Lens - saved custom queries
- Public dashboards
- Priority support

## Configuration

Copy `.env.example` to `.env` and configure:

### Required (Production)

| Variable | Description |
|----------|-------------|
| `FUSIONALY_PRIVATE_KEY` | Session encryption key (32+ random characters). **Must change from default!** |

### Optional

| Variable | Default | Description |
|----------|---------|-------------|
| `FUSIONALY_APP_PORT` | `3000` | HTTP server port |
| `FUSIONALY_ENV` | `development` | Environment: `development`, `production`, `test` |
| `FUSIONALY_LOG_LEVEL` | `debug` | Log level: `debug`, `info`, `warn`, `error` |
| `FUSIONALY_DOMAIN` | `localhost:3000` | Public domain for cookies/CSRF |
| `FUSIONALY_STORAGE_PATH` | `storage` | SQLite database directory |
| `FUSIONALY_GEO_DB_PATH` | `internal-storage/GeoLite2-City.mmdb` | GeoLite2 database for location detection (optional) |
| `FUSIONALY_SESSION_TIMEOUT_SECONDS` | `1800` | Session timeout (30 minutes) |
| `FUSIONALY_DB_MAX_OPEN_CONNS` | Auto | Database connection pool size |
| `FUSIONALY_LOGS_DIR` | `logs` | Log file directory |

### GeoIP (Optional)

For country/city detection, register at [MaxMind](https://www.maxmind.com/en/geolite2/signup) and download GeoLite2-City.mmdb. Works without it.

## Documentation

Full documentation at [fusionaly.com/docs](https://fusionaly.com/docs)

## License

AGPL-3.0 - see [LICENSE](./LICENSE)

## Links

- Website: [fusionaly.com](https://fusionaly.com)
- Documentation: [fusionaly.com/docs](https://fusionaly.com/docs)
- Issues: [GitHub Issues](https://github.com/karloscodes/fusionaly-oss/issues)
