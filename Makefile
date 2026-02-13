# Configuration
SHELL := /bin/bash
PROJECT_NAME := fusionaly
GO := go
NPM := npm
DIST_DIR := dist
TMP_DIR := tmp
APP_PORT ?= 3000
GOTESTSUM := $(shell command -v gotestsum 2> /dev/null)
WATCHEXEC := watchexec
BINARY_NAME := fusionaly
IMCTL_BINARY := fnctl
MANAGER_BINARY := fusionaly-manager
PID_FILE := ./.dev-server.pid
DEVICE_DETECTOR_VERSION := master
UA_DATABASE_DIR := internal/pkg/user_agent/database

# Version from git tag or dev
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: default install deps install-tools dev watch-web watch-go dev-server dev-web db-seed db-migrate db-drop test test-e2e test-installer perftest perf-run loadtest loadtest-quick loadtest-heavy build build-manager build-manager-linux clean dist lint update-ua-database download-test-fixtures test-ua-parser test-ua-fixtures check release

# Default target
default: dev

# Run all CI checks locally
check:
	@./scripts/check.sh

# Create a new release (triggers GoReleaser via GitHub Actions)
# Usage: make release v=1.0.0
release:
	@if [ -z "$(v)" ]; then \
		echo "Usage: make release v=1.0.0"; \
		exit 1; \
	fi
	@echo "Creating release v$(v)..."
	@git diff --quiet || (echo "Error: Uncommitted changes. Commit first." && exit 1)
	git tag -a "v$(v)" -m "Release v$(v)"
	git push origin "v$(v)"
	@echo ""
	@echo "Release v$(v) triggered!"
	@echo "GoReleaser will build: binaries + Docker images + GitHub release"
	@echo "Watch: https://github.com/$$(gh repo view --json nameWithOwner -q .nameWithOwner)/actions"

# Test release locally (without publishing)
release-dry-run:
	goreleaser release --snapshot --clean

# Install dependencies and tools
install: deps install-tools

deps:
	@echo "Installing dependencies..."
	$(GO) mod tidy
	cd web && $(NPM) install --legacy-peer-deps
	cd e2e && $(NPM) install

install-tools:
	@echo "Installing development tools..."
	brew install watchexec golangci-lint act || true  # Ignore if already installed
	@if [ -z "$(GOTESTSUM)" ]; then \
		echo "Installing gotestsum..."; \
		$(GO) install gotest.tools/gotestsum@latest; \
	else \
		echo "gotestsum already installed at $(GOTESTSUM)"; \
	fi
	cd e2e && $(NPM) install && npx playwright install --with-deps --no-shell chromium

# Run development servers in parallel
dev:
	@echo "Starting development servers..."
	@trap 'kill 0' SIGINT SIGTERM EXIT; \
	$(MAKE) -j 2 watch-web watch-go

# Watch for changes in Go files
watch-go:
	@echo "Watching Go files..."
	$(WATCHEXEC) \
		--stop-timeout 10s \
		-w . \
		-i "web/*" \
		-i "$(DIST_DIR)/*" \
		-i ".git/*" \
		-i "$(TMP_DIR)/*" \
		-e go \
		-r \
		-- $(MAKE) dev-server

# Watch for changes in web files
watch-web:
	$(WATCHEXEC) \
		--stop-timeout 10s \
		-w web \
		-i "web/public/*" \
		-i "web/node_modules/*" \
		-i "web/dist/*" \
		-i "web/.env" \
		-r \
		-- $(MAKE) dev-web

# Target for Go server with auto-reload
dev-server:
	@echo "Building and starting server..."
	@mkdir -p $(TMP_DIR)
	$(GO) build -o $(TMP_DIR)/$(BINARY_NAME) cmd/fusionaly/main.go
	@FUSIONALY_ENV=development exec ./$(TMP_DIR)/$(BINARY_NAME)

# Target for npm development
dev-web:
	@echo "Starting npm dev server..."
	cd web && $(NPM) run dev

# Database operations
db-build-tools:
	@echo "Building database tools..."
	@mkdir -p $(TMP_DIR)
	$(GO) build -o $(TMP_DIR)/fnctl cmd/fnctl/main.go

db-migrate: db-build-tools
	@echo "Running migrations..."
	./$(TMP_DIR)/fnctl migrate

db-seed: db-migrate
	@echo "Running database seed..."
	./$(TMP_DIR)/fnctl seed

