# Runtime API Reference

This document provides a comprehensive reference for Compozy's runtime system APIs, interfaces, and types.

## Core Runtime Interface

### `Runtime`

The primary interface for executing tools in different JavaScript runtimes.

```go
type Runtime interface {
    ExecuteTool(ctx context.Context, toolID string, toolExecID core.ID, input *core.Input, env core.EnvMap) (*core.Output, error)
    ExecuteToolWithTimeout(ctx context.Context, toolID string, toolExecID core.ID, input *core.Input, env core.EnvMap, timeout time.Duration) (*core.Output, error)
    GetGlobalTimeout() time.Duration
}
```

#### Methods

##### `ExecuteTool`

Executes a tool with the global timeout configuration.

**Parameters:**

- `ctx context.Context` - Request context for cancellation and deadlines
- `toolID string` - Unique identifier for the tool to execute
- `toolExecID core.ID` - Unique execution ID for tracking this specific run
- `input *core.Input` - Input data for the tool (map[string]any)
- `env core.EnvMap` - Environment variables to set during execution

**Returns:**

- `*core.Output` - Tool execution results (map[string]any)
- `error` - Error if execution fails

**Example:**

```go
output, err := runtime.ExecuteTool(
    ctx,
    "weather_tool",
    core.MustNewID(),
    &core.Input{"city": "London"},
    core.EnvMap{"API_KEY": "your-key"},
)
```

##### `ExecuteToolWithTimeout`

Executes a tool with a custom timeout, overriding the global timeout.

**Parameters:**

- Same as `ExecuteTool` plus:
- `timeout time.Duration` - Custom timeout for this execution

**Returns:**

- Same as `ExecuteTool`

**Example:**

```go
output, err := runtime.ExecuteToolWithTimeout(
    ctx,
    "slow_tool",
    core.MustNewID(),
    &core.Input{"data": largeDataset},
    core.EnvMap{},
    5*time.Minute, // Custom 5-minute timeout
)
```

##### `GetGlobalTimeout`

Returns the configured global timeout for tool execution.

**Returns:**

- `time.Duration` - Global timeout value

**Example:**

```go
timeout := runtime.GetGlobalTimeout()
fmt.Printf("Global timeout: %v\n", timeout)
```

## Configuration Types

### `Config`

Runtime configuration structure.

```go
type Config struct {
    BackoffInitialInterval time.Duration
    BackoffMaxInterval     time.Duration
    BackoffMaxElapsedTime  time.Duration
    WorkerFilePerm         os.FileMode
    ToolExecutionTimeout   time.Duration
    RuntimeType            string
    EntrypointPath         string
    BunPermissions         []string
    NodeOptions            []string
}
```

#### Fields

- **`BackoffInitialInterval`** - Initial delay before first retry (default: 100ms)
- **`BackoffMaxInterval`** - Maximum delay between retries (default: 5s)
- **`BackoffMaxElapsedTime`** - Total time limit for all retries (default: 30s)
- **`WorkerFilePerm`** - File permissions for worker files (default: 0600)
- **`ToolExecutionTimeout`** - Global tool execution timeout (default: 60s)
- **`RuntimeType`** - Runtime type: "bun" or "node" (default: "bun")
- **`EntrypointPath`** - Path to entrypoint file
- **`BunPermissions`** - Bun-specific command line flags
- **`NodeOptions`** - Node.js-specific command line options

#### Configuration Functions

##### `DefaultConfig()`

Returns sensible default configuration.

```go
func DefaultConfig() *Config
```

**Example:**

```go
config := runtime.DefaultConfig()
// config.ToolExecutionTimeout == 60 * time.Second
// config.RuntimeType == "bun"
```

##### `TestConfig()`

Returns configuration optimized for testing.

```go
func TestConfig() *Config
```

**Example:**

```go
config := runtime.TestConfig()
// config.ToolExecutionTimeout == 5 * time.Second
// config.BackoffMaxElapsedTime == 1 * time.Second
```

## Configuration Options

Functional options for configuring the runtime.

### Timeout Options

```go
// WithToolExecutionTimeout sets the global tool execution timeout
func WithToolExecutionTimeout(timeout time.Duration) Option

// WithBackoffInitialInterval sets the initial backoff interval
func WithBackoffInitialInterval(interval time.Duration) Option

// WithBackoffMaxInterval sets the maximum backoff interval
func WithBackoffMaxInterval(interval time.Duration) Option

// WithBackoffMaxElapsedTime sets the maximum elapsed time for backoff
func WithBackoffMaxElapsedTime(elapsed time.Duration) Option
```

**Example:**

