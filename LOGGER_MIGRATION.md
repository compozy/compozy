# Logger Migration to Context-Based Approach

## Overview

This document outlines the necessary changes to migrate from the global logger variable to a context-based approach using the new `logger.Logger` interface and context propagation.

## Key Changes in Logger Package

The logger package has been refactored to:

1. Remove global state variables
2. Introduce a `Logger` interface
3. Provide a `NewLogger(cfg *Config)` constructor
4. Support test environments with `TestConfig()`
5. **Context Integration**: Add `ContextWithLogger` and `LoggerFromContext` functions for context-based logger propagation

### New Context Functions

```go
// ContextKey is an alias for string type
type ContextKey string

const (
    // LoggerCtxKey is the string used to extract logger
    LoggerCtxKey ContextKey = "logger"
)

// ContextWithLogger stores a logger in the context
func ContextWithLogger(ctx context.Context, l Logger) context.Context

// LoggerFromContext retrieves a logger from the context, returning a default logger if none is found
func LoggerFromContext(ctx context.Context) Logger
```

### Critical Interface Fix Required

The current `Logger` interface has a DIP violation that must be fixed:

```go
// Current (WRONG - violates DIP)
type Logger interface {
    Debug(msg string, keyvals ...any)
    Info(msg string, keyvals ...any)
    Warn(msg string, keyvals ...any)
    Error(msg string, keyvals ...any)
    With(args ...any) *charmlog.Logger // ❌ Returns concrete type
}

// Fixed (CORRECT - follows DIP)
type Logger interface {
    Debug(msg string, keyvals ...any)
    Info(msg string, keyvals ...any)
    Warn(msg string, keyvals ...any)
    Error(msg string, keyvals ...any)
    With(args ...any) Logger // ✅ Returns interface type
}
```

The `loggerImpl` implementation must also be updated to return the interface type.

## Migration Strategy

### 1. Core Principles

- **Context propagation**: Logger is propagated through context using `context.Context` as the first parameter
- **Use interface type**: Always use `logger.Logger` interface, not concrete types
- **Context injection**: Pass logger through context using `ContextWithLogger`
- **Context extraction**: Extract logger using `LoggerFromContext`
- **Test logger**: Use `logger.NewLogger(logger.TestConfig())` in tests
- **Idiomatic Go**: Follow Go conventions with `context.Context` as first parameter

### 2. Files Requiring Updates

Based on the codebase analysis, the following files need updates:

#### CLI Layer

- `cli/dev.go` - ✅ **COMPLETED** - Uses context-based logger for file watching and server lifecycle
- `cli/mcp_proxy.go` - ✅ **COMPLETED** - Context-based logger usage for MCP proxy operations

#### Infrastructure Layer

- `engine/infra/cache/redis.go` - Redis client logging
- `engine/infra/server/mod.go` - Server initialization and lifecycle
- `engine/infra/server/register.go` - Route registration
- `engine/infra/server/config/service.go` - Configuration service
- `engine/infra/server/middleware.go` - HTTP middleware logging
- `engine/infra/server/router/response.go` - Response handling
- `engine/infra/server/router/helpers.go` - Router helpers
- `engine/infra/monitoring/` - All monitoring-related files
    - `engine/infra/monitoring/middleware/gin.go`
    - `engine/infra/monitoring/interceptor/temporal.go`
    - `engine/infra/monitoring/config.go`
    - `engine/infra/monitoring/system.go`
    - `engine/infra/monitoring/monitoring.go`
    - `engine/infra/monitoring/monitoring_test.go`
- `engine/infra/store/` - Database repositories
    - `engine/infra/store/taskrepo.go`
    - `engine/infra/store/workflowrepo.go`
    - `engine/infra/store/db.go`

#### Engine Layer

- `engine/autoload/autoload.go` - Auto-loading functionality
- `engine/llm/` - LLM service and related components
    - `engine/llm/prompt_builder.go`
    - `engine/llm/service.go`
    - `engine/llm/tool_registry.go`
    - `engine/llm/adapter/langchain_adapter_test.go`
    - `engine/llm/orchestrator.go`
    - `engine/llm/proxy_tool_test.go`
