#!/bin/bash
# Development server script that ensures clean port binding
# Handles the case where the previous process hasn't released the port yet

set -e

PORT="${APP_PORT:-3000}"
TMP_DIR="${TMP_DIR:-tmp}"
BINARY_NAME="${BINARY_NAME:-fusionaly}"

# Function to kill process on port
kill_port() {
    local pid=$(lsof -ti :$PORT 2>/dev/null || true)
    if [ -n "$pid" ]; then
        echo "Killing process on port $PORT (PID: $pid)..."
        kill -9 $pid 2>/dev/null || true
        sleep 0.5
    fi
}

# Function to wait for port to be free
wait_for_port() {
    local max_wait=5
    local waited=0
    while lsof -ti :$PORT >/dev/null 2>&1; do
        if [ $waited -ge $max_wait ]; then
            echo "Port $PORT still in use after ${max_wait}s, force killing..."
            kill_port
            break
        fi
        echo "Waiting for port $PORT to be released..."
        sleep 0.5
        waited=$((waited + 1))
    done
}

# Build the binary
echo "Building server..."
go build -o "$TMP_DIR/$BINARY_NAME" cmd/fusionaly/main.go

# Ensure port is free
wait_for_port

# Start the server
echo "Starting server on port $PORT..."
FUSIONALY_ENV=development exec "./$TMP_DIR/$BINARY_NAME"
