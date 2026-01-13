# Fusionaly

Privacy-first, self-hosted web analytics. Simple, fast, no cookies required.

**Documentation:** [fusionaly.com/docs](https://fusionaly.com/docs)

## Development Setup

**Requirements:** Go 1.25+, Node.js 22+, SQLite

```bash
# Install dependencies
make install

# Run database migrations
make db-migrate

# Start dev server (hot reload)
make dev
```

Access at `http://localhost:3000`

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
└── storage/           # Runtime data (database, GeoLite2, uploads)
```

## Binaries

| Binary | Purpose |
|--------|---------|
| `fusionaly` | Main web server |
| `fnctl` | CLI: migrations, create-admin-user, export |
| `fusionaly-manager` | Production: health checks, graceful upgrades |

## Commands

```bash
make dev          # Run with hot reload
make test         # Run unit tests (~3s)
make test-e2e     # Run Playwright tests (~5min)
make build        # Build production binaries
make lint         # Run linters
make db-migrate   # Apply migrations
make db-seed      # Migrations + sample data
```

## Tech Stack

- **Backend**: Go 1.25, Fiber, GORM, SQLite
- **Frontend**: React 19, Inertia.js, Tailwind CSS, shadcn/ui
- **Testing**: Go testing, Playwright

## Configuration

For development, defaults work out of the box.

For production deployment and configuration options, see [Installation Guide](https://fusionaly.com/docs/installation/).

**Key environment variables:**
- `FUSIONALY_ENV` - Set to `production` for production
- `FUSIONALY_PRIVATE_KEY` - **Required in production** (generate with `openssl rand -hex 32`)

## Contributing

1. **Open an issue first** - Discuss features or significant changes before starting work
2. **Bug fixes** - Submit PRs directly with clear problem/solution description
3. **Code style** - Run `make lint` before submitting
4. **Tests** - Run `make test` and `make test-e2e` to verify changes

## License

AGPL-3.0 - see [LICENSE](./LICENSE)

## Links

- Website: [fusionaly.com](https://fusionaly.com)
- Documentation: [fusionaly.com/docs](https://fusionaly.com/docs)
- Issues: [GitHub Issues](https://github.com/karloscodes/fusionaly-oss/issues)