- `engine/core/subject.go` - Core subject logging
- `engine/runtime/` - Runtime and types
    - `engine/runtime/runtime.go`
    - `engine/runtime/types.go`
- `engine/mcp/service.go` - MCP service
- `engine/worker/` - Worker components

#### Package Layer

- `pkg/mcp-proxy/` - MCP proxy package
    - `pkg/mcp-proxy/admin_handlers.go`
    - `pkg/mcp-proxy/client_manager.go`

#### Test Files

- `test/integration/repo/task_test.go`
- `test/helpers/helpers.go`
- Various `*_test.go` files

### 3. Implementation Patterns

#### Pattern A: Service/Repository with Context-Based Logger

```go
// Before (using global logger)
type Service struct {
    db *sql.DB
}

func (s *Service) DoSomething() {
    logger.Info("doing something")
}

// After (with context-based logger)
type Service struct {
    db *sql.DB
}

func NewService(db *sql.DB) *Service {
    return &Service{
        db: db,
    }
}

func (s *Service) DoSomething(ctx context.Context) {
    log := logger.LoggerFromContext(ctx)
    log.Info("doing something")
}
```

#### Pattern B: HTTP Handler/Middleware with Context

```go
// Before
func LoggerMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        logger.Info("request", "path", c.Request.URL.Path)
        c.Next()
    }
}

// After
func LoggerMiddleware(log logger.Logger) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Add logger to Gin context
        ctx := logger.ContextWithLogger(c.Request.Context(), log)
        c.Request = c.Request.WithContext(ctx)

        log := logger.LoggerFromContext(c.Request.Context())
        log.Info("request", "path", c.Request.URL.Path)
        c.Next()
    }
}

// In handlers
func GetItemHandler(c *gin.Context) {
    log := logger.LoggerFromContext(c.Request.Context())
    log.Info("handling get item request")
    // ... handler logic
}
```

#### Pattern C: Worker/Background Tasks with Context

```go
// Before
func (w *Worker) Start() {
    logger.Info("worker starting")
}

// After
type Worker struct {
    // other fields, no logger field needed
}

func NewWorker(/* other deps */) *Worker {
    return &Worker{
        // other fields
    }
}

func (w *Worker) Start(ctx context.Context) {
    log := logger.LoggerFromContext(ctx)
    log.Info("worker starting")
}
```

#### Pattern D: Service Initialization with Context

```go
// In main.go or CLI command
func runCommand(cmd *cobra.Command, args []string) error {
    // Initialize logger
    cfg := &logger.Config{
        Level:     logLevel,
        JSON:      logJSON,
        AddSource: logSource,
    }
    log := logger.NewLogger(cfg)

    // Create context with logger
    ctx := context.Background()
    ctx = logger.ContextWithLogger(ctx, log)

    // Pass context to services
    server := server.NewServer(serverConfig, temporalConfig, storeConfig)
    return server.Run(ctx)
}
```

#### Pattern E: Contextual Logging

```go
// Add service-specific context to all logs from a component
func NewTaskService(repo TaskRepository) *TaskService {
    return &TaskService{
        repo: repo,
    }
}

// Create contextual logger once and use throughout the operation
func (s *TaskService) ExecuteTask(ctx context.Context, id core.ID) error {
    log := logger.LoggerFromContext(ctx).With("service", "TaskService", "task_id", id)
    log.Info("executing task")

    // For operations that need the contextual logger, create new context
    ctxWithLogger := logger.ContextWithLogger(ctx, log)
    return s.processTask(ctxWithLogger, id)
}

func (s *TaskService) processTask(ctx context.Context, id core.ID) error {
    log := logger.LoggerFromContext(ctx) // Already has service and task_id context
    log.Debug("processing task steps")
    // ...
}
```

### 4. Specific File Changes

#### `cli/dev.go` ✅ **COMPLETED**

```go
func handleDevCmd(cmd *cobra.Command, _ []string) error {
    // Initialize logger
    logLevel, logJSON, logSource, err := logger.GetLoggerConfig(cmd)
    if err != nil {
        return err
    }
    log := logger.SetupLogger(logLevel, logJSON, logSource)

    // Create context with logger
    ctx := context.Background()
    ctx = logger.ContextWithLogger(ctx, log)

    // Pass context to server and other components
    server := server.NewServer(serverConfig, temporalConfig, storeConfig)
    return server.Run(ctx)
}

func setupWatcher(ctx context.Context, cwd string) (*fsnotify.Watcher, error) {
    log := logger.LoggerFromContext(ctx)
    // ... use log instead of global logger
}
```

