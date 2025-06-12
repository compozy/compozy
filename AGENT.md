# Compozy Development Guide

Compozy is a **workflow orchestration engine for AI agents** that enables building AI-powered applications through declarative YAML configuration and a robust Go backend.

## Quick Start

```bash
# Setup
make deps && make start-docker && make migrate-up

# Development
make dev              # Start development server
make test             # Run tests (excludes slow tests)
make test-all         # Full test suite
make fmt && make lint # Format and lint code
```

## Architecture

```
compozy/
├── engine/           # Core domain logic
│   ├── agent/        # AI agent management
│   ├── task/         # Task orchestration
│   ├── tool/         # Tool execution
│   ├── workflow/     # Workflow definition
│   ├── runtime/      # Deno runtime integration
│   └── infra/        # Infrastructure (server, db, messaging)
├── cli/              # Command-line interface
├── pkg/              # Reusable packages
└── test/             # Test suite
```

**Tech Stack:** Go 1.24+, PostgreSQL, Temporal, NATS, Gin, Deno

## 🚨 CRITICAL: Testing Standards

### Test Pattern Requirements

**MANDATORY:** All tests MUST use the `t.Run("Should...")` pattern:

```go
// ✅ CORRECT - Always use this pattern
func TestTaskExecutor_Execute(t *testing.T) {
    t.Run("Should execute task successfully", func(t *testing.T) {
        // test implementation
    })

    t.Run("Should handle execution errors", func(t *testing.T) {
        // test implementation
    })
}

// ❌ WRONG - Do not write tests like this
func TestTaskExecutor_Execute(t *testing.T) {
    // direct test implementation without t.Run
}
```

### Table-Driven Tests

**ONLY** use table-driven tests when you have **many similar test cases** (5+ variations):

```go
// ✅ ACCEPTABLE - Only when truly necessary
func TestValidateInput(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected bool
    }{
        {"Should accept valid email", "user@example.com", true},
        {"Should reject invalid email", "invalid", false},
        // ... many more cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test implementation
        })
    }
}

// ❌ AVOID - Don't use tables for just 2-3 cases
```

### Test Organization

- Place `*_test.go` files alongside implementation files
- Use testify for assertions and mocks
- Each test must be independent and repeatable
- Mock external dependencies

## Code Quality Standards

### Linting Requirements (`.golangci.yml`)

- **Function length:** Max 80 lines or 50 statements
- **Line length:** Max 120 characters
- **Cyclomatic complexity:** Max 15
- **Error handling:** All errors must be checked

### Go Best Practices

#### Error Handling

- **Custom errors:** Use `core.NewError(err, "CODE", details)` for structured errors
- **Error wrapping:** `fmt.Errorf("context: %w", err)` for context
- **Transaction pattern:**

```go
defer func() {
    if err != nil { tx.Rollback(ctx) } else { tx.Commit(ctx) }
}()
```

#### Core Patterns & Conventions

**Interface Design:**

```go
// Small, focused interfaces
type Storage interface {
    SaveMCP(ctx context.Context, def *MCPDefinition) error
    LoadMCP(ctx context.Context, name string) (*MCPDefinition, error)
    Close() error
}
```

**Concurrency Patterns:**

```go
// Thread-safe structs with embedded mutex
type Status struct {
    Name   string
    mu     sync.RWMutex // Protects all fields
}

// Concurrent operations with errgroup
g, ctx := errgroup.WithContext(ctx)
for _, item := range items {
    item := item // capture loop variable
    g.Go(func() error { return process(ctx, item) })
}
```

**Factory Pattern:**

```go
func NewStorage(config *StorageConfig) (Storage, error) {
    switch config.Type {
    case StorageTypeRedis:
        return NewRedisStorage(config.Redis)
    case StorageTypeMemory:
        return NewMemoryStorage(), nil
    }
}
```

**Configuration with Defaults:**

```go
func NewService(config *Config) *Service {
    if config == nil {
        config = DefaultConfig() // Always provide defaults
    }
}
```

**Graceful Shutdown:**

```go
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
select {
case <-ctx.Done():
    return shutdown(ctx)
case <-quit:
    return shutdown(ctx)
}
```

**Middleware Pattern:**

```go
func authMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        if !isValidToken(c.GetHeader("Authorization")) {
            c.JSON(401, gin.H{"error": "unauthorized"})
            c.Abort()
            return
        }
        c.Next()
    }
}
```

**Resource Management:**

```go
// Connection limits
if len(m.clients) >= m.config.MaxConnections {
    return fmt.Errorf("max connections reached")
}

// Cleanup with defer
defer func() {
    m.cancel()
    m.wg.Wait()
}()
```

#### Common Libraries

- **Web:** `gin-gonic/gin` for HTTP APIs
- **DB:** `jackc/pgx/v5` (PostgreSQL), `redis/go-redis/v9`
- **Testing:** `stretchr/testify` for assertions/mocks
- **Validation:** `go-playground/validator/v10`
- **Logging:** `charmbracelet/log` - use `logger.Info/Error/Debug`
- **CLI:** `spf13/cobra` for commands
- **Docs:** `swaggo/swag` for API documentation

### Project Utilities

- **Logger:** Use `pkg/logger` for structured logging
    ```go
    logger.Info("task executed", "task_id", id, "duration", time.Since(start))
    logger.Error("execution failed", "error", err, "task_id", id)
    ```
- **Core types:** `core.ID` (UUIDs), `core.Ref` (polymorphic refs)
- **Test helpers:** `utils.SetupTest()`, `utils.SetupFixture()`
- **Template engine:** `pkg/tplengine` for dynamic configs

## Configuration

### Project Configuration (`compozy.yaml`)

```yaml
name: project-name
version: 0.1.0

workflows:
    - source: ./workflow.yaml

models:
    - provider: ollama|openai|groq
      model: model-name
      api_key: "{{ .env.API_KEY }}"

runtime:
    permissions:
        - --allow-read
        - --allow-net
```

### Environment Variables

- Development: Use `.env` files
- Production: Environment-based configuration
- **NEVER** commit API keys or secrets

## API Development

- RESTful design with consistent responses
- API versioned at `/api/v0/`
- Swagger docs at `/swagger/index.html`
- Update annotations for API changes

## Workflow & Runtime

### Temporal Integration

- Automatic retry and error recovery
- Distributed workflow execution
- Built-in state tracking

### Deno Tool Execution

- Configurable permissions per project
- JSON-based stdin/stdout communication
- Process isolation with timeout handling

## Development Workflow

1. **Before commits:** Run `make fmt && make lint && make test`
2. **API changes:** Update Swagger annotations
3. **Schema changes:** Create database migrations
4. **New features:** Include comprehensive tests

## Debugging

- Use `--debug` flag for verbose logging
- Temporal Web UI for workflow inspection
- Check logs for Deno runtime errors
- Verify PostgreSQL connectivity
