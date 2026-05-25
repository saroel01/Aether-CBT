# ==============================================================================
# STAGE 1: Build Frontend (SvelteKit)
# ==============================================================================
FROM node:18-alpine AS frontend-builder
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
FROM golang:1.22-alpine AS backend-builder
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Copy go mod files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy backend source code
COPY cmd/ ./cmd
COPY internal/ ./internal

# Compile Go server into an optimized static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server cmd/server/main.go

# ==============================================================================
# STAGE 3: Final Runner (Minimal Production Image)
# ==============================================================================
FROM alpine:latest AS runner
WORKDIR /app

# Install runtime dependencies (sqlite & ca-certificates for webhook SSL requests)
RUN apk add --no-cache ca-certificates sqlite

# Copy compiled backend binary
COPY --from=backend-builder /app/server ./server

# Copy pre-compiled static frontend assets into SvelteKit serving directory
COPY --from=frontend-builder /app/web/build ./web/build

# Create data directory for SQLite database storage
RUN mkdir -p ./data

# Expose Fiber default port
EXPOSE 3000

# Set environment defaults (can be overridden in Coolify environment variables)
ENV PORT=3000
ENV DATABASE_URL=data/cbt_aether.db
ENV JWT_SECRET=supersecurejwtkey2026

# Run the unified Go server serving both frontend and backend
CMD ["./server"]