#### `engine/infra/server/mod.go`

```go
type Server struct {
    Config         *Config
    TemporalConfig *worker.TemporalConfig
    StoreConfig    *store.Config
    // ... other fields, no logger field needed
}

func NewServer(config *Config, tConfig *worker.TemporalConfig, sConfig *store.Config) *Server {
    ctx, cancel := context.WithCancel(context.Background())
    return &Server{
        Config:         config,
        TemporalConfig: tConfig,
        StoreConfig:    sConfig,
        ctx:            ctx,
        cancel:         cancel,
        // ... other fields
    }
}

// Context-first parameter
func (s *Server) Run(ctx context.Context) error {
    log := logger.LoggerFromContext(ctx)
    log.Info("starting server")

    // Propagate context to dependencies
    return s.setupDependencies(ctx)
}

func (s *Server) Shutdown(ctx context.Context) error {
    log := logger.LoggerFromContext(ctx)
    log.Info("shutting down server")
    s.cancel()
    return nil
}

func (s *Server) setupDependencies(ctx context.Context) error {
    log := logger.LoggerFromContext(ctx)

    configService := csvc.NewService(s.Config.EnvFilePath)
    // Initialize configService with context when calling methods
    err := configService.LoadConfig(ctx)
    // ... rest of the code
}
```

#### `engine/infra/monitoring/monitoring.go`

```go
type Service struct {
    // ... other fields, no logger field needed
}

func NewMonitoringService(cfg MonitoringConfig) (*Service, error) {
    return &Service{
        // ... other fields, no logger field
    }, nil
}

func (s *Service) Start(ctx context.Context) error {
    log := logger.LoggerFromContext(ctx)
    log.Info("starting monitoring service")
    // ... implementation
}
```

### 5. Testing Updates

All test files should use the test logger configuration with context:

```go
func TestSomething(t *testing.T) {
    // Create test logger and context
    testLogger := logger.NewLogger(logger.TestConfig())
    ctx := context.Background()
    ctx = logger.ContextWithLogger(ctx, testLogger)

    // Pass context to service under test
    service := NewService(/* other deps */)

    // Call methods with context
    err := service.DoSomething(ctx)
    assert.NoError(t, err)
}
```

### 6. Context Propagation Graph

The logger flows through the application via context:

```
main.go
  └─> cli/dev.go (creates logger + context)
       └─> server.Run(ctx)
            ├─> config.Service.LoadConfig(ctx)
            ├─> monitoring.Service.Start(ctx)
            ├─> store.SetupStore(ctx)
            │    ├─> WorkflowRepo.methods(ctx, ...)
            │    └─> TaskRepo.methods(ctx, ...)
            └─> worker.NewWorker().Start(ctx)
                 ├─> llm.Service.methods(ctx, ...)
                 ├─> mcp.Service.methods(ctx, ...)
                 └─> runtime.Runtime.methods(ctx, ...)
```

### 7. Migration Steps

1. **Add context import**: Add `"context"` import to all files that will use logger
2. **Update method signatures**: Add `ctx context.Context` as first parameter to all methods that need logging
3. **Extract logger from context**: Replace `logger.Info()` with `log := logger.LoggerFromContext(ctx); log.Info()`
4. **Propagate context**: Pass context through the call chain
5. **Update constructors**: Remove logger parameters from constructors (context provides logger)
6. **Update tests**: Create context with test logger in all test setups
7. **Update entry points**: Create logger and context at CLI/main level
8. **Remove global usage**: Remove any remaining global logger usage
9. **Add contextual logging**: Use `logger.With()` to add service-specific context
10. **Reduce log verbosity**: Remove unnecessary logs and optimize logging levels

### 8. Context-Based Patterns

#### HTTP Context Pattern

