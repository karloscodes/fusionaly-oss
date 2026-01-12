# Fusionaly DevBox

One-command local environment to explore Fusionaly with a beautiful, interactive demo.

## Quick Start (one command)

Run this from your terminal:

```bash
# Raw GitHub (main branch)
curl -fsSL https://raw.githubusercontent.com/karloscodes/fusionaly-devbox/refs/heads/main/setup.sh | bash
```

- Demo site: http://localhost:8080
- Dashboard: http://localhost:8080/admin

## What You Get

- Full Fusionaly server (Docker) via Caddy
- Beautiful demo at `/` with interactive event buttons
- Admin dashboard under `/admin`
- Runs on http://localhost:8080 (no HTTPS needed)
- No license required — DevBox runs in test mode with a test key

## Manual Setup (clone + run)

```bash
git clone https://github.com/karloscodes/fusionaly-devbox.git
cd fusionaly-devbox
chmod +x setup.sh
./setup.sh
```

Or without the helper script:

```bash
docker compose up -d
```

## Useful Commands

```bash
# View logs
docker compose logs -f

# Stop everything
docker compose down

# Clean up (removes all data)
docker compose down -v

# Restart services
docker compose restart
```

## Troubleshooting

- Port in use: free port 8080
- Still starting: give it 1–2 minutes on first run

## Documentation

Full docs and one‑liner: https://fusionaly.com/docs/devbox/

## Contributing

Issues and PRs welcome.
