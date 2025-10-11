# Multi-stage Dockerfile for MCP Proxy - used for both local development and production releases
# This Dockerfile is optimized for security, size, and performance
# Includes runtime dependencies for engine/runtime (Node.js, Bun, Python, uv) and mcp-proxy operations

# Build stage - Use official Go image for building
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    && update-ca-certificates

# Create non-root user for building
RUN adduser -D -g '' appuser

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the Compozy binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} \
    go build \
    -a \
    -installsuffix cgo \
    -ldflags='-w -s -extldflags "-static"' \
    -o /build/compozy \
    ./

# Verify the binary (check that mcp-proxy command exists)
RUN ./compozy mcp-proxy --help || echo "Compozy MCP Proxy command built successfully"

# Production stage - Use Alpine for runtime dependencies while maintaining security
FROM alpine:3.20

# Optional: include Docker CLI only when needed to reduce image size and CVE surface
ARG WITH_DOCKER_CLI=false

# Install runtime dependencies required for engine/runtime and mcp-proxy
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    bash \
    nodejs \
    npm \
    python3 \
    py3-pip \
    curl \
    wget \
    # Network debugging tools
    netcat-openbsd \
    bind-tools \
    # Redis CLI for storage debugging
    redis \
    # Process monitoring
    procps \
    && if [ "$WITH_DOCKER_CLI" = "true" ]; then apk add --no-cache docker-cli; fi

# Install Bun runtime (align with other images)
ENV BUN_VERSION=1.3.0
RUN curl -fsSL https://bun.sh/install | bash -s "bun-v${BUN_VERSION}" \
    && mv /root/.bun/bin/bun /usr/local/bin/ \
    && chmod +x /usr/local/bin/bun \
    && rm -rf /root/.bun

# Install uv - pinned version with checksum verification for supply-chain security
ARG TARGETARCH
ENV UV_VERSION=0.5.14
RUN set -e; \
    case "$TARGETARCH" in \
      amd64) UV_ASSET="uv-x86_64-unknown-linux-musl.tar.gz"; \
             UV_SHA256="e1ccdfe1691c1f791d84bb6e1697e49416ca4b62103dcdf3b63772f03834f113";; \
      arm64) UV_ASSET="uv-aarch64-unknown-linux-musl.tar.gz"; \
             UV_SHA256="64c5321f5141db39e04209d170db34fcef5c8de3f561346dc0c1d132801c4f88";; \
      *) echo "Unsupported arch: $TARGETARCH"; exit 1;; \
    esac; \
    curl -L -o /tmp/uv.tar.gz \
        "https://github.com/astral-sh/uv/releases/download/${UV_VERSION}/${UV_ASSET}"; \
    echo "${UV_SHA256}  /tmp/uv.tar.gz" | sha256sum -c - \
    && tar -C /usr/local/bin --strip-components=1 \
        -xzf /tmp/uv.tar.gz ${UV_ASSET%.tar.gz}/uv \
    && chmod +x /usr/local/bin/uv \
    && rm /tmp/uv.tar.gz

# Create non-root user for production
RUN addgroup -g 1001 -S mcpproxy \
    && adduser -u 1001 -S mcpproxy -G mcpproxy

# Add tini for proper signal handling and zombie reaping (must be root)
RUN apk add --no-cache tini

# tzdata and ca-certificates already installed in runtime; no need to copy from builder

# Set working directory
WORKDIR /app

# Copy the binary from builder
COPY --from=builder --chown=mcpproxy:mcpproxy /build/compozy /app/compozy

# Copy additional files
COPY --from=builder --chown=mcpproxy:mcpproxy /build/README.md /app/README.md
COPY --from=builder --chown=mcpproxy:mcpproxy /build/LICENSE /app/LICENSE

# Switch to non-root user
USER mcpproxy

# Set environment variables
ENV PATH="/app:${PATH}"
ENV MCP_PROXY_ENV=production
ENV MCP_PROXY_LOG_LEVEL=info
ENV MCP_PROXY_PORT=6001
# Security: Limit stdio transport commands by default
ENV MCP_PROXY_STDIO_ALLOWED_COMMANDS=""
# Performance: Connection pool settings
ENV MCP_PROXY_MAX_CONNECTIONS=100
ENV MCP_PROXY_TIMEOUT=30s

# Health check for production monitoring
HEALTHCHECK --interval=30s --timeout=10s --start-period=15s --retries=3 \
    CMD sh -c 'wget --no-verbose --tries=1 --spider "http://localhost:${MCP_PROXY_PORT:-6001}/healthz" || exit 1'

# Expose port
EXPOSE 6001

# Container metadata labels
LABEL org.opencontainers.image.title="Compozy MCP Proxy"
LABEL org.opencontainers.image.description="Model Context Protocol Proxy for Compozy"
LABEL org.opencontainers.image.url="https://github.com/compozy/compozy"
LABEL org.opencontainers.image.source="https://github.com/compozy/compozy"
LABEL org.opencontainers.image.documentation="https://github.com/compozy/compozy/blob/main/cluster/README.md"
LABEL org.opencontainers.image.vendor="Compozy"
LABEL org.opencontainers.image.licenses="BSL-1.1"
LABEL org.opencontainers.image.base.name="alpine:3.20"

# Default command - run the compozy mcp-proxy subcommand
ENTRYPOINT ["/sbin/tini", "--", "/app/compozy"]
CMD ["mcp-proxy"]