```go
// Middleware adds logger to request context
func LoggerMiddleware(log logger.Logger) gin.HandlerFunc {
    return func(c *gin.Context) {
        ctx := logger.ContextWithLogger(c.Request.Context(), log)
        c.Request = c.Request.WithContext(ctx)
        c.Next()
    }
}

// Handlers extract logger from context
func HandlerFunc(c *gin.Context) {
    log := logger.LoggerFromContext(c.Request.Context())
    log.Info("handling request")
}
```

#### Background Worker Pattern

```go
func (w *Worker) processJob(ctx context.Context, job Job) error {
    log := logger.LoggerFromContext(ctx).With("job_id", job.ID, "worker", "JobWorker")

    // Create new context with enriched logger
    jobCtx := logger.ContextWithLogger(ctx, log)

    log.Info("processing job")
    return w.executeSteps(jobCtx, job)
}
```

#### Database Operation Pattern

```go
func (r *Repository) Save(ctx context.Context, entity *Entity) error {
    log := logger.LoggerFromContext(ctx)

    log.Debug("saving entity", "id", entity.ID)
    if err := r.db.Save(entity); err != nil {
        log.Error("failed to save entity", "id", entity.ID, "error", err)
        return err
    }
    return nil
}
```

### 9. Log Optimization Guidelines

During the migration, apply these logging best practices to reduce verbosity:

#### Remove Unnecessary Logs

- **Remove "happy path" logs**: Don't log successful operations unless they're significant
- **Remove redundant logs**: If an operation is logged at a higher level, don't log it again at lower levels
- **Remove development/debug logs**: Convert temporary debugging logs to Debug level or remove them

#### Optimize Log Levels

```go
// ❌ Bad: Too verbose
func (s *Service) GetItem(ctx context.Context, id string) (*Item, error) {
    log := logger.LoggerFromContext(ctx)
    log.Info("Getting item", "id", id)
    item, err := s.repo.Find(ctx, id)
    if err != nil {
        log.Error("Failed to get item", "id", id, "error", err)
        return nil, err
    }
    log.Info("Successfully got item", "id", id)
    return item, nil
}

// ✅ Good: Only log significant events and errors
func (s *Service) GetItem(ctx context.Context, id string) (*Item, error) {
    log := logger.LoggerFromContext(ctx)
    item, err := s.repo.Find(ctx, id)
    if err != nil {
        log.Error("Failed to get item", "id", id, "error", err)
        return nil, err
    }
    return item, nil
}
```

#### Context Enrichment Pattern

```go
// Create rich context at service boundaries
func (s *UserService) ProcessUser(ctx context.Context, userID string) error {
    log := logger.LoggerFromContext(ctx).With(
        "service", "UserService",
        "user_id", userID,
        "operation", "ProcessUser",
    )

    // Create enriched context for downstream calls
    enrichedCtx := logger.ContextWithLogger(ctx, log)

    log.Info("processing user")
    return s.doProcessUser(enrichedCtx, userID)
}

func (s *UserService) doProcessUser(ctx context.Context, userID string) error {
    log := logger.LoggerFromContext(ctx) // Already enriched with service context

    // Only log significant events - context already has user_id
    if err := s.validateUser(ctx, userID); err != nil {
        log.Error("user validation failed", "error", err)
        return err
    }

    return s.updateUser(ctx, userID)
}
```

### 10. Specific Build Errors to Fix

Based on the build output, here are the specific files and functions that need context-based updates:

#### Files with Global Logger Calls

1. **engine/infra/monitoring/middleware/gin.go**

    ```go
    // Before
    func MonitoringMiddleware() gin.HandlerFunc {
        return func(c *gin.Context) {
            logger.Error("monitoring error")
        }
    }

    // After
    func MonitoringMiddleware(log logger.Logger) gin.HandlerFunc {
        return func(c *gin.Context) {
            ctx := logger.ContextWithLogger(c.Request.Context(), log)
            c.Request = c.Request.WithContext(ctx)

            log := logger.LoggerFromContext(c.Request.Context())
            log.Error("monitoring error")
        }
    }
    ```

2. **engine/infra/cache/redis.go**

    ```go
    // Add context parameter to methods
    func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
        log := logger.LoggerFromContext(ctx)
        log.Debug("getting key from redis", "key", key)
        // ... implementation
    }
    ```

