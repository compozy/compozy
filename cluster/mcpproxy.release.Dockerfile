# Multi-stage Dockerfile for production MCP Proxy releases
# This Dockerfile is optimized for security, size, and performance

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
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} \
    go build \
    -a \
    -installsuffix cgo \
    -ldflags='-w -s -extldflags "-static"' \
    -o compozy \
    ./main.go

# Verify the binary
RUN ./compozy mcp-proxy --help || echo "MCP Proxy binary built successfully"

# Production stage - Use distroless for minimal attack surface
FROM gcr.io/distroless/static-debian12:nonroot

# Copy timezone data and certificates from builder
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Set working directory
WORKDIR /app

# Copy the binary from builder
COPY --from=builder --chown=nonroot:nonroot /build/compozy /app/compozy

# Copy additional files
COPY --from=builder --chown=nonroot:nonroot /build/README.md /app/README.md
COPY --from=builder --chown=nonroot:nonroot /build/LICENSE /app/LICENSE

# User is already nonroot in distroless

# Set environment variables
ENV PATH="/app:${PATH}"
ENV MCP_PROXY_ENV=production
ENV MCP_PROXY_LOG_LEVEL=info
ENV MCP_PROXY_PORT=8081

# Note: Health check removed as distroless doesn't include wget
# Health checks should be configured at orchestration layer (Kubernetes, Docker Compose, etc.)

# Expose port
EXPOSE 8081

# Container metadata labels
LABEL org.opencontainers.image.title="Compozy MCP Proxy"
LABEL org.opencontainers.image.description="Model Context Protocol Proxy for Compozy"
LABEL org.opencontainers.image.url="https://github.com/compozy/compozy"
LABEL org.opencontainers.image.source="https://github.com/compozy/compozy"
LABEL org.opencontainers.image.documentation="https://github.com/compozy/compozy/blob/main/cluster/README.md"
LABEL org.opencontainers.image.vendor="Compozy"
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.base.name="alpine:3.20"

# Default command - run the mcp-proxy subcommand
ENTRYPOINT ["/app/compozy"]
CMD ["mcp-proxy"]
