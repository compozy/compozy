# `mcp` – _Model Context Protocol client for external tool integration_

> **MCP (Model Context Protocol) client implementation providing seamless integration with external tools and services through a standardized protocol interface.**

---

## 📑 Table of Contents

- [🎯 Overview](#-overview)
- [💡 Motivation](#-motivation)
- [⚡ Design Highlights](#-design-highlights)
- [🚀 Getting Started](#-getting-started)
- [📖 Usage](#-usage)
  - [MCP Configuration](#mcp-configuration)
  - [Register Service](#register-service)
  - [Client Operations](#client-operations)
  - [Workflow Integration](#workflow-integration)
- [🔧 Configuration](#-configuration)
- [🎨 Examples](#-examples)
  - [Basic MCP Registration](#basic-mcp-registration)
  - [Remote MCP Servers](#remote-mcp-servers)
  - [Local Process Servers](#local-process-servers)
  - [Docker-based Servers](#docker-based-servers)
  - [Workflow Integration](#workflow-integration-1)
- [📚 API Reference](#-api-reference)
  - [Config](#config)
  - [RegisterService](#registerservice)
  - [Client](#client)
  - [Configuration Types](#configuration-types)
- [🧪 Testing](#-testing)
- [📦 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🎯 Overview

The `mcp` package provides a complete Model Context Protocol (MCP) client implementation for Compozy. MCP is a standardized protocol that enables AI agents to interact with external tools, services, and data sources through a unified interface.

Key capabilities include:

- **MCP Server Registration**: Manage registration and lifecycle of MCP servers with proxy
- **Transport Support**: Multiple transport mechanisms (SSE, HTTP streaming, stdio)
- **Tool Discovery**: Automatic discovery and registration of tools from MCP servers
- **Concurrent Operations**: Parallel registration/deregistration with error handling
- **Health Monitoring**: Server health checks and connection management
- **Retry Logic**: Robust error handling with exponential backoff

---

## 💡 Motivation

- **Tool Extension**: Enable AI agents to access external tools and services beyond core LLM capabilities
- **Standardization**: Provide a unified interface for tool integration across different providers
- **Scalability**: Support dynamic tool registration and management for large-scale deployments
- **Reliability**: Ensure robust communication with external services through retry mechanisms and health checks

---

## ⚡ Design Highlights

- **Protocol Compliance**: Full MCP protocol implementation with proper message handling
- **Transport Flexibility**: Support for multiple transport mechanisms (SSE, HTTP, stdio)
- **Proxy Architecture**: Centralized proxy management for MCP server coordination
- **Concurrent Safety**: Thread-safe operations with proper synchronization
- **Error Resilience**: Comprehensive error handling with retry logic and circuit breakers
- **Configuration Validation**: Robust validation of MCP server configurations
- **Resource Management**: Proper cleanup and resource management for server lifecycles

---

## 🚀 Getting Started

### Prerequisites

- Go 1.21+ with generics support
- MCP proxy server running (required for MCP operations)
- Environment variables configured:
  - `MCP_PROXY_URL`: URL of the MCP proxy service
  - `MCP_PROXY_ADMIN_TOKEN`: Optional admin token for proxy authentication

### Quick Setup

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/compozy/compozy/engine/mcp"
)

func main() {
    ctx := context.Background()

    // Create MCP client
    client := mcp.NewProxyClient(
        "http://localhost:3000",  // MCP proxy URL
        "admin-token",            // Admin token (optional)
        30*time.Second,          // Timeout
    )

    // Create register service
    service := mcp.NewRegisterService(client)

    // Configure MCP server
    config := &mcp.Config{
        ID:        "filesystem",
        URL:       "http://localhost:6001/mcp",
        Transport: "sse",
        Proto:     "2025-03-26",
    }

    // Register MCP server
    err := service.Ensure(ctx, config)
    if err != nil {
        log.Fatal(err)
    }

    // List registered tools
    tools, err := client.ListTools(ctx)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Available tools: %d", len(tools))
}
```

---

## 📖 Usage

### MCP Configuration

Configure MCP servers with proper transport and connection settings:

```go
// Remote MCP server with SSE transport
config := &mcp.Config{
    ID:        "github-api",
    URL:       "https://api.github.com/mcp/v1",
    Transport: "sse",
    Proto:     "2025-03-26",
    Env: map[string]string{
        "GITHUB_TOKEN": "your-github-token",
    },
}

// Local process server with stdio transport
config := &mcp.Config{
    ID:           "filesystem",
    Command:      "mcp-server-filesystem",
    Transport:    "stdio",
    Proto:        "2025-03-26",
    StartTimeout: 30 * time.Second,
    MaxSessions:  5,
}

// Set defaults and validate
config.SetDefaults()
if err := config.Validate(); err != nil {
    log.Fatal(err)
}
```

### Register Service

The `RegisterService` manages MCP server lifecycle:

```go
// Create service with proxy client
service := mcp.NewRegisterService(client)

// Register single MCP server
err := service.Ensure(ctx, config)
if err != nil {
    return err
}

// Register multiple servers concurrently
configs := []mcp.Config{
    {ID: "filesystem", Command: "mcp-server-filesystem", Transport: "stdio"},
    {ID: "github", URL: "https://api.github.com/mcp", Transport: "sse"},
    {ID: "database", Command: "mcp-server-db", Transport: "stdio"},
}

err = service.EnsureMultiple(ctx, configs)
if err != nil {
    return err
}

// Check registration status
isRegistered, err := service.IsRegistered(ctx, "filesystem")
if err != nil {
    return err
}

// List all registered MCPs
mcpIDs, err := service.ListRegistered(ctx)
if err != nil {
    return err
}

// Deregister when done
err = service.Deregister(ctx, "filesystem")
if err != nil {
    return err
}
```

### Client Operations

The `Client` provides direct proxy communication:

```go
// Create proxy client
client := mcp.NewProxyClient(
    "http://mcp-proxy:3000",
    "admin-token",
    30*time.Second,
)

// Health check
err := client.Health(ctx)
if err != nil {
    log.Printf("Proxy unhealthy: %v", err)
}

// List available tools
tools, err := client.ListTools(ctx)
if err != nil {
    return err
}

for _, tool := range tools {
    log.Printf("Tool: %s (%s) - %s", tool.Name, tool.MCPName, tool.Description)
}

// Call a tool
result, err := client.CallTool(ctx, "filesystem", "read_file", map[string]any{
    "path": "/etc/hosts",
})
if err != nil {
    return err
}

log.Printf("Tool result: %v", result)
```

### Workflow Integration

Integrate MCP servers into workflow configurations:

```go
// Workflow configuration with MCP servers
type WorkflowConfig struct {
    ID    string
    MCPs  []mcp.Config
    Tasks []TaskConfig
}

func (w *WorkflowConfig) GetMCPs() []mcp.Config {
    return w.MCPs
}

// Setup MCP service for workflows
workflows := []mcp.WorkflowConfig{
    &WorkflowConfig{
        ID: "data-processing",
        MCPs: []mcp.Config{
            {ID: "database", Command: "mcp-server-db", Transport: "stdio"},
            {ID: "s3", URL: "https://s3-mcp.example.com", Transport: "sse"},
        },
    },
}

// Initialize MCP service
service, err := mcp.SetupForWorkflows(ctx, workflows)
if err != nil {
    return err
}

// Service automatically registers MCPs in background
// Cleanup on shutdown
defer service.Shutdown(ctx)
```

---

## 🔧 Configuration

### MCP Server Configuration

```go
type Config struct {
    Resource     string                    // Resource identifier
    ID           string                    // Unique MCP server ID
    URL          string                    // Remote server URL (for HTTP transports)
    Command      string                    // Local command to execute (for stdio)
    Env          map[string]string         // Environment variables
    Proto        string                    // Protocol version (YYYY-MM-DD)
    Transport    mcpproxy.TransportType    // Transport type (sse, streamable-http, stdio)
    StartTimeout time.Duration            // Process startup timeout
    MaxSessions  int                       // Maximum concurrent sessions
}
```

### Transport Types

```go
const (
    TransportSSE              = "sse"               // Server-Sent Events
    TransportStreamableHTTP   = "streamable-http"   // HTTP with streaming
    TransportStdio           = "stdio"             // Standard I/O
)
```

### Default Values

```go
const (
    DefaultProtocolVersion = "2025-03-26"    // Default protocol version
    DefaultTransport      = TransportSSE     // Default transport type
)
```

---

## 🎨 Examples

### Basic MCP Registration

```go
func ExampleBasicRegistration() {
    ctx := context.Background()

    // Create client and service
    client := mcp.NewProxyClient("http://localhost:3000", "", 30*time.Second)
    service := mcp.NewRegisterService(client)

    // Configure MCP server
    config := &mcp.Config{
        ID:        "calculator",
        URL:       "http://localhost:6001/mcp",
        Transport: "sse",
    }

    // Set defaults and validate
    config.SetDefaults()
    if err := config.Validate(); err != nil {
        log.Fatal(err)
    }

    // Register server
    err := service.Ensure(ctx, config)
    if err != nil {
        log.Fatal(err)
    }

    // Verify registration
    registered, err := service.IsRegistered(ctx, "calculator")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Calculator MCP registered: %t\n", registered)
}
```

### Remote MCP Servers

```go
func ExampleRemoteServer() {
    ctx := context.Background()

    // GitHub API MCP server
    githubConfig := &mcp.Config{
        ID:        "github-api",
        URL:       "https://api.github.com/mcp/v1",
        Transport: "sse",
        Proto:     "2025-03-26",
        Env: map[string]string{
            "GITHUB_TOKEN": "ghp_xxxxxxxxxxxxxxxxxxxx",
            "GITHUB_ORG":   "my-organization",
        },
    }

    // Weather API MCP server
    weatherConfig := &mcp.Config{
        ID:        "weather-api",
        URL:       "https://weather-mcp.example.com/v1",
        Transport: "streamable-http",
        Proto:     "2025-03-26",
        Env: map[string]string{
            "API_KEY": "your-weather-api-key",
        },
        MaxSessions: 10,
    }

    // Register multiple remote servers
    service := mcp.NewRegisterService(client)

    configs := []mcp.Config{*githubConfig, *weatherConfig}
    err := service.EnsureMultiple(ctx, configs)
    if err != nil {
        log.Fatal(err)
    }

    // List available tools
    tools, err := client.ListTools(ctx)
    if err != nil {
        log.Fatal(err)
    }

    for _, tool := range tools {
        fmt.Printf("Available tool: %s from %s\n", tool.Name, tool.MCPName)
    }
}
```

### Local Process Servers

```go
func ExampleLocalProcessServer() {
    ctx := context.Background()

    // Filesystem MCP server
    filesystemConfig := &mcp.Config{
        ID:           "filesystem",
        Command:      "mcp-server-filesystem",
        Transport:    "stdio",
        Proto:        "2025-03-26",
        StartTimeout: 30 * time.Second,
        MaxSessions:  5,
        Env: map[string]string{
            "ALLOWED_PATHS": "/data,/tmp",
            "LOG_LEVEL":     "info",
        },
    }

    // Python runtime server
    pythonConfig := &mcp.Config{
        ID:           "python-runtime",
        Command:      "python /app/mcp_python_server.py",
        Transport:    "stdio",
        Proto:        "2025-03-26",
        StartTimeout: 60 * time.Second,
        Env: map[string]string{
            "PYTHONPATH": "/app/libs:/app/modules",
            "PYTHON_ENV": "production",
        },
    }

    service := mcp.NewRegisterService(client)

    // Register filesystem server
    err := service.Ensure(ctx, filesystemConfig)
    if err != nil {
        log.Fatal(err)
    }

    // Register Python runtime
    err = service.Ensure(ctx, pythonConfig)
    if err != nil {
        log.Fatal(err)
    }

    // Test filesystem tool
    result, err := client.CallTool(ctx, "filesystem", "list_directory", map[string]any{
        "path": "/data",
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Directory listing: %v\n", result)
}
```

### Docker-based Servers

```go
func ExampleDockerServer() {
    ctx := context.Background()

    // PostgreSQL MCP server in Docker
    postgresConfig := &mcp.Config{
        ID:           "postgres-db",
        Command:      "docker run --rm -i mcp/postgres:latest",
        Transport:    "stdio",
        Proto:        "2025-03-26",
        StartTimeout: 60 * time.Second,
        MaxSessions:  3,
        Env: map[string]string{
            "DATABASE_URL": "postgres://user:pass@db:5432/myapp",
            "SCHEMA":       "public",
        },
    }

    // Redis MCP server in Docker
    redisConfig := &mcp.Config{
        ID:           "redis-cache",
        Command:      "docker run --rm -i mcp/redis:latest",
        Transport:    "stdio",
        Proto:        "2025-03-26",
        StartTimeout: 30 * time.Second,
        Env: map[string]string{
            "REDIS_URL": "redis://redis:6379/0",
        },
    }

    service := mcp.NewRegisterService(client)

    // Register Docker-based servers
    configs := []mcp.Config{*postgresConfig, *redisConfig}
    err := service.EnsureMultiple(ctx, configs)
    if err != nil {
        log.Fatal(err)
    }

    // Use database tool
    result, err := client.CallTool(ctx, "postgres-db", "execute_query", map[string]any{
        "query": "SELECT COUNT(*) FROM users",
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Query result: %v\n", result)
}
```

### Workflow Integration

```go
func ExampleWorkflowIntegration() {
    ctx := context.Background()

    // Define workflow with MCP servers
    type DataProcessingWorkflow struct {
        ID   string
        MCPs []mcp.Config
    }

    func (w *DataProcessingWorkflow) GetMCPs() []mcp.Config {
        return w.MCPs
    }

    // Create workflow configuration
    workflow := &DataProcessingWorkflow{
        ID: "data-processing-pipeline",
        MCPs: []mcp.Config{
            {
                ID:        "s3-storage",
                URL:       "https://s3-mcp.example.com/v1",
                Transport: "sse",
                Env: map[string]string{
                    "AWS_ACCESS_KEY_ID":     "AKIAIOSFODNN7EXAMPLE",
                    "AWS_SECRET_ACCESS_KEY": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
                    "AWS_REGION":            "us-east-1",
                },
            },
            {
                ID:        "data-processor",
                Command:   "python /app/data_processor.py",
                Transport: "stdio",
                Env: map[string]string{
                    "PROCESSING_MODE": "batch",
                    "BATCH_SIZE":      "1000",
                },
            },
        },
    }

    // Setup MCP service for workflows
    workflows := []mcp.WorkflowConfig{workflow}
    service, err := mcp.SetupForWorkflows(ctx, workflows)
    if err != nil {
        log.Fatal(err)
    }

    // Service automatically registers MCPs in background
    // Give time for registration
    time.Sleep(2 * time.Second)

    // Verify registration
    registered, err := service.ListRegistered(ctx)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Registered MCPs: %v\n", registered)

    // Cleanup on shutdown
    defer func() {
        if err := service.Shutdown(ctx); err != nil {
            log.Printf("Shutdown error: %v", err)
        }
    }()
}
```

---

## 📚 API Reference

### Config

```go
type Config struct {
    Resource     string                    // Resource identifier (defaults to ID)
    ID           string                    // Unique MCP server identifier
    URL          string                    // Remote server URL (for HTTP transports)
    Command      string                    // Local command (for stdio transport)
    Env          map[string]string         // Environment variables
    Proto        string                    // Protocol version (YYYY-MM-DD format)
    Transport    mcpproxy.TransportType    // Transport mechanism
    StartTimeout time.Duration            // Process startup timeout
    MaxSessions  int                       // Maximum concurrent sessions
}

func (c *Config) SetDefaults()
func (c *Config) Validate() error
func (c *Config) Clone() (*Config, error)
```

### RegisterService

```go
type RegisterService struct {
    // MCP server registration management
}

func NewRegisterService(proxyClient *Client) *RegisterService
func NewWithTimeout(proxyURL, adminToken string, timeout time.Duration) *RegisterService

// Registration operations
func (s *RegisterService) Ensure(ctx context.Context, config *Config) error
func (s *RegisterService) EnsureMultiple(ctx context.Context, configs []Config) error
func (s *RegisterService) Deregister(ctx context.Context, mcpID string) error

// Query operations
func (s *RegisterService) IsRegistered(ctx context.Context, mcpID string) (bool, error)
func (s *RegisterService) ListRegistered(ctx context.Context) ([]string, error)

// Lifecycle operations
func (s *RegisterService) HealthCheck(ctx context.Context) error
func (s *RegisterService) Shutdown(ctx context.Context) error
```

### Client

```go
type Client struct {
    // HTTP client for MCP proxy communication
}

func NewProxyClient(baseURL, adminToken string, timeout time.Duration) *Client

// Proxy operations
func (c *Client) Health(ctx context.Context) error
func (c *Client) Register(ctx context.Context, def *Definition) error
func (c *Client) Deregister(ctx context.Context, name string) error
func (c *Client) ListMCPs(ctx context.Context) ([]Definition, error)

// Tool operations
func (c *Client) ListTools(ctx context.Context) ([]ToolDefinition, error)
func (c *Client) CallTool(ctx context.Context, mcpName, toolName string, arguments map[string]any) (any, error)

// Resource management
func (c *Client) Close() error
```

### Configuration Types

```go
type Definition struct {
    Name      string                    // MCP server name
    Env       map[string]string         // Environment variables
    URL       string                    // Remote server URL
    Transport mcpproxy.TransportType    // Transport type
    Command   string                    // Local command
    Args      []string                  // Command arguments
}

type ToolDefinition struct {
    Name        string            // Tool name
    Description string            // Tool description
    InputSchema map[string]any    // JSON schema for input
    MCPName     string            // Associated MCP server name
}

type RetryConfig struct {
    MaxAttempts uint64            // Maximum retry attempts
    BaseDelay   time.Duration     // Base delay between retries
    MaxDelay    time.Duration     // Maximum delay between retries
}
```

### Error Types

```go
type ProxyRequestError struct {
    StatusCode int               // HTTP status code
    Message    string            // Error message
    Err        error             // Underlying error
}

func (e *ProxyRequestError) Error() string
func (e *ProxyRequestError) Unwrap() error
```

### Utility Functions

```go
func CollectWorkflowMCPs(workflows []WorkflowConfig) []Config
func SetupForWorkflows(ctx context.Context, workflows []WorkflowConfig) (*RegisterService, error)
func DefaultRetryConfig() RetryConfig
```

---

## 🧪 Testing

### Unit Testing

```go
func TestRegisterService_Ensure(t *testing.T) {
    ctx := context.Background()

    // Mock client
    mockClient := &MockClient{}
    service := mcp.NewRegisterService(mockClient)

    // Test configuration
    config := &mcp.Config{
        ID:        "test-server",
        URL:       "http://localhost:6001/mcp",
        Transport: "sse",
    }

    // Set up expectations
    mockClient.On("Register", mock.Anything, mock.AnythingOfType("*mcp.Definition")).
        Return(nil)

    // Test registration
    err := service.Ensure(ctx, config)
    assert.NoError(t, err)

    // Verify expectations
    mockClient.AssertExpectations(t)
}
```

### Integration Testing

```go
func TestClient_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    ctx := context.Background()

    // Setup real proxy client
    proxyURL := os.Getenv("MCP_PROXY_URL")
    if proxyURL == "" {
        t.Skip("MCP_PROXY_URL not set")
    }

    client := mcp.NewProxyClient(proxyURL, "", 30*time.Second)

    // Test health check
    err := client.Health(ctx)
    assert.NoError(t, err)

    // Test MCP registration
    definition := &mcp.Definition{
        Name:      "test-mcp",
        URL:       "http://localhost:6001/mcp",
        Transport: "sse",
    }

    err = client.Register(ctx, definition)
    assert.NoError(t, err)

    // Cleanup
    defer client.Deregister(ctx, "test-mcp")

    // Test listing
    mcps, err := client.ListMCPs(ctx)
    assert.NoError(t, err)
    assert.NotEmpty(t, mcps)
}
```

### Running Tests

```bash
# Run unit tests
go test ./engine/mcp/...

# Run with coverage
go test -cover ./engine/mcp/...

# Run integration tests
go test -v -tags=integration ./engine/mcp/...

# Run specific test
go test -v -run TestRegisterService_Ensure ./engine/mcp/
```

---

## 📦 Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md) for development guidelines.

---

## 📄 License

BSL-1.1 License - see [LICENSE](../../LICENSE) for details.