3. **pkg/mcp-proxy/** (multiple files)
    ```go
    // Update all handler methods to accept context and extract logger
    func (h *AdminHandler) HandleRequest(ctx context.Context, req *Request) error {
        log := logger.LoggerFromContext(ctx)
        log.Info("handling admin request")
        // ... implementation
    }
    ```

#### Special Cases

##### Runtime SafeLogger with Context

```go
// safeLogger provides a nil-safe wrapper that works with context
type safeLogger struct {
    logger logger.Logger
}

func (s *safeLogger) Debug(msg string, keyvals ...any) {
    if s.logger == nil {
        return
    }
    s.logger.Debug(msg, keyvals...)
}

func (s *safeLogger) Info(msg string, keyvals ...any) {
    if s.logger == nil {
        return
    }
    s.logger.Info(msg, keyvals...)
}

func (s *safeLogger) Warn(msg string, keyvals ...any) {
    if s.logger == nil {
        return
    }
    s.logger.Warn(msg, keyvals...)
}

func (s *safeLogger) Error(msg string, keyvals ...any) {
    if s.logger == nil {
        return
    }
    s.logger.Error(msg, keyvals...)
}

func (s *safeLogger) With(args ...any) logger.Logger {
    if s.logger == nil {
        return s
    }
    return &safeLogger{logger: s.logger.With(args...)}
}

// Usage in runtime
func (r *Runtime) ExecuteTool(ctx context.Context, toolName string, input map[string]any) (map[string]any, error) {
    // Try to get logger from context, fallback to safe logger
    var log logger.Logger
    if ctxLogger := logger.LoggerFromContext(ctx); ctxLogger != nil {
        log = ctxLogger
    } else {
        log = &safeLogger{logger: nil} // Nil-safe fallback
    }

    log.Info("executing tool", "tool", toolName)
    // ... implementation
}
```

### 11. Validation

After migration:

- All methods that need logging should accept `context.Context` as first parameter
- No files should import logger and use it globally
- All logger usage should be through `logger.LoggerFromContext(ctx)`
- All tests should pass with test logger configuration in context
- No build errors related to undefined logger references
- All `logger.GetDefault()` calls removed
- `safeLogger` implements full `Logger` interface

### 12. Benefits of Context-Based Approach

- **Idiomatic Go**: Follows Go conventions with context as first parameter
- **No parameter pollution**: Don't need to add logger to every struct
- **Thread safety**: Context is designed for concurrent use
- **Testability**: Easy to provide test loggers through context
- **Flexibility**: Different parts of request can have different logger configurations
- **Request tracing**: Can add request IDs and other contextual information
- **Explicit propagation**: Clear path of context through call stack
- **No dependency explosion**: Services don't need logger in constructors

### 13. Important Notes

1. **Context First**: Always use `context.Context` as the first parameter
2. **Logger Extraction**: Use `logger.LoggerFromContext(ctx)` to get the logger
3. **Context Enrichment**: Use `logger.ContextWithLogger(ctx, enrichedLogger)` to add context
4. **Nil Safety**: Handle cases where no logger is in context gracefully
5. **Test Configuration**: Always use `logger.TestConfig()` in tests
6. **HTTP Integration**: Add logger to request context in middleware
7. **Background Jobs**: Create context with logger for background operations

### 14. Context Propagation Rules

1. **Entry Points**: CLI commands and HTTP handlers create initial context with logger
2. **Service Boundaries**: Services receive context and extract logger
3. **Method Calls**: Pass context through method calls
4. **Background Operations**: Create new context with logger for goroutines
5. **HTTP Requests**: Use request context for handlers
6. **Database Operations**: Pass context to repository methods
7. **Testing**: Create context with test logger in test setup

### 15. Compliance with Project Standards

This migration follows:

- **SOLID Principles**: Especially DIP (Dependency Inversion Principle) from architecture.mdc
- **Go Context Patterns**: Context as first parameter following Go conventions
- **Testing Standards**: Using testify patterns from testing-standards.mdc
- **Core Libraries**: Using charmbracelet/log as required by core-libraries.mdc
- **Resource Management**: Proper cleanup patterns from go-patterns.mdc
- **API Standards**: Context propagation through HTTP middleware from api-standards.mdc
