# Multi-stage Dockerfile for production Compozy deployments
# This Dockerfile includes all runtime dependencies and is optimized for production use

# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    && update-ca-certificates

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download and cache dependencies
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download && go mod verify

# Copy source code
COPY . .

# Build arguments for version injection
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE

# Build the binary with optimizations
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} \
    go build -a -installsuffix cgo \
    -ldflags="-w -s -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildDate=${BUILD_DATE}" \
    -o compozy \
    ./

# Runtime stage - Alpine-based for Bun support
FROM alpine:3.20

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    bash \
    # For process management
    tini \
    # For debugging in production
    htop \
    procps

# Install Bun runtime (required for TypeScript tool execution)
ENV BUN_VERSION=1.3.0
RUN curl -fsSL https://bun.sh/install | bash -s "bun-v${BUN_VERSION}" \
    && mv /root/.bun/bin/bun /usr/local/bin/ \
    && chmod +x /usr/local/bin/bun \
    && rm -rf /root/.bun

# Create non-root user
RUN addgroup -g 1001 -S compozy \
    && adduser -u 1001 -S compozy -G compozy

# Create necessary directories
RUN mkdir -p /app/config /app/data /app/tools /app/logs /app/tmp /app/docs \
    && chown -R compozy:compozy /app

# Copy timezone data and certificates from builder
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary from builder
COPY --from=builder --chown=compozy:compozy /build/compozy /usr/local/bin/compozy
COPY --from=builder --chown=compozy:compozy /build/docs /app/docs

# Note: Configuration should be provided at runtime via volume mount or ConfigMap

# Switch to non-root user
USER compozy

# Set working directory
WORKDIR /app

# Environment variables with sensible defaults
ENV PATH="/usr/local/bin:${PATH}"
ENV RUNTIME_ENVIRONMENT=${RUNTIME_ENVIRONMENT:-production}
ENV RUNTIME_LOG_LEVEL=${RUNTIME_LOG_LEVEL:-info}
ENV COMPOZY_CONFIG_FILE=${COMPOZY_CONFIG_FILE:-/app/config/compozy.yaml}

# Server Configuration
ENV SERVER_HOST=${SERVER_HOST:-0.0.0.0}
ENV SERVER_PORT=${SERVER_PORT:-5001}
ENV SERVER_TIMEOUT=${SERVER_TIMEOUT:-30s}
ENV SERVER_CORS_ENABLED=${SERVER_CORS_ENABLED:-true}
ENV SERVER_CORS_ALLOWED_ORIGINS=${SERVER_CORS_ALLOWED_ORIGINS:-["http://localhost:3000","http://localhost:5001"]}
ENV SERVER_CORS_ALLOW_CREDENTIALS=${SERVER_CORS_ALLOW_CREDENTIALS:-true}
ENV SERVER_CORS_MAX_AGE=${SERVER_CORS_MAX_AGE:-86400}
ENV SERVER_AUTH_ENABLED=${SERVER_AUTH_ENABLED:-false}
ENV SERVER_AUTH_WORKFLOW_EXCEPTIONS=${SERVER_AUTH_WORKFLOW_EXCEPTIONS:-[]}

# Database Configuration (Required)
ENV DB_CONN_STRING=""
ENV DB_HOST=${DB_HOST:-localhost}
ENV DB_PORT=${DB_PORT:-5432}
ENV DB_USER=${DB_USER:-postgres}
ENV DB_PASSWORD=""
ENV DB_NAME=${DB_NAME:-compozy}
ENV DB_SSL_MODE=${DB_SSL_MODE:-require}

