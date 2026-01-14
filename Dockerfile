# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies (build-base includes gcc, g++, make)
RUN apk add --no-cache build-base musl-dev git nodejs npm

WORKDIR /app

# Cache Go dependencies (invalidates only when go.mod/go.sum changes)
COPY go.mod go.sum ./
RUN go mod download

# Cache npm dependencies (invalidates only when package.json changes)
COPY web/package*.json web/package-lock.json ./web/
RUN cd web && npm ci --legacy-peer-deps --production=false

# Copy only necessary source files (excluding web/node_modules)
COPY Makefile ./
COPY cmd ./cmd
COPY internal ./internal
COPY api ./api
COPY web/src ./web/src
COPY web/public ./web/public
COPY web/index.html web/vite.config.ts web/tsconfig*.json web/postcss.config.js web/tailwind.config.ts ./web/

# Build binaries and web assets
RUN mkdir -p dist && \
  CGO_ENABLED=1 go build -o dist/fusionaly-server cmd/fusionaly/main.go && \
  CGO_ENABLED=1 go build -o dist/fnctl cmd/fnctl/main.go && \
  cd web && npm run build

# Final stage - minimal runtime image
FROM alpine:3.19
WORKDIR /app

# Install runtime dependencies and create directories in one layer
RUN apk add --no-cache ca-certificates tzdata sqlite curl && \
  mkdir -p /app/logs /app/storage /app/web/dist /app/internal-storage

# Copy compiled binaries and built assets from builder
COPY --from=builder /app/dist/fusionaly-server /app/fusionaly-server
COPY --from=builder /app/dist/fnctl /app/fnctl
COPY --from=builder /app/web/dist /app/web/dist

# GeoLite2 database is optional - mount at runtime:
# -v /path/to/GeoLite2-City.mmdb:/app/internal-storage/GeoLite2-City.mmdb

# Copy entrypoint script
COPY scripts/entrypoint.sh /app/entrypoint.sh

# Set permissions in one RUN command
RUN chmod +x /app/entrypoint.sh /app/fusionaly-server /app/fnctl
ENV FUSIONALY_ENV=production \
  FUSIONALY_STORAGE_PATH=/app/storage \
  FUSIONALY_GEO_DB_PATH=/app/internal-storage/GeoLite2-City.mmdb \
  FUSIONALY_PUBLIC_DIR=/app/web/dist \
  FUSIONALY_LOGS_DIR=/app/logs \
  FUSIONALY_APP_PORT=8080

EXPOSE ${FUSIONALY_APP_PORT}

# Health check using curl
# -f flag makes curl fail on HTTP errors (4xx, 5xx)
# Using shell form to ensure environment variable expansion
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD /bin/sh -c "curl -f http://localhost:${FUSIONALY_APP_PORT}/_health || exit 1"

ENTRYPOINT ["/app/entrypoint.sh"]