```go
manager, err := runtime.NewManager(ctx, workingDir,
    runtime.WithToolExecutionTimeout(2*time.Minute),
    runtime.WithBackoffMaxElapsedTime(10*time.Second),
)
```

### Runtime Selection Options

```go
// WithRuntimeType sets the runtime type
func WithRuntimeType(runtimeType string) Option

// WithEntrypointPath sets the entrypoint file path
func WithEntrypointPath(path string) Option

// WithBunPermissions sets Bun-specific permissions
func WithBunPermissions(permissions []string) Option

// WithNodeOptions sets Node.js-specific options
func WithNodeOptions(options []string) Option
```

**Example:**

```go
manager, err := runtime.NewManager(ctx, workingDir,
    runtime.WithRuntimeType(runtime.RuntimeTypeBun),
    runtime.WithEntrypointPath("./custom-entrypoint.ts"),
    runtime.WithBunPermissions([]string{"--allow-read", "--allow-net"}),
)
```

### Security Options

```go
// WithWorkerFilePerm sets the file permissions for worker files
func WithWorkerFilePerm(perm os.FileMode) Option
```

**Example:**

```go
manager, err := runtime.NewManager(ctx, workingDir,
    runtime.WithWorkerFilePerm(0644), // More permissive for shared environments
)
```

### Composite Options

```go
// WithConfig applies a complete configuration
func WithConfig(config *Config) Option

// WithTestConfig applies test-specific configuration
func WithTestConfig() Option
```

**Example:**

```go
// Using complete config
config := &runtime.Config{
    ToolExecutionTimeout: 30 * time.Second,
    RuntimeType:         runtime.RuntimeTypeBun,
    BunPermissions:      []string{"--allow-read"},
}
manager, err := runtime.NewManager(ctx, workingDir, runtime.WithConfig(config))

// Using test config
manager, err := runtime.NewManager(ctx, workingDir, runtime.WithTestConfig())
```

## Error Types

### `ToolExecutionError`

Structured error for tool execution failures.

```go
type ToolExecutionError struct {
    ToolID     string
    ToolExecID string
    Operation  string
    Err        error
}

func (e *ToolExecutionError) Error() string
func (e *ToolExecutionError) Unwrap() error
```

**Example:**

```go
var toolErr *runtime.ToolExecutionError
if errors.As(err, &toolErr) {
    fmt.Printf("Tool %s failed during %s: %v\n",
        toolErr.ToolID, toolErr.Operation, toolErr.Err)
}
```

### `ProcessError`

Structured error for runtime process issues.

```go
type ProcessError struct {
    Operation string
    Err       error
}

func (e *ProcessError) Error() string
func (e *ProcessError) Unwrap() error
```

**Example:**

```go
var procErr *runtime.ProcessError
if errors.As(err, &procErr) {
    fmt.Printf("Process %s failed: %v\n", procErr.Operation, procErr.Err)
}
```

## Data Types

### Tool Execution Types

```go
// ToolExecuteParams represents parameters for tool execution
type ToolExecuteParams struct {
    ToolID     string      `json:"tool_id"`
    ToolExecID string      `json:"tool_exec_id"`
    Input      *core.Input `json:"input"`
    Env        core.EnvMap `json:"env"`
}

// ToolExecuteResult represents the result of tool execution
type ToolExecuteResult = core.Output
```

### Core Types

```go
// Input type alias for tool input data
type Input = map[string]any

// Output type alias for tool output data
type Output = map[string]any

// EnvMap type alias for environment variables
type EnvMap = map[string]string

// ID represents a unique identifier
type ID string
```

## Constants

### Runtime Types

```go
const (
    RuntimeTypeBun  = "bun"   // Bun JavaScript runtime
    RuntimeTypeNode = "node"  // Node.js runtime (future)
)
```

### Security Limits

```go
const (
    // Maximum tool output size (10MB)
    MaxOutputSize = 10 * 1024 * 1024

    // Initial buffer size for output (4KB)
    InitialBufferSize = 4 * 1024

    // Key for primitive value wrapping
    PrimitiveValueKey = "value"
)
```

## Factory Functions

### `NewManager`

Creates a new runtime manager instance.

```go
func NewManager(ctx context.Context, projectRoot string, options ...Option) (Runtime, error)
```

**Parameters:**

- `ctx context.Context` - Context for initialization
- `projectRoot string` - Project root directory path
- `options ...Option` - Configuration options

**Returns:**

- `Runtime` - Runtime interface implementation
- `error` - Error if initialization fails

**Example:**

