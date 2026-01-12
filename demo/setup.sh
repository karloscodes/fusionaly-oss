#!/usr/bin/env bash
set -euo pipefail

# Colors and formatting
green="\033[0;32m"; yellow="\033[1;33m"; red="\033[0;31m"; blue="\033[0;34m"; reset="\033[0m"
print_status() { echo -e "${green}✓${reset} $1"; }
print_info()   { echo -e "${blue}›${reset} $1"; }
print_warn()   { echo -e "${yellow}!${reset} $1"; }
print_error()  { echo -e "${red}✗${reset} $1"; }

command_exists() { command -v "$1" >/dev/null 2>&1; }

echo ""
echo "Fusionaly Demo"
echo "=========================="
echo ""

# Check Docker
if ! command_exists docker; then
  print_error "Docker is not installed or not in PATH."
  exit 1
fi
if ! docker info >/dev/null 2>&1; then
  print_error "Docker is not running. Please start Docker Desktop."
  exit 1
fi
print_status "Docker is available"

# Compose command
if command_exists docker-compose; then
  COMPOSE_CMD="docker-compose"
elif docker compose version >/dev/null 2>&1; then
  COMPOSE_CMD="docker compose"
else
  print_error "Docker Compose is not available."
  exit 1
fi
print_status "Using $COMPOSE_CMD"

# Port checks
print_info "Checking for port conflicts..."
conflict=0
for p in 8080; do
  if lsof -Pi :$p -sTCP:LISTEN -t >/dev/null 2>&1; then
    print_warn "Port $p is already in use"
    conflict=1
  fi
done
if [ $conflict -eq 1 ]; then
  read -p "Continue anyway? Some services may fail to start. (y/N): " -n 1 -r; echo
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    print_info "Setup cancelled. Please free up port 8080."
    exit 1
  fi
else
  print_status "Port 8080 available"
fi

print_info "Pulling latest Fusionaly image..."
if docker pull karloscodes/fusionaly-oss:latest; then
  print_status "Latest image pulled successfully"
else
  print_warn "Could not pull latest image, using cached version"
fi

# Ensure compose files exist; if not, fetch them to current directory
REPO_RAW_BASE="https://raw.githubusercontent.com/karloscodes/fusionaly-oss/main/demo"
WORKDIR="$PWD"
if [ ! -f "$WORKDIR/docker-compose.yml" ]; then
  print_info "Downloading Fusionaly Demo to current directory"
  curl -fsSL "$REPO_RAW_BASE/docker-compose.yml" -o "$WORKDIR/docker-compose.yml"
  curl -fsSL "$REPO_RAW_BASE/Caddyfile" -o "$WORKDIR/Caddyfile"
  curl -fsSL "$REPO_RAW_BASE/index.html" -o "$WORKDIR/index.html"
  print_status "Demo files downloaded to current directory"
  print_info "You can now stop the demo with: docker compose down"
  print_info "Or restart it with: docker compose up -d"
fi

# Helper to run compose in the working directory
run_compose() { (cd "$WORKDIR" && $COMPOSE_CMD "$@"); }

# Helper to run commands in fusionaly container
run_in_container() {
  docker exec fusionaly-demo sh -lc "$1" 2>/dev/null || return 1
}

print_info "Starting demo services..."
# Clean up any existing containers with fixed names
docker rm -f fusionaly-demo >/dev/null 2>&1 || true
docker rm -f caddy-demo >/dev/null 2>&1 || true
run_compose up -d

# Wait for health via Caddy HTTP and ensure database is accessible
print_info "Waiting for Fusionaly to be ready..."
HEALTH_READY=false
for i in {1..30}; do
  if curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/_health | grep -q "200"; then
    if run_in_container "sqlite3 /app/storage/fusionaly-test.db 'SELECT 1;'" >/dev/null 2>&1; then
      print_status "Fusionaly and database are ready!"
      HEALTH_READY=true
      break
    else
      print_info "HTTP ready, waiting for database initialization... ($i/30)"
    fi
  else
    print_info "Waiting for Fusionaly to start... ($i/30)"
  fi
  sleep 2
  if [ $i -eq 30 ]; then
    print_warn "Fusionaly may still be starting up. Proceeding anyway..."
  fi
done

if [ "$HEALTH_READY" = false ]; then
  print_warn "Fusionaly may not be fully ready. User creation might fail."
fi

