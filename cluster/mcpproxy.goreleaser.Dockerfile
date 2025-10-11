# GoReleaser-compatible Dockerfile for MCP Proxy
# This Dockerfile expects the pre-built binary from GoReleaser

# Production stage - Use Alpine for runtime dependencies while maintaining security
FROM alpine:3.20

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
    procps

# Install Bun - Latest stable version for production
ENV BUN_VERSION=1.3.0
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

# Set working directory
WORKDIR /app

# Copy the pre-built binary from GoReleaser
COPY compozy /app/compozy
RUN chmod +x /app/compozy

# Copy additional files if they exist
COPY README.md /app/README.md
COPY LICENSE /app/LICENSE

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

# Default command - run the compozy mcp-proxy subcommand
ENTRYPOINT ["/app/compozy"]
CMD ["mcp-proxy"]
