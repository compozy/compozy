# Compozy Development Guide

Compozy is a **workflow orchestration engine for AI agents** that enables building AI-powered applications through declarative YAML configuration and a robust Go backend. It integrates with various LLM providers and supports the Model Context Protocol (MCP) for extending AI capabilities.

## Quick Start

```bash
# Setup
make deps && make start-docker && make migrate-up

# Development
make dev              # Start development server
make test             # Run tests (excludes slow tests)
make test-all         # Full test suite including E2E
make fmt && make lint # Format and lint code

# Run specific test
go test -v ./engine/task -run TestExecutor_Execute
```

## Architecture

```
compozy/
├── engine/           # Core domain logic
│   ├── agent/        # AI agent management
│   ├── task/         # Task orchestration (basic, parallel, collection, router types)
│   ├── tool/         # Tool execution framework (TypeScript/Deno-based)
│   ├── workflow/     # Workflow definition and execution
│   ├── mcp/          # Model Context Protocol integration for external tool servers
│   ├── llm/          # LLM service integration (OpenAI, Groq, Ollama)
│   ├── runtime/      # Deno runtime for executing TypeScript tools
│   ├── worker/       # Temporal-based workflow execution
│   └── infra/        # Infrastructure (server, db, messaging)
├── cli/              # Command-line interface
├── pkg/              # Reusable packages (mcp-proxy, utils, logger, tplengine)
└── test/             # Test suite
```

**Tech Stack:**

- **Go 1.24+**: Core language
- **PostgreSQL**: Main database (5432) + Temporal database (5433)
- **Redis**: Caching, config storage, and pub/sub (6379)
- **Temporal**: Workflow orchestration (7233, UI: 8080)
- **MCP Proxy**: HTTP proxy for MCP servers (8081)
- **NATS**: Messaging system
- **Deno**: Runtime for TypeScript tools

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
    default:
        return nil, fmt.Errorf("unsupported storage type: %s", config.Type)
    }
}
```

**Configuration with Defaults:**

```go
func NewService(config *Config) *Service {
    if config == nil {
        config = DefaultConfig() // Always provide defaults
    }
    return &Service{config: config}
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
    if closeErr := m.conn.Close(); closeErr != nil {
        logger.Error("failed to close connection", "error", closeErr)
    }
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

    // NEVER log sensitive data
    logger.Info("user authenticated", "user_id", userID) // ✅ Good
    logger.Info("user authenticated", "password", pass) // ❌ Never do this
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

mcps:
    - id: my-mcp-server
      url: http://localhost:3000/mcp
      transport: sse

runtime:
    permissions:
        - --allow-read
        - --allow-net
```

### Environment Variables

- Development: Use `.env` files
- Production: Environment-based configuration
- **CRITICAL:** Never commit API keys or secrets

## API Development

- RESTful design with consistent responses
- API versioned at `/api/v0/`
- Swagger docs at `/swagger/index.html`
- Update annotations for API changes

## MCP Integration

The MCP (Model Context Protocol) integration enables external tool servers to be used with Compozy:

- **engine/mcp/**: MCP client implementation
- **pkg/mcp-proxy/**: HTTP proxy for MCP servers (runs on port 8081)
- **engine/llm/proxy_tool.go**: Tool for proxying MCP calls

MCP servers are configured in YAML under the `mcps` section:

```yaml
mcps:
    - id: search-mcp
      url: http://localhost:3000
      transport: sse
      env:
          API_KEY: "{{ .env.SEARCH_API_KEY }}"
```

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
3. **Schema changes:** Create migrations with `make migrate-create name=<name>`
4. **New features:** Include comprehensive tests following the mandatory pattern

## Debugging

- Use `--debug` flag for verbose logging
- Temporal Web UI for workflow inspection (port 8080)
- Check logs for Deno runtime errors
- Verify PostgreSQL connectivity
