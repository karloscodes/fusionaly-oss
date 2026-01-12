# Fusionaly

Privacy-first, self-hosted web analytics. Simple, fast, no cookies required.

## Features

- **Privacy-First**: No cookies, no IP logging, GDPR-compliant by design
- **Self-Hosted**: Full control over your data
- **Fast**: Go backend + SQLite (WAL mode) + Inertia.js frontend
- **Lightweight Tracking**: ~6KB SDK (gzipped), minimal performance impact
- **Event Tracking**: Custom events, goals, revenue tracking
- **Real-time Dashboard**: Live visitor counts, page views, referrers

## Architecture

Fusionaly uses a three-tier data pipeline optimized for durability and performance:

1. **Ingestion**: Browser SDK batches events (200ms intervals) → HTTP API → `ingested_events` table
2. **Processing**: Background jobs process raw events → `events` table with retries and backpressure
3. **Aggregation**: Hourly rollups into optimized tables for fast dashboard queries (<10ms)

SQLite with WAL mode handles hundreds of thousands of daily events per installation. See [Internal Architecture](https://fusionaly.com/docs/internal-architecture/) for details.

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

### Core Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `FUSIONALY_APP_PORT` | `3000` | HTTP server port |
| `FUSIONALY_ENV` | `development` | Environment: `development`, `production`, `test` |
| `FUSIONALY_DOMAIN` | `localhost:3000` | Public domain for cookies/CSRF |
| `FUSIONALY_LOG_LEVEL` | `debug` | Log level: `debug`, `info`, `warn`, `error` |

### Storage

| Variable | Default | Description |
|----------|---------|-------------|
| `FUSIONALY_STORAGE_PATH` | `storage` | SQLite database directory |
| `FUSIONALY_DB_MAX_OPEN_CONNS` | `10` (prod) / `1` (test) | Max database connections |
| `FUSIONALY_DB_MAX_IDLE_CONNS` | `5` (prod) / `1` (test) | Idle database connections |

### GeoIP (Optional - for location data)

| Variable | Default | Description |
|----------|---------|-------------|
| `FUSIONALY_GEO_DB_PATH` | `internal-storage/GeoLite2-City.mmdb` | Path to GeoLite2 database |

GeoLite2 enables country/city detection in your analytics. Without it, all visitor locations show as "Unknown" but event tracking works normally.

**To enable location detection:**
1. Register at [MaxMind](https://www.maxmind.com/en/geolite2/signup) (free account)
2. Download GeoLite2-City.mmdb from your MaxMind account
3. Place it at `internal-storage/GeoLite2-City.mmdb` or configure `FUSIONALY_GEO_DB_PATH`
4. Restart Fusionaly

### Session & Jobs

| Variable | Default | Description |
|----------|---------|-------------|
| `FUSIONALY_SESSION_TIMEOUT_SECONDS` | `1800` | Session timeout (30 minutes) |
| `FUSIONALY_JOB_INTERVAL_SECONDS` | `60` | Background job interval |
| `FUSIONALY_INGESTED_EVENTS_RETENTION_DAYS` | `90` | Days to keep raw events |

### Logging

| Variable | Default | Description |
|----------|---------|-------------|
| `FUSIONALY_LOGS_DIR` | `logs` | Log file directory |
| `FUSIONALY_LOGS_MAX_SIZE_IN_MB` | `20` | Max log file size |
| `FUSIONALY_LOGS_MAX_BACKUPS` | `10` | Number of log backups |
| `FUSIONALY_LOGS_MAX_AGE_IN_DAYS` | `30` | Days to keep logs |

## Contributing

We welcome contributions! Please follow these guidelines:

1. **Open an issue first** - Before submitting a feature or significant change, open an issue to discuss it. This ensures your contribution aligns with the project direction and avoids wasted effort.

2. **Bug fixes** - Small bug fixes can be submitted directly as PRs with a clear description of the problem and solution.

3. **Code style** - Follow existing patterns. Run `make lint` before submitting.

4. **Tests** - Add tests for new functionality. Run `make test` and `make test-e2e` to verify.

## Documentation

Full documentation at [fusionaly.com/docs](https://fusionaly.com/docs)

## License

AGPL-3.0 - see [LICENSE](./LICENSE)

## Links

- Website: [fusionaly.com](https://fusionaly.com)
- Documentation: [fusionaly.com/docs](https://fusionaly.com/docs)
- Issues: [GitHub Issues](https://github.com/karloscodes/fusionaly-oss/issues)
