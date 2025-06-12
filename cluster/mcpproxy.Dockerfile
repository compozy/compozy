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

# Install uv (fast Python package manager)
RUN curl -LsSf https://astral.sh/uv/install.sh | sh

# Find and copy uv binary to user accessible location
RUN find /root -name "uv" -type f -executable 2>/dev/null | head -1 | xargs -I {} cp {} /usr/local/bin/uv && \
    chmod +x /usr/local/bin/uv

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
