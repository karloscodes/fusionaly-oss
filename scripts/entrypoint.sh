#!/bin/sh
set -e

DB_PATH="/app/storage/fusionaly-production.db"
BACKUP_ENABLED="${ENABLE_BACKUPS:-false}"
LOG_DIR="/app/logs"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Ensure log directory and file are writable
mkdir -p "$LOG_DIR"
chmod -R u+w "$LOG_DIR"

# Log function
log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_DIR/entrypoint.log"
}

# Function to run migrations
run_migrations() {
  log "Checking database status..."

  log "Running database migrations..."
  /app/fnctl migrate
  log "Migrations completed successfully"
}

# Handle different command modes
COMMAND=${1:-"server"}

case "$COMMAND" in
  migrate)
    log "Running migrations only..."
    run_migrations
    ;;

  server)
    log "Starting web server with ID ${SERVER_INSTANCE_ID}..."
    run_migrations
    exec /app/fusionaly
    ;;

  shell)
    log "Starting shell..."
    exec /bin/sh
    ;;

  create-admin-user)
    log "Creating admin user..."
    /app/fnctl create-admin-user $2 $3
    ;;

  change-admin-password)
    log "Changing admin password..."
    /app/fnctl change-admin-password $2 $3
    ;;

  *)
    log "Unknown command: $COMMAND"
    log "Valid commands: migrate, backup, server (default), shell"
    exit 1
    ;;
esac
