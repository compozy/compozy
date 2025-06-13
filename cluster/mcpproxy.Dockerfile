# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the mcp-proxy binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o mcp-proxy \
    .

# Final stage
FROM alpine:3.20

# Install ca-certificates for HTTPS requests and Node.js/npm and Python/uv
RUN apk --no-cache add ca-certificates tzdata nodejs npm python3 py3-pip curl

# Install uv - pinned version with checksum verification for supply-chain security
ENV UV_VERSION=0.7.13 \
    UV_SHA256=560bb64e060354e45138d7dd47c8dd48a4f7a349af5520d29cd3c704e79f286c
RUN curl -L -o /tmp/uv.tar.gz \
      "https://github.com/astral-sh/uv/releases/download/${UV_VERSION}/uv-x86_64-unknown-linux-musl.tar.gz" && \
    echo "${UV_SHA256}  /tmp/uv.tar.gz" | sha256sum -c - && \
    tar -C /usr/local/bin --strip-components=1 -xzf /tmp/uv.tar.gz && \
    chmod +x /usr/local/bin/uv && \
    rm /tmp/uv.tar.gz

# Create non-root user
RUN addgroup -g 1001 -S mcpproxy && \
    adduser -u 1001 -S mcpproxy -G mcpproxy

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/mcp-proxy .

# Change ownership to non-root user
RUN chown mcpproxy:mcpproxy /app/mcp-proxy

# Switch to non-root user
USER mcpproxy

# Expose port
EXPOSE 8081

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8081/healthz || exit 1

# Set default command
CMD ["./mcp-proxy", "mcp-proxy"]
