# Compozy Code Quality Rules (Extracted from AGENT.md)

## 🚨 CRITICAL: Testing Standards

### MANDATORY Test Patterns

- **ALL tests MUST use `t.Run("Should...")` pattern**
- **AVOID** table-driven tests unless 5+ similar variations
- Place `*_test.go` files alongside implementation files
- Use `stretchr/testify` for assertions and mocks
- Each test must be independent and repeatable
- Mock external dependencies

```go
// ✅ REQUIRED PATTERN
func TestService_Method(t *testing.T) {
    t.Run("Should handle success case", func(t *testing.T) {
        // test implementation
    })
    t.Run("Should handle error case", func(t *testing.T) {
        // test implementation
    })
}

// ❌ FORBIDDEN - Direct test without t.Run
func TestService_Method(t *testing.T) {
    // direct implementation
}
```

## Linting Requirements

### Code Metrics

- **Function length:** Max 80 lines OR 50 statements
- **Line length:** Max 120 characters
- **Cyclomatic complexity:** Max 15
- **Error handling:** ALL errors must be checked

## Error Handling Patterns

### Required Error Patterns

- **Custom errors:** `core.NewError(err, "CODE", details)`
- **Error wrapping:** `fmt.Errorf("context: %w", err)`
- **Transaction pattern:**

```go
defer func() {
    if err != nil { tx.Rollback(ctx) } else { tx.Commit(ctx) }
}()
```

## Core Patterns & Conventions

### Interface Design

- **Small, focused interfaces** (single responsibility)

```go
type Storage interface {
    SaveMCP(ctx context.Context, def *MCPDefinition) error
    LoadMCP(ctx context.Context, name string) (*MCPDefinition, error)
    Close() error
}
```

### Concurrency Patterns

- **Thread-safe structs:** Embed `sync.RWMutex` and protect all fields
- **Concurrent operations:** Use `errgroup.WithContext(ctx)`

```go
type Status struct {
    Name string
    mu   sync.RWMutex // Protects all fields
}

g, ctx := errgroup.WithContext(ctx)
for _, item := range items {
    item := item // capture loop variable
    g.Go(func() error { return process(ctx, item) })
}
```

### Factory Pattern

- **Switch-based constructors** for different implementations

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

### Configuration with Defaults

- **Always provide defaults** for nil configs

```go
func NewService(config *Config) *Service {
    if config == nil {
        config = DefaultConfig()
    }
}
```

### Graceful Shutdown

- **Signal handling pattern:**

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

### Middleware Pattern

- **Gin middleware structure:**

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

### Resource Management

- **Connection limits:** Check max connections before creating
- **Cleanup with defer:** Always use defer for cleanup

```go
if len(m.clients) >= m.config.MaxConnections {
    return fmt.Errorf("max connections reached")
}

defer func() {
    m.cancel()
    m.wg.Wait()
}()
```

## Required Libraries

### MANDATORY Library Usage

- **Web:** `gin-gonic/gin` for HTTP APIs
- **DB:** `jackc/pgx/v5` (PostgreSQL), `redis/go-redis/v9`
- **Testing:** `stretchr/testify` for assertions/mocks
- **Validation:** `go-playground/validator/v10`
- **Logging:** `charmbracelet/log` - use `logger.Info/Error/Debug`
- **CLI:** `spf13/cobra` for commands
- **Docs:** `swaggo/swag` for API documentation

## Project Utilities

### Required Project-Specific Usage

- **Logger:** Use `pkg/logger` for structured logging

```go
logger.Info("task executed", "task_id", id, "duration", time.Since(start))
logger.Error("execution failed", "error", err, "task_id", id)
```

- **Core types:** `core.ID` (UUIDs), `core.Ref` (polymorphic refs)
- **Test helpers:** `utils.SetupTest()`, `utils.SetupFixture()`
- **Template engine:** `pkg/tplengine` for dynamic configs

## Security & Configuration

### FORBIDDEN Practices

- **NEVER** commit API keys or secrets
- **NEVER** log secrets or sensitive data

### Required Practices

- Use `.env` files for development
- Environment-based configuration for production
- RESTful design with consistent responses
- API versioned at `/api/v0/`
- Update Swagger annotations for API changes

## Development Workflow Requirements

### Pre-commit Requirements

- **MUST run:** `make fmt && make lint && make test`
- **API changes:** Update Swagger annotations
- **Schema changes:** Create database migrations
- **New features:** Include comprehensive tests

## Naming Conventions

### File Organization

- `*_test.go` files alongside implementation
- Package structure follows domain boundaries
- Clear separation of concerns (engine/, cli/, pkg/)

### Code Style

- Use structured logging with key-value pairs
- Context propagation through all functions
- Consistent error handling patterns
- Resource cleanup with defer statements