```go
manager, err := runtime.NewManager(
    context.Background(),
    "/path/to/project",
    runtime.WithToolExecutionTimeout(30*time.Second),
    runtime.WithBunPermissions([]string{"--allow-read", "--allow-net"}),
)
if err != nil {
    return fmt.Errorf("failed to create runtime manager: %w", err)
}
```

### `NewBunManager`

Creates a Bun-specific runtime manager.

```go
func NewBunManager(ctx context.Context, projectRoot string, options ...Option) (*BunManager, error)
```

**Example:**

```go
bunManager, err := runtime.NewBunManager(
    ctx,
    "/path/to/project",
    runtime.WithBunPermissions([]string{"--allow-read", "--allow-write"}),
)
```

## Utility Functions

### `IsBunAvailable`

Checks if Bun runtime is available on the system.

```go
func IsBunAvailable() bool
```

**Example:**

```go
if !runtime.IsBunAvailable() {
    return errors.New("Bun runtime is required but not installed")
}
```

## Tool Development API

While tools are written in TypeScript/JavaScript, they interface with the Go runtime through a structured protocol.

### Tool Function Signature

```typescript
// TypeScript tool function signature
export function tool_name(input: ToolInput): ToolOutput | Promise<ToolOutput>;

// Where:
type ToolInput = any; // JSON-serializable input
type ToolOutput = any; // JSON-serializable output
```

### Tool Communication Protocol

Tools communicate with the runtime through JSON messages:

#### Request Format

```json
{
    "tool_id": "weather_tool",
    "tool_exec_id": "uuid-string",
    "input": { "city": "London" },
    "env": { "API_KEY": "secret" },
    "timeout_ms": 60000
}
```

#### Response Format

```json
{
    "result": { "temperature": 20, "weather": "sunny" },
    "error": null,
    "metadata": {
        "tool_id": "weather_tool",
        "tool_exec_id": "uuid-string",
        "execution_time": 1250
    }
}
```

#### Error Response Format

```json
{
    "result": null,
    "error": {
        "message": "API key is required",
        "stack": "Error: API key is required\n    at...",
        "name": "ValidationError",
        "tool_id": "weather_tool",
        "tool_exec_id": "uuid-string",
        "timestamp": "2024-01-01T12:00:00.000Z"
    }
}
```

## Best Practices

### Error Handling

```go
// Proper error handling with type checking
output, err := runtime.ExecuteTool(ctx, toolID, execID, input, env)
if err != nil {
    var toolErr *runtime.ToolExecutionError
    if errors.As(err, &toolErr) {
        // Handle tool-specific error
        log.Printf("Tool %s failed: %v", toolErr.ToolID, toolErr.Err)
        return handleToolError(toolErr)
    }

    var procErr *runtime.ProcessError
    if errors.As(err, &procErr) {
        // Handle process error
        log.Printf("Runtime process failed: %v", procErr.Err)
        return handleProcessError(procErr)
    }

    // Handle generic error
    return fmt.Errorf("runtime error: %w", err)
}
```

### Timeout Management

```go
// Use appropriate timeouts for different tool types
func createRuntimeForToolType(toolType string) (Runtime, error) {
    var timeout time.Duration
    switch toolType {
    case "quick":
        timeout = 5 * time.Second
    case "standard":
        timeout = 30 * time.Second
    case "heavy":
        timeout = 5 * time.Minute
    default:
        timeout = 60 * time.Second
    }

    return runtime.NewManager(ctx, projectRoot,
        runtime.WithToolExecutionTimeout(timeout),
    )
}
```

### Security Configuration

```go
// Production security configuration
func createProductionRuntime() (Runtime, error) {
    return runtime.NewManager(ctx, projectRoot,
        runtime.WithRuntimeType(runtime.RuntimeTypeBun),
        runtime.WithBunPermissions([]string{
            "--allow-read",  // Minimal permissions
        }),
        runtime.WithWorkerFilePerm(0600), // Secure file permissions
        runtime.WithToolExecutionTimeout(30*time.Second), // Shorter timeout
    )
}

// Development configuration
func createDevelopmentRuntime() (Runtime, error) {
    return runtime.NewManager(ctx, projectRoot,
        runtime.WithRuntimeType(runtime.RuntimeTypeBun),
        runtime.WithBunPermissions([]string{
            "--allow-read",
            "--allow-write",
            "--allow-net",
        }),
        runtime.WithToolExecutionTimeout(120*time.Second), // Longer for debugging
    )
}
```

For implementation examples and advanced usage patterns, see the [Runtime Configuration Guide](runtime-configuration.md) and [Entrypoint Pattern Guide](runtime-entrypoint.md).