db-drop:
	@if [ "$(FUSIONALY_ENV)" = "production" ]; then \
		echo "Error: Cannot drop the database in production environment"; \
		exit 1; \
	else \
		CURRENT_ENV=$${FUSIONALY_ENV:-development}; \
		echo "Dropping database for environment: $$CURRENT_ENV"; \
		rm -f storage/fusionaly-$$CURRENT_ENV.db storage/fusionaly-$$CURRENT_ENV.db-*; \
		echo "Database for $$CURRENT_ENV environment dropped"; \
	fi

# Testing
test:
	@echo "Running Go unit tests..."
	@mkdir -p storage
	FUSIONALY_ENV=test $(MAKE) db-drop
	FUSIONALY_ENV=test $(MAKE) db-migrate
	@echo "Running tests..."
	@if [ -n "$(t)" ]; then \
		FUSIONALY_ENV=test gotestsum --format testname -- --run "$(t)" ./...; \
	else \
		FUSIONALY_ENV=test gotestsum --format testname ./...; \
	fi

# Linting
lint:
	@echo "Running linters..."
	@echo "Running staticcheck..."
	@staticcheck ./...
	@echo "Running golangci-lint..."
	@golangci-lint run

# E2E Testing (optimized for speed)
test-e2e:
	@echo "Running E2E tests..."
	@echo "Checking if Fusionaly is already running..."
	@if lsof -i :$(APP_PORT) >/dev/null 2>&1; then \
		echo "ERROR: Port $(APP_PORT) is already in use. Please stop any running Fusionaly instances before running e2e tests."; \
		echo "You can check what's running on port $(APP_PORT) with: lsof -i :$(APP_PORT)"; \
		echo "To kill the process, use: kill \$$(lsof -ti :$(APP_PORT))"; \
		exit 1; \
	fi
	@echo "Ensuring Playwright browsers are installed..."
	@cd e2e && npx playwright install --with-deps chromium
	@echo "Building web assets (cached)..."
	@cd web && $(NPM) run build --if-present
	@echo "Building fnctl (cached)..."
	@$(GO) build -o $(TMP_DIR)/fnctl cmd/fnctl/main.go
	@echo "Running Playwright tests..."
	@cd e2e && FUSIONALY_ENV=test npx playwright test
	@echo "Cleaning up..."

# Test installer locally using OrbStack VM
# Requires: brew install orbstack
test-installer:
	@echo "Building manager for Linux arm64..."
	@mkdir -p $(TMP_DIR)
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build \
		-ldflags "-X main.currentManagerVersion=$(VERSION)" \
		-o $(TMP_DIR)/fusionaly-test-linux ./cmd/manager/
	@echo "Recreating OrbStack VM..."
	@orb delete installer-test -f 2>/dev/null || true
	@orb create ubuntu:22.04 installer-test
	@echo ""
	@echo "Run the installer with:"
	@echo "  orb -m installer-test -u root $(CURDIR)/$(TMP_DIR)/fusionaly-test-linux install"
	@echo ""
	@echo "Or run interactively:"
	@echo "  orb -m installer-test"
	@echo ""