# Safety guard: ensure container runs in test mode
ENV_MODE=$(run_in_container 'printenv FUSIONALY_ENV || echo ""' 2>/dev/null || echo "")
if [ "$ENV_MODE" != "test" ]; then
  print_error "Demo must run with FUSIONALY_ENV=test (got: '$ENV_MODE'). Aborting to prevent misuse."
  exit 1
fi
print_status "Verified test mode"

# Safety guard: ensure test-only license key is set
LIC_KEY=$(run_in_container 'printenv FUSIONALY_LICENSE_KEY || echo ""' 2>/dev/null || echo "")
if [ "$LIC_KEY" != "IM-DEMO-TEST-ONLY" ]; then
  print_error "Unexpected license key inside container. Expected IM-DEMO-TEST-ONLY. Aborting."
  exit 1
fi
print_status "Verified test license key"

# Ensure a 'localhost' website exists for event ingestion
print_info "Ensuring 'localhost' website exists..."
if run_in_container "sqlite3 /app/storage/fusionaly-test.db \"INSERT OR IGNORE INTO websites (domain, created_at) VALUES ('localhost', datetime('now'));\"" >/dev/null 2>&1; then
  print_status "Website 'localhost' is present"
else
  print_warn "Could not create 'localhost' website automatically. You can add it later in the dashboard."
fi

# Ensure test license key is persisted in DB settings
print_info "Persisting test license key in settings..."
if run_in_container "sqlite3 /app/storage/fusionaly-test.db \"INSERT INTO settings (key, value, created_at, updated_at) VALUES ('license_key','IM-DEMO-TEST-ONLY', datetime('now'), datetime('now')) ON CONFLICT(key) DO UPDATE SET value='IM-DEMO-TEST-ONLY', updated_at=datetime('now');\"" >/dev/null 2>&1; then
  print_status "License key persisted to DB"
else
  print_warn "Could not persist license key into DB. Env key will still be used."
fi

# Ensure an admin user exists with known credentials
ADMIN_EMAIL="admin@example.com"
ADMIN_PASSWORD="demo"
print_info "Ensuring admin user ($ADMIN_EMAIL) exists..."

# Retry user creation with better error handling
USER_CREATION_SUCCESS=false
for attempt in {1..3}; do
  FNCTL_OK=$(run_in_container 'test -x /app/fnctl && echo OK || echo MISSING' 2>/dev/null || echo MISSING)
  if [ "$FNCTL_OK" = "OK" ]; then
    if run_in_container "/app/fnctl create-admin-user '$ADMIN_EMAIL' '$ADMIN_PASSWORD' >/dev/null 2>&1 || /app/fnctl change-admin-password '$ADMIN_EMAIL' '$ADMIN_PASSWORD' >/dev/null 2>&1"; then
      print_status "Admin user ready (via fnctl)"
      USER_CREATION_SUCCESS=true
      break
    else
      print_warn "fnctl attempt $attempt failed, retrying..."
      sleep 2
    fi
  else
    print_warn "fnctl not found in container (attempt $attempt)"
    sleep 2
  fi
done

# Final check: ensure at least one user exists
USER_COUNT=$(run_in_container "sqlite3 /app/storage/fusionaly-test.db 'SELECT COUNT(*) FROM users;' 2>/dev/null || echo 0" | tr -dc '0-9')
if [ "$USER_COUNT" = "" ]; then USER_COUNT=0; fi
if [ "$USER_COUNT" -gt 0 ]; then
  print_status "Detected $USER_COUNT user(s) in database"
else
  print_error "CRITICAL: No users detected in database! Admin user creation failed."
  print_error "This will cause the onboarding flow to appear instead of the login page."
  print_info "You can manually create a user by running:"
  print_info "  cd $WORKDIR && $COMPOSE_CMD exec fusionaly /app/fnctl create-admin-user '$ADMIN_EMAIL' '$ADMIN_PASSWORD'"
  print_warn "Continuing anyway, but /admin will redirect to onboarding..."
fi


echo ""
echo "Fusionaly Demo is Ready!"
echo "==================================="
print_status "Demo:      http://localhost:8080"
print_status "Dashboard: http://localhost:8080/admin"
print_status "Admin:     admin@example.com / demo"
echo ""
echo "Management Commands:"
print_status "Logs:     docker logs -f fusionaly-demo"
print_status "Stop:     docker compose down"
print_status "Restart:  docker compose up -d"
echo ""

# Try opening demo
if command_exists open; then open http://localhost:8080 2>/dev/null || true; fi
if command_exists xdg-open; then xdg-open http://localhost:8080 2>/dev/null || true; fi

echo "Happy testing!"
