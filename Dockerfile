# ==============================================================================
# STAGE 1: Build Frontend (SvelteKit)
# ==============================================================================
FROM node:22-alpine3.22 AS frontend-builder

WORKDIR /app/web

# Copy package files and install dependencies
COPY web/package*.json ./
RUN npm ci

# Copy frontend source code
COPY web/ ./

# Build frontend to static assets (output written to web/build)
RUN npm run build

# ==============================================================================
# STAGE 2: Build Backend (Go)
# ==============================================================================
FROM golang:1.26.8-alpine3.23 AS backend-builder

WORKDIR /app

# Pure-Go SQLite driver (modernc.org/sqlite): no C toolchain needed.
COPY go.mod go.sum ./
RUN go mod download

# Copy backend source code (migrations are embedded from internal/db/migrations)
COPY cmd/ ./cmd
COPY internal/ ./internal

# Compile Go server into an optimized static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server ./cmd/server

# ==============================================================================
# STAGE 3: Final Runner (Minimal Production Image)
# ==============================================================================
FROM alpine:3.22 AS runner

WORKDIR /app

# ca-certificates for outbound TLS; the SQLite engine is compiled into the binary.
RUN apk add --no-cache ca-certificates \
    && addgroup -S app && adduser -S -G app -u 10001 app

# Copy compiled backend binary
COPY --from=backend-builder /app/server ./server

# Copy pre-compiled static frontend assets into SvelteKit serving directory
COPY --from=frontend-builder /app/web/build ./web/build

# Data directory (SQLite database, queue, extracted packages) owned by the runtime user
RUN mkdir -p /app/data && chown -R app:app /app/data

# Expose Fiber default port
EXPOSE 3000

# Set environment defaults (can be overridden in deployment environment variables)
ENV PORT=3000
ENV DATABASE_URL=data/cbt_aether.db
ENV ENV=production

USER app

VOLUME ["/app/data"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD wget -qO- http://127.0.0.1:${PORT:-3000}/api/health >/dev/null || exit 1

# Run the unified Go server serving both frontend and backend
CMD ["./server"]