# Redis Configuration (Required)
ENV REDIS_URL=""
ENV REDIS_HOST=${REDIS_HOST:-localhost}
ENV REDIS_PORT=${REDIS_PORT:-6379}
ENV REDIS_PASSWORD=""
ENV REDIS_DB=${REDIS_DB:-0}
ENV REDIS_MAX_RETRIES=${REDIS_MAX_RETRIES:-3}
ENV REDIS_POOL_SIZE=${REDIS_POOL_SIZE:-10}
ENV REDIS_DIAL_TIMEOUT=${REDIS_DIAL_TIMEOUT:-5s}
ENV REDIS_READ_TIMEOUT=${REDIS_READ_TIMEOUT:-3s}
ENV REDIS_WRITE_TIMEOUT=${REDIS_WRITE_TIMEOUT:-3s}
ENV REDIS_POOL_TIMEOUT=${REDIS_POOL_TIMEOUT:-4s}
ENV REDIS_TLS_ENABLED=${REDIS_TLS_ENABLED:-false}

# Temporal Configuration (Required)
ENV TEMPORAL_HOST_PORT=${TEMPORAL_HOST_PORT:-localhost:7233}
ENV TEMPORAL_NAMESPACE=${TEMPORAL_NAMESPACE:-default}
ENV TEMPORAL_TASK_QUEUE=${TEMPORAL_TASK_QUEUE:-compozy-tasks}

# Runtime Configuration
ENV RUNTIME_DISPATCHER_HEARTBEAT_INTERVAL=${RUNTIME_DISPATCHER_HEARTBEAT_INTERVAL:-30s}
ENV RUNTIME_DISPATCHER_HEARTBEAT_TTL=${RUNTIME_DISPATCHER_HEARTBEAT_TTL:-90s}
ENV RUNTIME_DISPATCHER_STALE_THRESHOLD=${RUNTIME_DISPATCHER_STALE_THRESHOLD:-120s}
ENV RUNTIME_ASYNC_TOKEN_COUNTER_WORKERS=${RUNTIME_ASYNC_TOKEN_COUNTER_WORKERS:-4}
ENV RUNTIME_ASYNC_TOKEN_COUNTER_BUFFER_SIZE=${RUNTIME_ASYNC_TOKEN_COUNTER_BUFFER_SIZE:-100}
ENV TOOL_EXECUTION_TIMEOUT=${TOOL_EXECUTION_TIMEOUT:-60s}
ENV RUNTIME_TYPE=${RUNTIME_TYPE:-bun}
ENV RUNTIME_ENTRYPOINT_PATH=${RUNTIME_ENTRYPOINT_PATH:-./tools.ts}
ENV RUNTIME_BUN_PERMISSIONS=${RUNTIME_BUN_PERMISSIONS:-["--allow-read"]}

# System Limits
ENV LIMITS_MAX_NESTING_DEPTH=${LIMITS_MAX_NESTING_DEPTH:-20}
ENV LIMITS_MAX_STRING_LENGTH=${LIMITS_MAX_STRING_LENGTH:-10485760}
ENV LIMITS_MAX_MESSAGE_CONTENT_LENGTH=${LIMITS_MAX_MESSAGE_CONTENT_LENGTH:-10240}
ENV LIMITS_MAX_TOTAL_CONTENT_SIZE=${LIMITS_MAX_TOTAL_CONTENT_SIZE:-102400}
ENV LIMITS_MAX_TASK_CONTEXT_DEPTH=${LIMITS_MAX_TASK_CONTEXT_DEPTH:-5}
ENV LIMITS_PARENT_UPDATE_BATCH_SIZE=${LIMITS_PARENT_UPDATE_BATCH_SIZE:-100}

# Memory Service
ENV MEMORY_PREFIX=${MEMORY_PREFIX:-compozy:memory:}
ENV MEMORY_TTL=${MEMORY_TTL:-24h}
ENV MEMORY_MAX_ENTRIES=${MEMORY_MAX_ENTRIES:-10000}