perf-run:
	@echo "Running performance test with custom parameters..."
	@PERF_URL=$${PERF_URL:-http://localhost:3000}; \
	PERF_CONCURRENCY=$${PERF_CONCURRENCY:-10}; \
	PERF_DURATION=$${PERF_DURATION:-30s}; \
	PERF_RATE=$${PERF_RATE:-0}; \
	echo "Running performance test against $$PERF_URL with $$PERF_CONCURRENCY concurrent clients for $$PERF_DURATION at rate $$PERF_RATE..."; \
	go build -o $(TMP_DIR)/$(PROJECT_NAME)-perftest cmd/tools/perftest/main.go
	./$(TMP_DIR)/$(PROJECT_NAME)-perftest -url="$$PERF_URL" -c $$PERF_CONCURRENCY -d $$PERF_DURATION -rate $$PERF_RATE

# Load testing
loadtest:
	@echo "Running production load tests..."
	@./scripts/run_production_loadtest.sh

loadtest-quick:
	@echo "Running quick load test (light scenario only)..."
	@SCENARIOS="light" ./scripts/run_production_loadtest.sh

loadtest-heavy:
	@echo "Running heavy load tests (heavy + extreme)..."
	@SCENARIOS="heavy extreme" ./scripts/run_production_loadtest.sh

# Build distribution (app only)
build: clean
	@echo "Building distribution..."
	@mkdir -p $(DIST_DIR)
	@$(eval export CGO_ENABLED=1)
	go build -o $(DIST_DIR)/$(BINARY_NAME) cmd/fusionaly/main.go
	go build -o $(DIST_DIR)/$(IMCTL_BINARY) cmd/fnctl/main.go
	@echo "Building web assets..."
	cd web && $(NPM) run build
	@echo "Build completed successfully."

# Build manager for current platform
build-manager:
	@echo "Building manager (version: $(VERSION))..."
	@mkdir -p $(DIST_DIR)
	$(GO) build -ldflags "-X main.currentManagerVersion=$(VERSION)" \
		-o $(DIST_DIR)/$(MANAGER_BINARY) ./cmd/manager/
	@echo "Manager built: $(DIST_DIR)/$(MANAGER_BINARY)"

# Build manager for Linux (cross-compile for servers)
build-manager-linux:
	@echo "Building manager for Linux (version: $(VERSION))..."
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build \
		-ldflags "-X main.currentManagerVersion=$(VERSION)" \
		-o $(DIST_DIR)/$(MANAGER_BINARY)-linux-amd64 ./cmd/manager/
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build \
		-ldflags "-X main.currentManagerVersion=$(VERSION)" \
		-o $(DIST_DIR)/$(MANAGER_BINARY)-linux-arm64 ./cmd/manager/
	@echo "Manager binaries built:"
	@ls -lh $(DIST_DIR)/$(MANAGER_BINARY)-linux-*

# Alias for build target (for compatibility with Dockerfile)
dist: build
	@echo "Distribution created in $(DIST_DIR) directory."

# Cleanup
clean:
	@echo "Cleaning up..."
	@if [ -f $(PID_FILE) ]; then \
		pid=$$(cat $(PID_FILE)); \
		if ps -p $$pid > /dev/null; then \
			echo "Killing server (PID: $$pid)..."; \
			kill -9 $$pid || true; \
		fi; \
		rm -f $(PID_FILE); \
	fi
	@rm -rf $(DIST_DIR) $(TMP_DIR)

update-ua-database:
	@echo "Updating User-Agent database..."
	@mkdir -p $(UA_DATABASE_DIR)/client/hints
	@mkdir -p $(UA_DATABASE_DIR)/device
	@echo "Downloading device-detector database from Matomo (version: $(DEVICE_DETECTOR_VERSION))..."
	@echo "Downloading root files..."
	@curl -L https://raw.githubusercontent.com/matomo-org/device-detector/$(DEVICE_DETECTOR_VERSION)/regexes/bots.yml -o $(UA_DATABASE_DIR)/bots.yml
	@curl -L https://raw.githubusercontent.com/matomo-org/device-detector/$(DEVICE_DETECTOR_VERSION)/regexes/oss.yml -o $(UA_DATABASE_DIR)/oss.yml
	@curl -L https://raw.githubusercontent.com/matomo-org/device-detector/$(DEVICE_DETECTOR_VERSION)/regexes/vendorfragments.yml -o $(UA_DATABASE_DIR)/vendorfragments.yml
	@echo "Downloading client files..."
	@curl -L https://raw.githubusercontent.com/matomo-org/device-detector/$(DEVICE_DETECTOR_VERSION)/regexes/client/browser_engine.yml -o $(UA_DATABASE_DIR)/client/browser_engine.yml
	@curl -L https://raw.githubusercontent.com/matomo-org/device-detector/$(DEVICE_DETECTOR_VERSION)/regexes/client/browsers.yml -o $(UA_DATABASE_DIR)/client/browsers.yml
	@curl -L https://raw.githubusercontent.com/matomo-org/device-detector/$(DEVICE_DETECTOR_VERSION)/regexes/client/feed_readers.yml -o $(UA_DATABASE_DIR)/client/feed_readers.yml
	@curl -L https://raw.githubusercontent.com/matomo-org/device-detector/$(DEVICE_DETECTOR_VERSION)/regexes/client/libraries.yml -o $(UA_DATABASE_DIR)/client/libraries.yml
	@curl -L https://raw.githubusercontent.com/matomo-org/device-detector/$(DEVICE_DETECTOR_VERSION)/regexes/client/mediaplayers.yml -o $(UA_DATABASE_DIR)/client/mediaplayers.yml
	@curl -L https://raw.githubusercontent.com/matomo-org/device-detector/$(DEVICE_DETECTOR_VERSION)/regexes/client/mobile_apps.yml -o $(UA_DATABASE_DIR)/client/mobile_apps.yml
	@curl -L https://raw.githubusercontent.com/matomo-org/device-detector/$(DEVICE_DETECTOR_VERSION)/regexes/client/pim.yml -o $(UA_DATABASE_DIR)/client/pim.yml
	@echo "Downloading client hints..."
	@curl -L https://raw.githubusercontent.com/matomo-org/device-detector/$(DEVICE_DETECTOR_VERSION)/regexes/client/hints/apps.yml -o $(UA_DATABASE_DIR)/client/hints/apps.yml
	@curl -L https://raw.githubusercontent.com/matomo-org/device-detector/$(DEVICE_DETECTOR_VERSION)/regexes/client/hints/browsers.yml -o $(UA_DATABASE_DIR)/client/hints/browsers.yml
	@echo "Downloading device files..."
	@curl -L https://raw.githubusercontent.com/matomo-org/device-detector/$(DEVICE_DETECTOR_VERSION)/regexes/device/cameras.yml -o $(UA_DATABASE_DIR)/device/cameras.yml
	@curl -L https://raw.githubusercontent.com/matomo-org/device-detector/$(DEVICE_DETECTOR_VERSION)/regexes/device/car_browsers.yml -o $(UA_DATABASE_DIR)/device/car_browsers.yml
	@curl -L https://raw.githubusercontent.com/matomo-org/device-detector/$(DEVICE_DETECTOR_VERSION)/regexes/device/consoles.yml -o $(UA_DATABASE_DIR)/device/consoles.yml
	@curl -L https://raw.githubusercontent.com/matomo-org/device-detector/$(DEVICE_DETECTOR_VERSION)/regexes/device/mobiles.yml -o $(UA_DATABASE_DIR)/device/mobiles.yml
	@curl -L https://raw.githubusercontent.com/matomo-org/device-detector/$(DEVICE_DETECTOR_VERSION)/regexes/device/notebooks.yml -o $(UA_DATABASE_DIR)/device/notebooks.yml
	@curl -L https://raw.githubusercontent.com/matomo-org/device-detector/$(DEVICE_DETECTOR_VERSION)/regexes/device/portable_media_player.yml -o $(UA_DATABASE_DIR)/device/portable_media_player.yml
	@curl -L https://raw.githubusercontent.com/matomo-org/device-detector/$(DEVICE_DETECTOR_VERSION)/regexes/device/shell_tv.yml -o $(UA_DATABASE_DIR)/device/shell_tv.yml
	@curl -L https://raw.githubusercontent.com/matomo-org/device-detector/$(DEVICE_DETECTOR_VERSION)/regexes/device/televisions.yml -o $(UA_DATABASE_DIR)/device/televisions.yml
	@echo "User-Agent database updated successfully."

# Download test fixtures from Matomo device-detector
download-test-fixtures:
	@echo "Downloading test fixtures from Matomo device-detector..."
	@mkdir -p fixtures/user_agent
	@curl -L https://raw.githubusercontent.com/matomo-org/device-detector/master/Tests/fixtures/desktop.yml -o fixtures/user_agent/desktop.yml
	@curl -L https://raw.githubusercontent.com/matomo-org/device-detector/master/Tests/fixtures/smartphone.yml -o fixtures/user_agent/smartphone.yml
	@curl -L https://raw.githubusercontent.com/matomo-org/device-detector/master/Tests/fixtures/tablet.yml -o fixtures/user_agent/tablet.yml
	@curl -L https://raw.githubusercontent.com/matomo-org/device-detector/master/Tests/fixtures/bots.yml -o fixtures/user_agent/bots.yml
	@echo "Test fixtures downloaded successfully"

# Run comprehensive user agent parsing tests
test-ua-parser:
	@echo "Running user agent parser tests..."
	@go test -v ./tests/internal/pkg/user_agent/ -run TestSimplePatterns
	@echo "User agent parser tests completed"

# Run strict Matomo fixture tests (separate from main test suite)
test-ua-fixtures:
	@echo "Running strict Matomo fixture tests..."
	@go test -v -tags matomo_fixtures ./tests/internal/pkg/user_agent/ -run TestMatomoFixtures
	@echo "Matomo fixture tests completed"
