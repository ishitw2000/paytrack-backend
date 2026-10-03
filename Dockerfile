# ==========================================
# Stage 1: Build the Go binary
# ==========================================
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates tzdata

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o paytrack-server ./cmd/server

# ==========================================
# Stage 2: Minimal runtime image
# ==========================================
FROM alpine:3.20

WORKDIR /app

# Install ca-certificates (for TLS/IMAP) and tzdata (for cron timezones)
RUN apk add --no-cache ca-certificates tzdata curl && \
    addgroup -g 10001 -S appgroup && \
    adduser -u 10001 -S appuser -G appgroup

# Create directory for persistent SQLite database with proper permissions
RUN mkdir -p /app/data && chown -R appuser:appgroup /app

# Copy compiled binary from builder
COPY --from=builder --chown=appuser:appgroup /build/paytrack-server /app/paytrack-server

# Switch to non-root user
USER appuser

# Expose HTTP API port
EXPOSE 8080

# Environment defaults
ENV PORT=8080
ENV DB_PATH=/app/data/paytrack.db
ENV CRON_SCHEDULE="0 0,6,12,18 * * *"
ENV CRON_ENABLED=true
ENV LLM_ENABLED=true
ENV OLLAMA_BASE_URL=http://ollama:11434
ENV OLLAMA_MODEL=llama3.2

# Health check endpoint
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/paytrack-server"]
