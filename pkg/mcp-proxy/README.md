# `mcp-proxy` – _Lightweight, secure Model-Context-Protocol gateway_

> **A powerful MCP proxy that exposes multiple remote (or local) MCP servers through a single, unified HTTP interface with comprehensive administration, observability, and security features.**

---

## 📑 Table of Contents

- [`mcp-proxy` – _Lightweight, secure Model-Context-Protocol gateway_](#mcp-proxy--lightweight-secure-model-context-protocol-gateway)
  - [📑 Table of Contents](#-table-of-contents)
  - [🎯 Overview](#-overview)
  - [💡 Motivation](#-motivation)
  - [⚡ Design Highlights](#-design-highlights)
  - [🚀 Getting Started](#-getting-started)
  - [📖 Usage](#-usage)
    - [Basic Proxy Setup](#basic-proxy-setup)
    - [Admin API](#admin-api)
    - [Proxy Endpoints](#proxy-endpoints)
  - [🔧 Configuration](#-configuration)
    - [Server Configuration](#server-configuration)
    - [MCP Definition Schema](#mcp-definition-schema)
  - [Storage Options](#storage-options)
  - [Mode-Aware Defaults](#mode-aware-defaults)
  - [🎨 Examples](#-examples)
    - [Complete Production Setup](#complete-production-setup)
    - [Compozy Engine Integration](#compozy-engine-integration)
    - [Workflow YAML Integration](#workflow-yaml-integration)
  - [📚 API Reference](#-api-reference)
    - [Core Types](#core-types)
    - [HTTP API Endpoints](#http-api-endpoints)
      - [System Endpoints](#system-endpoints)
      - [Admin API (requires authentication)](#admin-api-requires-authentication)
      - [Proxy Endpoints](#proxy-endpoints-1)
    - [Factory Functions](#factory-functions)
  - [🧪 Testing](#-testing)
  - [📦 Contributing](#-contributing)
  - [📄 License](#-license)

---

## 🎯 Overview

The `mcp-proxy` package provides a lightweight, secure **Model-Context-Protocol (MCP) gateway** that exposes multiple remote (or local) MCP servers through a single, unified HTTP interface. It acts as a reverse proxy, connection manager, and administration hub for MCP servers.

The proxy supports multiple transport protocols (stdio, SSE, streamable-http), provides dynamic server registration, built-in security features, and comprehensive observability with metrics and health checks.

---

## 💡 Motivation

- **Centralized Management**: Single point of control for multiple MCP servers
- **Security**: Built-in authentication, authorization, and IP filtering
- **Observability**: Health checks, metrics, and connection monitoring
- **Flexibility**: Support for multiple transport protocols and dynamic configuration
- **Integration**: Seamless integration with the Compozy engine ecosystem

---

## ⚡ Design Highlights

- **Multi-Transport Support**: Handles stdio, SSE, and streamable-http protocols
- **Dynamic Registration**: Hot-add, update, or remove servers via Admin API
- **Pluggable Storage**: In-memory (default) or Redis persistence
- **Auto-Reconnection**: Automatic connection recovery with configurable back-off
- **Tool Discovery**: Aggregates and exposes tools from all registered MCP servers
- **Thread-Safe**: Safe for concurrent use across multiple goroutines

---

## 🚀 Getting Started

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/compozy/compozy/pkg/mcp-proxy"
)

func main() {
    // Create proxy configuration
    config := &mcpproxy.Config{
        Port:               "6001",
        Host:               "127.0.0.1",
        BaseURL:            "http://127.0.0.1:6001",
        ShutdownTimeout:    10 * time.Second,
    }

    // Create storage (in-memory)
    storage := mcpproxy.NewInMemoryStorage()

    // Create and start proxy
    proxy := mcpproxy.NewProxy(config, storage)
    if err := proxy.Start(); err != nil {
        log.Fatal(err)
    }

    defer proxy.Stop()

    // Register an MCP server
    ctx := context.Background()
    definition := &mcpproxy.MCPDefinition{
        Name:      "echo-server",
        Transport: mcpproxy.TransportStdio,
        Command:   "echo",
        Args:      []string{"Hello, MCP!"},
    }

    if err := proxy.RegisterMCP(ctx, definition); err != nil {
        log.Fatal(err)
    }

    log.Println("Proxy running on http://127.0.0.1:6001")
    select {} // Keep running
}
```

---

## 📖 Usage

### Basic Proxy Setup

```go
// Create proxy with default configuration
proxy := mcpproxy.NewProxy(mcpproxy.DefaultConfig(), mcpproxy.NewInMemoryStorage())

// Start the proxy server
if err := proxy.Start(); err != nil {
    log.Fatal(err)
}

// Graceful shutdown
defer proxy.Stop()
```

### Admin API

```bash
# Register a new MCP server
curl -X POST http://127.0.0.1:6001/admin/mcps \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "chat-llm",
    "transport": "sse",
    "url": "https://api.example.com/mcp",
    "headers": {"X-API-Key": "secret"},
    "autoReconnect": true,
    "healthCheckEnabled": true
  }'

# List all registered MCPs
curl http://127.0.0.1:6001/admin/mcps

# Get aggregated tools
curl http://127.0.0.1:6001/admin/tools

# Check proxy health
curl http://127.0.0.1:6001/healthz
```

### Proxy Endpoints

```go
// Access MCP servers through the proxy
// For SSE transport
resp, err := http.Get("http://127.0.0.1:6001/chat-llm/sse")

// For streamable-http transport
resp, err := http.Post("http://127.0.0.1:6001/api-server/stream",
    "application/json", bytes.NewBuffer(payload))
```

<!-- Security configuration handled by external network controls -->

---

## 🔧 Configuration

### Server Configuration

```go
type Config struct {
    Port               string        // TCP port to bind (default: "6001")
    Host               string        // Listen address (default: "127.0.0.1")
    BaseURL            string        // Base URL for generating SSE paths
    ShutdownTimeout    time.Duration // Graceful shutdown timeout
}
```

### MCP Definition Schema

```go
type MCPDefinition struct {
    // Core identification
    Name        string        `json:"name"`
    Description string        `json:"description,omitempty"`
    Transport   TransportType `json:"transport"`

    // Stdio transport
    Command string            `json:"command,omitempty"`
    Args    []string          `json:"args,omitempty"`
    Env     map[string]string `json:"env,omitempty"`

    // HTTP-based transports
    URL     string            `json:"url,omitempty"`
    Headers map[string]string `json:"headers,omitempty"`
    Timeout time.Duration     `json:"timeout,omitempty"`

    // Security
    AuthTokens  []string `json:"authTokens,omitempty"`
    RequireAuth bool     `json:"requireAuth,omitempty"`

    // Behavior
    AutoReconnect       bool          `json:"autoReconnect,omitempty"`
    MaxReconnects       int           `json:"maxReconnects,omitempty"`
    ReconnectDelay      time.Duration `json:"reconnectDelay,omitempty"`
    HealthCheckEnabled  bool          `json:"healthCheckEnabled,omitempty"`
    HealthCheckInterval time.Duration `json:"healthCheckInterval,omitempty"`

    // Tool filtering
    ToolFilter *ToolFilter `json:"toolFilter,omitempty"`
}
```

### Storage Options

```go
// In-memory storage (default for tests)
storage := mcpproxy.NewMemoryStorage()

// Redis storage (distributed environments)
redisCfg := &mcpproxy.RedisConfig{Addr: "localhost:6379", DB: 0}
redisStorage, err := mcpproxy.NewRedisStorage(redisCfg)

// SugarDB storage (standalone environments)
// Uses embedded SugarDB to persist MCP definitions
sugarStorage := mcpproxy.NewSugarDBStorage()
```

### Mode-Aware Defaults

When embedded in Compozy, storage selection follows the global `mode`:

- `mode=standalone`: defaults to SugarDB storage (embedded) for zero external deps.
- `mode=distributed`: defaults to Redis storage when configured; otherwise falls back to memory.

You can override the default by providing an explicit storage instance to the server factory.

---

## 🎨 Examples

### Complete Production Setup

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/compozy/compozy/pkg/mcp-proxy"
)

func main() {
    // Production configuration
    config := &mcpproxy.Config{
        Port:            "6001",
        Host:            "0.0.0.0",
        BaseURL:         "https://mcp-proxy.example.com",
        ShutdownTimeout: 30 * time.Second,
    }

    // Redis storage for persistence
    redisConfig := &mcpproxy.RedisConfig{
        Addr:     os.Getenv("REDIS_URL"),
        Password: os.Getenv("REDIS_PASSWORD"),
    }

    storage, err := mcpproxy.NewRedisStorage(redisConfig)
    if err != nil {
        log.Fatal("Failed to create Redis storage:", err)
    }

    // Create and start proxy
    proxy := mcpproxy.NewProxy(config, storage)
    if err := proxy.Start(); err != nil {
        log.Fatal("Failed to start proxy:", err)
    }

    // Register some MCP servers
    ctx := context.Background()

    // OpenAI-compatible LLM server
    llmServer := &mcpproxy.MCPDefinition{
        Name:        "openai-llm",
        Description: "OpenAI-compatible LLM server",
        Transport:   mcpproxy.TransportSSE,
        URL:         "https://api.openai.com/v1/chat/completions",
        Headers: map[string]string{
            "Authorization": "Bearer " + os.Getenv("OPENAI_API_KEY"),
        },
        AutoReconnect:       true,
        MaxReconnects:       5,
        ReconnectDelay:      5 * time.Second,
        HealthCheckEnabled:  true,
        HealthCheckInterval: 30 * time.Second,
    }

    if err := proxy.RegisterMCP(ctx, llmServer); err != nil {
        log.Printf("Failed to register LLM server: %v", err)
    }

    // Local stdio MCP server
    toolServer := &mcpproxy.MCPDefinition{
        Name:      "local-tools",
        Transport: mcpproxy.TransportStdio,
        Command:   "python",
        Args:      []string{"-m", "mcp_tools.server"},
        Env: map[string]string{
            "PYTHONPATH": "/app/tools",
        },
        AutoReconnect:      true,
        HealthCheckEnabled: true,
    }

    if err := proxy.RegisterMCP(ctx, toolServer); err != nil {
        log.Printf("Failed to register tool server: %v", err)
    }

    log.Printf("Proxy running on %s", config.BaseURL)

    // Graceful shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan

    log.Println("Shutting down...")
    proxy.Stop()
}
```

### Compozy Engine Integration

```go
package main

import (
    "context"
    "time"

    "github.com/compozy/compozy/engine/mcp"
)

func main() {
    ctx := context.Background()

    // Create MCP proxy client
    client := mcp.NewProxyClient(
        "http://127.0.0.1:6001",
        "admin-token",
        30*time.Second,
    )

    // Health check
    if err := client.Health(ctx); err != nil {
        panic(err)
    }

    // Register MCP programmatically
    definition := &mcp.Definition{
        Name:      "search-engine",
        Transport: "sse",
        URL:       "https://search-api.example.com/mcp",
        Headers: map[string]string{
            "X-API-Key": "secret",
        },
        AutoReconnect:      true,
        HealthCheckEnabled: true,
    }

    if err := client.Register(ctx, definition); err != nil {
        panic(err)
    }

    // List all available tools
    tools, err := client.ListTools(ctx)
    if err != nil {
        panic(err)
    }

    for _, tool := range tools {
        log.Printf("Tool: %s from %s", tool.Name, tool.MCPName)
    }
}
```

### Workflow YAML Integration

```yaml
id: search-workflow
version: "0.1.0"

mcps:
  - id: search-mcp
    url: http://127.0.0.1:6001/search-engine/sse
    transport: sse
    use_proxy: true
    start_timeout: 10s
    max_sessions: 4

actions:
  - id: web-search
    prompt: "Search for: {{ .input.query }}"
    tool: $mcp(search-mcp#search_web)
    input:
      type: object
      properties:
        query:
          type: string
      required: [query]
```

---

## 📚 API Reference

### Core Types

```go
type Proxy interface {
    Start() error
    Stop() error
    RegisterMCP(ctx context.Context, def *MCPDefinition) error
    UpdateMCP(ctx context.Context, name string, def *MCPDefinition) error
    UnregisterMCP(ctx context.Context, name string) error
    ListMCPs(ctx context.Context) ([]MCPStatus, error)
    GetMCP(ctx context.Context, name string) (*MCPDefinition, error)
    ListTools(ctx context.Context) ([]Tool, error)
    Health(ctx context.Context) error
    Metrics(ctx context.Context) (*Metrics, error)
}

type TransportType string
type ConnectionStatus string
type MCPDefinition struct { ... }
type MCPStatus struct { ... }
```

### HTTP API Endpoints

#### System Endpoints

- `GET /healthz` - Health check
- `GET /admin/metrics` - Metrics

#### Admin API (requires authentication)

- `POST /admin/mcps` - Register MCP server
- `PUT /admin/mcps/{name}` - Update MCP server
- `DELETE /admin/mcps/{name}` - Remove MCP server
- `GET /admin/mcps` - List all MCP servers
- `GET /admin/mcps/{name}` - Get specific MCP server
- `GET /admin/tools` - List all tools

#### Proxy Endpoints

- `/{name}/sse[/{path}]` - SSE proxy endpoint
- `/{name}/stream[/{path}]` - Streamable HTTP proxy endpoint

### Factory Functions

```go
func NewProxy(config *Config, storage Storage) Proxy
func NewInMemoryStorage() Storage
func NewRedisStorage(config *RedisConfig) (Storage, error)
func DefaultConfig() *Config
```

---

## 🧪 Testing

```go
func TestProxyBasicFunctionality(t *testing.T) {
    // Create test proxy
    storage := mcpproxy.NewInMemoryStorage()
    config := &mcpproxy.Config{
        Port:        "0", // Random port
        Host:        "127.0.0.1",
    }

    proxy := mcpproxy.NewProxy(config, storage)
    err := proxy.Start()
    require.NoError(t, err)
    defer proxy.Stop()

    // Test registration
    ctx := context.Background()
    definition := &mcpproxy.MCPDefinition{
        Name:      "test-server",
        Transport: mcpproxy.TransportStdio,
        Command:   "echo",
        Args:      []string{"hello"},
    }

    err = proxy.RegisterMCP(ctx, definition)
    require.NoError(t, err)

    // Test listing
    mcps, err := proxy.ListMCPs(ctx)
    require.NoError(t, err)
    require.Len(t, mcps, 1)
    require.Equal(t, "test-server", mcps[0].Name)
}
```

---

## 📦 Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md)

---

## 📄 License

BSL-1.1 License - see [LICENSE](../../LICENSE)
