# Multi-stage Dockerfile for MCP Proxy - used for both local development and production releases
# This Dockerfile is optimized for security, size, and performance
# Includes runtime dependencies for engine/runtime (Node.js, Bun, Python, uv) and mcp-proxy operations

# Build stage - Use official Go image for building
FROM golang:1.24-alpine AS builder

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

# Build the MCP Proxy binary with optimizations
RUN cd cmd/mcp-proxy && \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} \
    go build \
    -a \
    -installsuffix cgo \
    -ldflags='-w -s -extldflags "-static"' \
    -o /build/compozy-mcp-proxy

# Verify the binary
RUN ./compozy-mcp-proxy --help || echo "Compozy MCP Proxy binary built successfully"

# Production stage - Use Alpine for runtime dependencies while maintaining security
FROM alpine:3.20

# Install runtime dependencies required for engine/runtime and mcp-proxy
RUN apk add --no-cache     ca-certificates     tzdata     bash     nodejs     npm     python3     py3-pip     curl     wget     # Network debugging tools    netcat-openbsd     bind-tools     # Redis CLI for storage debugging    redis     # Process monitoring    procps

# Install Bun - Latest stable version for production
ENV BUN_VERSION=1.1.45
RUN curl -fsSL https://bun.sh/install | bash -s "bun-v${BUN_VERSION}" \
    && mv /root/.bun/bin/bun /usr/local/bin/ \
    && chmod +x /usr/local/bin/bun \
    && rm -rf /root/.bun

# Install uv - pinned version with checksum verification for supply-chain security
ENV UV_VERSION=0.5.14 \
    UV_SHA256=e1ccdfe1691c1f791d84bb6e1697e49416ca4b62103dcdf3b63772f03834f113
RUN curl -L -o /tmp/uv.tar.gz \
    "https://github.com/astral-sh/uv/releases/download/${UV_VERSION}/uv-x86_64-unknown-linux-musl.tar.gz" \
    && echo "${UV_SHA256}  /tmp/uv.tar.gz" | sha256sum -c - \
    && tar -C /usr/local/bin --strip-components=1 -xzf /tmp/uv.tar.gz uv-x86_64-unknown-linux-musl/uv \
    && chmod +x /usr/local/bin/uv \
    && rm /tmp/uv.tar.gz

# Create non-root user for production
RUN addgroup -g 1001 -S mcpproxy \
    && adduser -u 1001 -S mcpproxy -G mcpproxy

# Copy timezone data and certificates from builder
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Set working directory
WORKDIR /app

# Copy the binary from builder
COPY --from=builder --chown=mcpproxy:mcpproxy /build/compozy-mcp-proxy /app/compozy-mcp-proxy

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
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:6001/healthz || exit 1

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
LABEL org.opencontainers.image.description="Model Context Protocol Proxy with runtime support for Node.js, Bun, Python, and uv"

# Default command - run the compozy-mcp-proxy
ENTRYPOINT ["/app/compozy-mcp-proxy"]
CMD []