# MCP Proxy Configuration
ENV MCP_PROXY_URL=${MCP_PROXY_URL:-http://localhost:6001}
ENV MCP_PROXY_HOST=${MCP_PROXY_HOST:-127.0.0.1}
ENV MCP_PROXY_PORT=${MCP_PROXY_PORT:-6001}
ENV MCP_PROXY_BASE_URL=""
ENV MCP_PROXY_SHUTDOWN_TIMEOUT=${MCP_PROXY_SHUTDOWN_TIMEOUT:-10s}

# Rate Limiting
ENV RATELIMIT_GLOBAL_LIMIT=${RATELIMIT_GLOBAL_LIMIT:-100}
ENV RATELIMIT_GLOBAL_PERIOD=${RATELIMIT_GLOBAL_PERIOD:-1m}
ENV RATELIMIT_API_KEY_LIMIT=${RATELIMIT_API_KEY_LIMIT:-100}
ENV RATELIMIT_API_KEY_PERIOD=${RATELIMIT_API_KEY_PERIOD:-1m}
ENV RATELIMIT_PREFIX=${RATELIMIT_PREFIX:-compozy:ratelimit:}
ENV RATELIMIT_MAX_RETRY=${RATELIMIT_MAX_RETRY:-3}

# Cache Configuration
ENV CACHE_ENABLED=${CACHE_ENABLED:-true}
ENV CACHE_TTL=${CACHE_TTL:-24h}
ENV CACHE_PREFIX=${CACHE_PREFIX:-compozy:cache:}
ENV CACHE_MAX_ITEM_SIZE=${CACHE_MAX_ITEM_SIZE:-1048576}
ENV CACHE_COMPRESSION_ENABLED=${CACHE_COMPRESSION_ENABLED:-true}
ENV CACHE_COMPRESSION_THRESHOLD=${CACHE_COMPRESSION_THRESHOLD:-1024}
ENV CACHE_EVICTION_POLICY=${CACHE_EVICTION_POLICY:-lru}
ENV CACHE_STATS_INTERVAL=${CACHE_STATS_INTERVAL:-5m}

# Worker Configuration
ENV WORKER_CONFIG_STORE_TTL=${WORKER_CONFIG_STORE_TTL:-24h}
ENV WORKER_HEARTBEAT_CLEANUP_TIMEOUT=${WORKER_HEARTBEAT_CLEANUP_TIMEOUT:-5s}
ENV WORKER_MCP_SHUTDOWN_TIMEOUT=${WORKER_MCP_SHUTDOWN_TIMEOUT:-30s}
ENV WORKER_DISPATCHER_RETRY_DELAY=${WORKER_DISPATCHER_RETRY_DELAY:-50ms}
ENV WORKER_DISPATCHER_MAX_RETRIES=${WORKER_DISPATCHER_MAX_RETRIES:-2}
ENV WORKER_MCP_PROXY_HEALTH_CHECK_TIMEOUT=${WORKER_MCP_PROXY_HEALTH_CHECK_TIMEOUT:-10s}

# CLI Configuration (for when using CLI from container)
ENV COMPOZY_API_KEY=""
ENV COMPOZY_BASE_URL=${COMPOZY_BASE_URL:-http://localhost:5001}
ENV COMPOZY_TIMEOUT=${COMPOZY_TIMEOUT:-30s}
ENV COMPOZY_CLI_MODE=${COMPOZY_CLI_MODE:-auto}
ENV COMPOZY_DEBUG=${COMPOZY_DEBUG:-false}

# Define volumes for persistence
VOLUME ["/app/config", "/app/data", "/app/tools", "/app/logs"]

# Health check with proper endpoint
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD ["/usr/local/bin/compozy", "api", "health"]

# Expose API port
EXPOSE 5001

# Container metadata labels
LABEL org.opencontainers.image.title="Compozy" \
      org.opencontainers.image.description="Next-level Agentic Orchestration Platform" \
      org.opencontainers.image.url="https://github.com/compozy/compozy" \
      org.opencontainers.image.source="https://github.com/compozy/compozy" \
      org.opencontainers.image.documentation="https://github.com/compozy/compozy/blob/main/docs/deployment.md" \
      org.opencontainers.image.vendor="Compozy" \
      org.opencontainers.image.licenses="BSL-1.1"

# Use tini for proper signal handling and process reaping
ENTRYPOINT ["/sbin/tini", "--"]

# Default command - start the API server
CMD ["/usr/local/bin/compozy", "server"]
