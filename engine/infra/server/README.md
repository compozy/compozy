# `server` – _HTTP server infrastructure with monitoring, rate limiting, and graceful shutdown_

> **Production-ready HTTP server foundation providing Gin-based web service infrastructure, application state management, and graceful shutdown for workflow orchestration.**

---

## 📑 Table of Contents

- [🎯 Overview](#-overview)
- [💡 Motivation](#-motivation)
- [⚡ Design Highlights](#-design-highlights)
- [🚀 Getting Started](#-getting-started)
- [📖 Usage](#-usage)
  - [Basic Server Setup](#basic-server-setup)
  - [Middleware Integration](#middleware-integration)
  - [Application State](#application-state)
  - [Route Registration](#route-registration)
- [🔧 Configuration](#-configuration)
- [🎨 Examples](#-examples)
- [📚 API Reference](#-api-reference)
- [🧪 Testing](#-testing)
- [📦 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🎯 Overview

The `server` package provides a comprehensive HTTP server infrastructure for the Compozy workflow orchestration engine. It combines Gin web framework with production-ready features including monitoring integration, rate limiting, graceful shutdown, and centralized application state management.

Key features include:

- **Gin-based HTTP Server**: High-performance web framework with middleware support
- **Application State Management**: Centralized dependency injection and state sharing
- **Graceful Shutdown**: Signal handling with proper resource cleanup
- **Monitoring Integration**: Built-in metrics and health check endpoints
- **Rate Limiting**: Configurable request rate limiting middleware
- **CORS Support**: Cross-origin resource sharing configuration

---

## 💡 Motivation

- **Production Foundation**: Robust HTTP server infrastructure for production deployment
- **Dependency Injection**: Centralized application state management across all handlers
- **Graceful Operations**: Proper startup, shutdown, and error handling patterns
- **Monitoring Integration**: Built-in observability for production environments

---

## ⚡ Design Highlights

- **Dependency Injection**: Application state pattern for clean dependency management
- **Middleware Pipeline**: Extensible middleware chain with monitoring, CORS, and rate limiting
- **Signal Handling**: Graceful shutdown on SIGINT/SIGTERM with proper cleanup
- **Health Checks**: Built-in health and readiness endpoints
- **Schedule Management**: Integrated workflow schedule reconciliation with retry logic

---

## 🚀 Getting Started

```go
package main

import (
    "context"
    "log"

    "github.com/compozy/compozy/engine/infra/server"
    "github.com/compozy/compozy/pkg/config"
)

func main() {
    ctx := context.Background()

    // Create unified configuration
    appConfig := &config.Config{
        Server: config.ServerConfig{
            Host:        "0.0.0.0",
            Port:        5001,
            CORSEnabled: true,
        },
        // Add database, temporal, and other configs...
    }

    // Create server instance
    srv, err := server.NewServer(ctx, ".", "compozy.yaml", ".env")
    if err != nil {
        log.Fatal("Failed to create server:", err)
    }

    // Start server (blocks until shutdown)
    if err := srv.Run(); err != nil {
        log.Fatal("Server failed:", err)
    }
}
```

---

## 📖 Usage

### Basic Server Setup

```go
// Create server with configuration
srv, err := server.NewServer(ctx, cwd, configFile, envFile)
if err != nil {
    log.Fatal("Failed to create server:", err)
}

// Start server (this blocks until shutdown signal)
if err := srv.Run(); err != nil {
    log.Fatal("Server failed:", err)
}
```

### Middleware Integration

```go
// Server automatically sets up middleware chain:
// 1. Recovery middleware
// 2. Rate limiting (if configured)
// 3. Monitoring middleware (if enabled)
// 4. Logger middleware
// 5. CORS middleware (if enabled)
// 6. Application state middleware
// 7. Error handler middleware

// Access middleware in handlers via context
func myHandler(c *gin.Context) {
    // Application state is automatically injected
    state := appstate.FromContext(c)

    // Use state for database access, etc.
    workflows := state.Store.NewWorkflowRepo()
    // ...
}
```

### Application State

```go
// Application state provides centralized access to dependencies
type State struct {
    Store          *store.Store
    ProjectConfig  *project.Config
    Workflows      []*workflow.Config
    ClientConfig   *worker.TemporalConfig
    Worker         *worker.Worker
    Extensions     map[string]interface{}
}

// Access state in handlers
func getWorkflows(c *gin.Context) {
    state := appstate.FromContext(c)

    // Access database
    repo := state.Store.NewWorkflowRepo()
    workflows, err := repo.GetAll(c.Request.Context())
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    c.JSON(200, workflows)
}
```

### Route Registration

```go
// Routes are automatically registered through RegisterRoutes function
// Custom routes can be added by modifying the registration logic

// Example custom route handler
func CreateHealthHandler(server *server.Server, version string) gin.HandlerFunc {
    return func(c *gin.Context) {
        status := gin.H{
            "status":  "ok",
            "version": version,
        }

        // Add reconciliation status
        if ready, lastAttempt, attempts, err := server.GetReconciliationStatus(); err != nil {
            status["reconciliation"] = gin.H{
                "ready":        ready,
                "last_attempt": lastAttempt,
                "attempts":     attempts,
                "error":        err.Error(),
            }
        } else {
            status["reconciliation"] = gin.H{
                "ready": ready,
            }
        }

        c.JSON(200, status)
    }
}
```

---

## 🔧 Configuration

### Server Configuration

```go
type Config struct {
    CWD         string                  // Working directory
    Host        string                  // Server host
    Port        int                     // Server port
    CORSEnabled bool                    // Enable CORS
    ConfigFile  string                  // Config file path
    EnvFilePath string                  // Environment file path
    RateLimit   *ratelimit.Config       // Rate limiting config
}
```

### Application Configuration

```go
// Uses unified config.Config structure
appConfig := &config.Config{
    Server: config.ServerConfig{
        Host:        "0.0.0.0",
        Port:        5001,
        CORSEnabled: true,
    },
    Database: config.DatabaseConfig{
        Driver:   "postgres",
        Host:     "localhost",
        Port:     5432,
        Name:     "compozy",
        User:     "postgres",
        Password: "password",
    },
    Temporal: config.TemporalConfig{
        HostPort:  "localhost:7233",
        Namespace: "default",
        TaskQueue: "compozy-tasks",
    },
    Cache: config.CacheConfig{
        Host: "localhost",
        Port: "6379",
    },
}
```

### Rate Limiting Configuration

```go
rateLimitConfig := &ratelimit.Config{
    GlobalRate: ratelimit.RateConfig{
        Limit:  100,                    // 100 requests
        Period: time.Minute,            // per minute
    },
    IPRate: ratelimit.RateConfig{
        Limit:  20,                     // 20 requests per IP
        Period: time.Minute,            // per minute
    },
}

// Server configuration is now handled through the unified config system
// See pkg/config.ServerConfig for the centralized server configuration
```

---

## 🎨 Examples

### Production Server Setup

```go
func startProductionServer() error {
    ctx := context.Background()

    // Load configuration from files and environment
    appConfig, err := config.LoadConfig("compozy.yaml", ".env")
    if err != nil {
        return fmt.Errorf("failed to load config: %w", err)
    }

    // Create server with production settings
    srv, err := server.NewServer(ctx, ".", "compozy.yaml", ".env")
    if err != nil {
        log.Fatal("Failed to create server:", err)
    }

    // Run server with graceful shutdown
    return srv.Run()
}
```

### Custom Middleware Integration

```go
// Add custom middleware to the server
func customMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Custom logic before request
        start := time.Now()

        // Process request
        c.Next()

        // Custom logic after request
        duration := time.Since(start)
        log.Printf("Request processed in %v", duration)
    }
}

// Middleware is automatically configured in buildRouter method
// To add custom middleware, modify the buildRouter function
```

### Schedule Management Integration

```go
func getScheduleStatus(c *gin.Context) {
    state := appstate.FromContext(c)

    // Get schedule manager from extensions
    scheduleManager, exists := state.Extensions[appstate.ScheduleManagerKey]
    if !exists {
        c.JSON(500, gin.H{"error": "schedule manager not available"})
        return
    }

    manager := scheduleManager.(schedule.Manager)

    // Get schedule information
    // This would require extending the schedule manager interface
    // with status methods

    c.JSON(200, gin.H{
        "status": "active",
        "manager": "initialized",
    })
}
```

### Health Check Implementation

```go
func advancedHealthCheck(server *server.Server) gin.HandlerFunc {
    return func(c *gin.Context) {
        state := appstate.FromContext(c)

        health := gin.H{
            "status":    "ok",
            "timestamp": time.Now().ISO8601(),
        }

        // Check database health
        if err := state.Store.HealthCheck(c.Request.Context()); err != nil {
            health["database"] = gin.H{
                "status": "unhealthy",
                "error":  err.Error(),
            }
        } else {
            health["database"] = gin.H{"status": "healthy"}
        }

        // Check reconciliation status
        ready, lastAttempt, attempts, err := server.GetReconciliationStatus()
        health["reconciliation"] = gin.H{
            "ready":        ready,
            "last_attempt": lastAttempt,
            "attempts":     attempts,
        }
        if err != nil {
            health["reconciliation"].(gin.H)["error"] = err.Error()
        }

        // Check worker health
        if state.Worker != nil {
            health["worker"] = gin.H{"status": "running"}
        } else {
            health["worker"] = gin.H{"status": "not_initialized"}
        }

        c.JSON(200, health)
    }
}
```

### Graceful Shutdown Handling

```go
func runServerWithGracefulShutdown(srv *server.Server) error {
    // Start server in goroutine
    errChan := make(chan error, 1)
    go func() {
        errChan <- srv.Run()
    }()

    // Wait for shutdown signal or error
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    select {
    case err := <-errChan:
        return err
    case sig := <-sigChan:
        log.Printf("Received signal %v, shutting down...", sig)

        // Trigger graceful shutdown
        srv.Shutdown()

        // Wait for shutdown to complete
        return <-errChan
    }
}
```

---

## 📚 API Reference

### Core Types

#### `Server`

Main server struct managing HTTP server lifecycle.

```go
type Server struct {
    Config              *Config
    AppConfig           *config.Config
    // private fields
}

func NewServer(ctx context.Context, cwd, configFile, envFilePath string) (*Server, error)
func (s *Server) Run() error
func (s *Server) Shutdown()
func (s *Server) GetReconciliationStatus() (bool, time.Time, int, error)
func (s *Server) IsReconciliationReady() bool
```

#### `Config`

Server configuration structure.

```go
type Config struct {
    CWD         string
    Host        string
    Port        int
    CORSEnabled bool
    ConfigFile  string
    EnvFilePath string
    RateLimit   *ratelimit.Config
}

func (c *Config) FullAddress() string
func (c *Config) ResolvePath(path string) string
```

### Middleware Functions

#### CORS Middleware

```go
func CORSMiddleware() gin.HandlerFunc
```

#### Logger Middleware

```go
func LoggerMiddleware(log logger.Logger) gin.HandlerFunc
```

### Route Registration

#### Route Registration Function

```go
func RegisterRoutes(ctx context.Context, router *gin.Engine, state *appstate.State, server *Server) error
```

#### Health Handler

```go
func CreateHealthHandler(server *Server, version string) gin.HandlerFunc
```

### Application State

#### State Management

```go
// Access state in handlers
func FromContext(c *gin.Context) *appstate.State

// State structure
type State struct {
    Store          *store.Store
    ProjectConfig  *project.Config
    Workflows      []*workflow.Config
    ClientConfig   *worker.TemporalConfig
    Worker         *worker.Worker
    Extensions     map[string]interface{}
}
```

### Constants

#### Timeout Constants

```go
const (
    monitoringInitTimeout     = 500 * time.Millisecond
    monitoringShutdownTimeout = 5 * time.Second
    dbShutdownTimeout         = 30 * time.Second
    workerShutdownTimeout     = 30 * time.Second
    serverShutdownTimeout     = 5 * time.Second
    httpReadTimeout           = 15 * time.Second
    httpWriteTimeout          = 15 * time.Second
    httpIdleTimeout           = 60 * time.Second
)
```

---

## 🧪 Testing

### Unit Tests

```go
func TestServer(t *testing.T) {
    ctx := context.Background()

    // Create test configuration
    appConfig := &config.Config{
        Server: config.ServerConfig{
            Host: "localhost",
            Port: 0, // Random port
        },
        Database: config.DatabaseConfig{
            Driver: "sqlite",
            Name:   ":memory:",
        },
    }

    t.Run("Should create server instance", func(t *testing.T) {
        srv, err := server.NewServer(ctx, ".", "test.yaml", ".env")
        assert.NoError(t, err)
        assert.NotNil(t, srv)
        assert.Equal(t, "localhost", srv.Config.Host)
    })
}
```

### Integration Tests

```go
func TestServerIntegration(t *testing.T) {
    ctx := context.Background()

    // Setup test server
    srv := setupTestServer(t)

    // Start server in background
    go func() {
        if err := srv.Run(); err != nil {
            t.Errorf("Server failed: %v", err)
        }
    }()

    // Wait for server to be ready
    time.Sleep(100 * time.Millisecond)

    // Test health endpoint
    resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", srv.Config.Port))
    require.NoError(t, err)
    defer resp.Body.Close()

    assert.Equal(t, 200, resp.StatusCode)

    // Test graceful shutdown
    srv.Shutdown()
}
```

### Test Utilities

```go
func setupTestServer(t *testing.T) *server.Server {
    ctx := context.Background()

    appConfig := &config.Config{
        Server: config.ServerConfig{
            Host: "localhost",
            Port: getFreePort(),
        },
        Database: config.DatabaseConfig{
            Driver: "sqlite",
            Name:   ":memory:",
        },
    }

    srv, err := server.NewServer(ctx, ".", "test.yaml", ".env")
    if err != nil {
        t.Fatal("Failed to create server:", err)
    }

    t.Cleanup(func() {
        srv.Shutdown()
    })

    return srv
}

func getFreePort() int {
    listener, err := net.Listen("tcp", ":0")
    if err != nil {
        return 5001
    }
    defer listener.Close()

    return listener.Addr().(*net.TCPAddr).Port
}
```

### Load Testing

```go
func TestServerLoad(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping load test in short mode")
    }

    srv := setupTestServer(t)

    // Start server
    go srv.Run()
    time.Sleep(100 * time.Millisecond)

    // Concurrent requests
    var wg sync.WaitGroup
    concurrency := 100
    requestsPerWorker := 10

    for i := 0; i < concurrency; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()

            for j := 0; j < requestsPerWorker; j++ {
                resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", srv.Config.Port))
                if err != nil {
                    t.Errorf("Request failed: %v", err)
                    return
                }
                resp.Body.Close()

                if resp.StatusCode != 200 {
                    t.Errorf("Expected 200, got %d", resp.StatusCode)
                    return
                }
            }
        }()
    }

    wg.Wait()
    srv.Shutdown()
}
```

---

## 📦 Contributing

See [CONTRIBUTING.md](../../../CONTRIBUTING.md)

---

## 📄 License

BSL-1.1 License - see [LICENSE](../../../LICENSE)
