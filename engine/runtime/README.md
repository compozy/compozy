# `engine/runtime` – _JavaScript/TypeScript Runtime Management_

> **Secure, high-performance runtime system for executing JavaScript/TypeScript tools in AI workflows with sandboxing, timeout management, and multi-runtime support.**

---

## 📑 Table of Contents

- [🎯 Overview](#-overview)
- [💡 Motivation](#-motivation)
- [⚡ Design Highlights](#-design-highlights)
- [🚀 Getting Started](#-getting-started)
- [📖 Usage](#-usage)
  - [Library](#library)
  - [Factory Pattern](#factory-pattern)
  - [Runtime Configuration](#runtime-configuration)
  - [Tool Execution](#tool-execution)
- [🔧 Configuration](#-configuration)
- [🎨 Examples](#-examples)
- [📚 API Reference](#-api-reference)
- [🧪 Testing](#-testing)
- [📦 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🎯 Overview

The `engine/runtime` package provides a secure and efficient runtime system for executing JavaScript and TypeScript tools within Compozy AI workflows. It supports multiple runtime environments (Bun, Node.js) with comprehensive security sandboxing, timeout management, and error handling.

This package enables AI agents to execute custom tools while maintaining security boundaries and performance guarantees essential for production AI systems.

---

## 💡 Motivation

- **Security Isolation**: Sandboxed execution environment with granular permission controls
- **Performance Optimization**: High-performance runtime selection with fast startup times
- **Multi-Runtime Support**: Flexible runtime selection (Bun, Node.js) based on requirements
- **Robust Error Handling**: Comprehensive error handling and timeout management for production reliability

---

## ⚡ Design Highlights

- **Factory Pattern**: Flexible runtime creation with configuration-based selection
- **Security Sandboxing**: Comprehensive permission system with principle of least privilege
- **Timeout Management**: Configurable execution timeouts with graceful cancellation
- **Multi-Runtime Support**: Support for Bun and Node.js runtimes with runtime-specific optimizations
- **Error Handling**: Detailed error reporting with process-level error capture
- **Configuration-Driven**: Extensible configuration system with environment-based overrides

---

## 🚀 Getting Started

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/compozy/compozy/engine/runtime"
    "github.com/compozy/compozy/engine/core"
)

func main() {
    ctx := context.Background()
    projectRoot := "/path/to/project"

    // Create runtime factory
    factory := runtime.NewDefaultFactory(projectRoot)

    // Create runtime configuration
    config := &runtime.Config{
        RuntimeType:           "bun",
        EntrypointPath:        "./tools.ts",
        ToolExecutionTimeout:  30 * time.Second,
        BunPermissions: []string{
            "--allow-read",
            "--allow-net=api.example.com",
        },
    }

    // Create runtime instance
    rt, err := factory.CreateRuntime(ctx, config)
    if err != nil {
        log.Fatal(err)
    }

    // Execute a tool
    input := &core.Input{
        Raw: map[string]any{
            "message": "Hello, world!",
        },
    }

    env := core.EnvMap{
        "API_KEY": "secret-key",
    }

    result, err := rt.ExecuteTool(
        ctx,
        "greet",           // Tool ID
        core.NewID(),      // Execution ID
        input,             // Input data
        env,               // Environment variables
    )
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Tool executed successfully: %+v", result)
}
```

---

## 📖 Usage

### Library

The runtime package provides a clean interface for tool execution:

```go
// Create factory
factory := runtime.NewDefaultFactory("/path/to/project")

// Create runtime with configuration
config := runtime.DefaultConfig()
config.RuntimeType = "bun"
config.EntrypointPath = "./tools.ts"

rt, err := factory.CreateRuntime(ctx, config)
if err != nil {
    return err
}

// Execute tool
result, err := rt.ExecuteTool(ctx, "myTool", execID, input, env)
```

### Factory Pattern

The factory pattern provides flexible runtime creation:

```go
// Default factory
factory := runtime.NewDefaultFactory(projectRoot)

// Create runtime based on configuration
runtime, err := factory.CreateRuntime(ctx, config)
if err != nil {
    return err
}

// Factory automatically selects appropriate runtime implementation
// based on config.RuntimeType ("bun" or "node")
```

### Runtime Configuration

Comprehensive configuration options:

```go
// Create custom configuration
config := &runtime.Config{
    RuntimeType:           "bun",
    EntrypointPath:        "./tools.ts",
    ToolExecutionTimeout:  45 * time.Second,
    WorkerFilePerm:        0644,

    // Bun-specific permissions
    BunPermissions: []string{
        "--allow-read=/data",
        "--allow-net=api.company.com",
        "--allow-env=API_KEY,DATABASE_URL",
    },

    // Node.js-specific options
    NodeOptions: []string{
        "--max-old-space-size=2048",
        "--experimental-modules",
    },

    // Backoff configuration for retries
    BackoffInitialInterval: 100 * time.Millisecond,
    BackoffMaxInterval:     5 * time.Second,
    BackoffMaxElapsedTime:  30 * time.Second,
}

// Use predefined configurations
config := runtime.DefaultConfig()  // Production defaults
config := runtime.TestConfig()     // Test environment
```

### Tool Execution

Execute tools with flexible timeout and environment support:

```go
// Execute with default timeout
result, err := runtime.ExecuteTool(
    ctx,
    "dataProcessor",
    execID,
    input,
    env,
)

// Execute with custom timeout
result, err := runtime.ExecuteToolWithTimeout(
    ctx,
    "slowTool",
    execID,
    input,
    env,
    60 * time.Second,  // Custom timeout
)

// Check global timeout
timeout := runtime.GetGlobalTimeout()
fmt.Printf("Global timeout: %v", timeout)
```

---

## 🔧 Configuration

### Runtime Options

```go
// Direct configuration approach
config := &runtime.Config{
    RuntimeType:            runtime.RuntimeTypeBun,
    EntrypointPath:         "./tools.ts",
    ToolExecutionTimeout:   30 * time.Second,
    BunPermissions: []string{
        "--allow-read",
        "--allow-net=api.example.com",
    },
    NodeOptions: []string{
        "--max-old-space-size=2048",
    },
    BackoffInitialInterval: 100 * time.Millisecond,
    BackoffMaxInterval:     5 * time.Second,
    BackoffMaxElapsedTime:  30 * time.Second,
}

runtime.NewBunManager(ctx, projectRoot, config)

// Or use defaults
runtime.NewBunManager(ctx, projectRoot, nil) // Uses DefaultConfig()

// Or from app config
appConfig := &appconfig.RuntimeConfig{
    Environment:          "production",
    ToolExecutionTimeout: 30 * time.Second,
}
runtimeConfig := runtime.FromAppConfig(appConfig)
runtime.NewBunManager(ctx, projectRoot, runtimeConfig)
```

### Security Configuration

```go
// Restrictive permissions (recommended)
config := &runtime.Config{
    BunPermissions: []string{
        "--allow-read=/data",               // Read access to specific directory
        "--allow-net=api.trusted.com",     // Network access to specific host
        "--allow-env=API_KEY,DATABASE_URL", // Specific environment variables
    },
}

// Permissive permissions (development only)
config := &runtime.Config{
    BunPermissions: []string{
        "--allow-read",  // Read access to all files
        "--allow-net",   // Network access to all hosts
        "--allow-env",   // Access to all environment variables
    },
}
```

### Environment Variables

```bash
# Runtime configuration via environment
COMPOZY_RUNTIME_TYPE=bun
COMPOZY_ENTRYPOINT=./tools.ts
COMPOZY_TOOL_TIMEOUT=30s

# Security permissions
COMPOZY_BUN_PERMISSIONS=--allow-read,--allow-net=api.example.com
COMPOZY_NODE_OPTIONS=--max-old-space-size=2048

# Backoff configuration
COMPOZY_BACKOFF_INITIAL_INTERVAL=100ms
COMPOZY_BACKOFF_MAX_INTERVAL=5s
COMPOZY_BACKOFF_MAX_ELAPSED_TIME=30s
```

---

## 🎨 Examples

### Basic Tool Execution

```go
func executeBasicTool() {
    ctx := context.Background()

    // Create runtime
    factory := runtime.NewDefaultFactory("/path/to/project")
    config := runtime.DefaultConfig()
    config.RuntimeType = "bun"
    config.EntrypointPath = "./tools.ts"

    rt, err := factory.CreateRuntime(ctx, config)
    if err != nil {
        panic(err)
    }

    // Prepare input
    input := &core.Input{
        Raw: map[string]any{
            "text": "Hello, world!",
            "options": map[string]any{
                "uppercase": true,
            },
        },
    }

    // Execute tool
    result, err := rt.ExecuteTool(
        ctx,
        "textProcessor",
        core.NewID(),
        input,
        core.EnvMap{},
    )
    if err != nil {
        panic(err)
    }

    fmt.Printf("Result: %+v\n", result)
}
```

### Secure Tool Execution

```go
func executeSecureTool() {
    ctx := context.Background()

    // Create runtime with restrictive permissions
    factory := runtime.NewDefaultFactory("/path/to/project")
    config := &runtime.Config{
        RuntimeType:           "bun",
        EntrypointPath:        "./tools.ts",
        ToolExecutionTimeout:  30 * time.Second,
        BunPermissions: []string{
            "--allow-read=/data/input",     // Only read from input directory
            "--allow-net=api.company.com",  // Only connect to company API
            "--allow-env=API_KEY",          // Only access specific env var
        },
    }

    rt, err := factory.CreateRuntime(ctx, config)
    if err != nil {
        panic(err)
    }

    // Execute tool with environment variables
    result, err := rt.ExecuteTool(
        ctx,
        "dataProcessor",
        core.NewID(),
        input,
        core.EnvMap{
            "API_KEY": "secret-api-key",
        },
    )
    if err != nil {
        panic(err)
    }

    fmt.Printf("Secure execution completed: %+v\n", result)
}
```

### Custom Timeout Handling

```go
func executeWithCustomTimeout() {
    ctx := context.Background()

    // Create runtime
    factory := runtime.NewDefaultFactory("/path/to/project")
    config := runtime.DefaultConfig()
    rt, err := factory.CreateRuntime(ctx, config)
    if err != nil {
        panic(err)
    }

    // Execute long-running tool with extended timeout
    result, err := rt.ExecuteToolWithTimeout(
        ctx,
        "longRunningTask",
        core.NewID(),
        input,
        env,
        5 * time.Minute,  // Extended timeout
    )
    if err != nil {
        if isTimeoutError(err) {
            fmt.Println("Tool execution timed out")
        } else {
            panic(err)
        }
        return
    }

    fmt.Printf("Long-running task completed: %+v\n", result)
}

func isTimeoutError(err error) bool {
    // Check if error is timeout-related
    return err != nil && (
        err == context.DeadlineExceeded ||
        strings.Contains(err.Error(), "timeout") ||
        strings.Contains(err.Error(), "deadline exceeded"))
}
```

### Multi-Runtime Support

```go
func demonstrateMultiRuntime() {
    ctx := context.Background()
    projectRoot := "/path/to/project"

    // Create Bun runtime
    bunConfig := &runtime.Config{
        RuntimeType:    "bun",
        EntrypointPath: "./tools.ts",
        BunPermissions: []string{"--allow-read", "--allow-net"},
    }

    factory := runtime.NewDefaultFactory(projectRoot)
    bunRuntime, err := factory.CreateRuntime(ctx, bunConfig)
    if err != nil {
        panic(err)
    }

    // Create Node.js runtime (when available)
    nodeConfig := &runtime.Config{
        RuntimeType:    "node",
        EntrypointPath: "./tools.js",
        NodeOptions: []string{
            "--max-old-space-size=2048",
            "--experimental-modules",
        },
    }

    nodeRuntime, err := factory.CreateRuntime(ctx, nodeConfig)
    if err != nil {
        if err.Error() == "node.js runtime not yet implemented" {
            fmt.Println("Node.js runtime not available yet")
        } else {
            panic(err)
        }
    }

    // Execute with Bun runtime
    result, err := bunRuntime.ExecuteTool(ctx, "fastTool", core.NewID(), input, env)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Bun execution: %+v\n", result)

    // Execute with Node.js runtime (when available)
    if nodeRuntime != nil {
        result, err := nodeRuntime.ExecuteTool(ctx, "compatTool", core.NewID(), input, env)
        if err != nil {
            panic(err)
        }

        fmt.Printf("Node.js execution: %+v\n", result)
    }
}
```

### Error Handling

```go
func handleExecutionErrors() {
    ctx := context.Background()

    // Create runtime
    factory := runtime.NewDefaultFactory("/path/to/project")
    config := runtime.DefaultConfig()
    rt, err := factory.CreateRuntime(ctx, config)
    if err != nil {
        panic(err)
    }

    // Execute tool with error handling
    result, err := rt.ExecuteTool(
        ctx,
        "errorProneTask",
        core.NewID(),
        input,
        env,
    )
    if err != nil {
        switch e := err.(type) {
        case *runtime.ProcessError:
            fmt.Printf("Process error: %s\n", e.Message)
            fmt.Printf("Exit code: %d\n", e.ExitCode)
            fmt.Printf("Stderr: %s\n", e.Stderr)

        case *runtime.ToolExecutionError:
            fmt.Printf("Tool execution error: %s\n", e.Error())
            fmt.Printf("Tool ID: %s\n", e.ToolID)

        default:
            fmt.Printf("Unknown error: %v\n", err)
        }
        return
    }

    fmt.Printf("Tool executed successfully: %+v\n", result)
}
```

### Runtime Availability Check

```go
func checkRuntimeAvailability() {
    // Check if Bun is available
    if runtime.IsBunAvailable() {
        fmt.Println("Bun runtime is available")
    } else {
        fmt.Println("Bun runtime is not available")
    }

    // Validate runtime type
    if runtime.IsValidRuntimeType("bun") {
        fmt.Println("Bun is a valid runtime type")
    }

    if runtime.IsValidRuntimeType("node") {
        fmt.Println("Node.js is a valid runtime type")
    }

    if !runtime.IsValidRuntimeType("python") {
        fmt.Println("Python is not a valid runtime type")
    }
}
```

---

## 📚 API Reference

### Factory Interface

```go
type Factory interface {
    CreateRuntime(ctx context.Context, config *Config) (Runtime, error)
}

// NewDefaultFactory creates a default factory
func NewDefaultFactory(projectRoot string) Factory
```

### Runtime Interface

```go
type Runtime interface {
    ExecuteTool(
        ctx context.Context,
        toolID string,
        toolExecID core.ID,
        input *core.Input,
        env core.EnvMap,
    ) (*core.Output, error)

    ExecuteToolWithTimeout(
        ctx context.Context,
        toolID string,
        toolExecID core.ID,
        input *core.Input,
        env core.EnvMap,
        timeout time.Duration,
    ) (*core.Output, error)

    GetGlobalTimeout() time.Duration
}
```

### Configuration

```go
type Config struct {
    RuntimeType              string        `json:"runtime_type"`
    EntrypointPath           string        `json:"entrypoint_path"`
    ToolExecutionTimeout     time.Duration `json:"tool_execution_timeout"`
    WorkerFilePerm           os.FileMode   `json:"worker_file_perm"`
    BunPermissions           []string      `json:"bun_permissions"`
    NodeOptions              []string      `json:"node_options"`
    BackoffInitialInterval   time.Duration `json:"backoff_initial_interval"`
    BackoffMaxInterval       time.Duration `json:"backoff_max_interval"`
    BackoffMaxElapsedTime    time.Duration `json:"backoff_max_elapsed_time"`
}

// Configuration functions
func DefaultConfig() *Config
func TestConfig() *Config
```

### Configuration Options

```go
// Configuration struct for direct field assignment
type Config struct {
    BackoffInitialInterval time.Duration
    BackoffMaxInterval     time.Duration
    BackoffMaxElapsedTime  time.Duration
    WorkerFilePerm         os.FileMode
    ToolExecutionTimeout   time.Duration
    RuntimeType            string   // "bun" or "node"
    EntrypointPath         string   // Path to entrypoint file
    BunPermissions         []string // Bun-specific permissions
    NodeOptions           []string // Node.js-specific options
    Environment           string   // Deployment environment
}

// Configuration factory functions
func DefaultConfig() *Config
func TestConfig() *Config
func FromAppConfig(appConfig *appconfig.RuntimeConfig) *Config
```

### BunManager

```go
// NewBunManager creates a new Bun runtime manager
func NewBunManager(
    ctx context.Context,
    projectRoot string,
    config *Config,
) (*BunManager, error)

// BunManager implements the Runtime interface
type BunManager struct {
    // Internal implementation
}
```

### Error Types

```go
type ProcessError struct {
    Message  string `json:"message"`
    ExitCode int    `json:"exit_code"`
    Stderr   string `json:"stderr"`
}

type ToolExecutionError struct {
    ToolID string
    Err    error
}

type ToolExecuteResult struct {
    Output *core.Output
    Error  *ToolExecutionError
}
```

### Utility Functions

```go
// Runtime availability and validation
func IsBunAvailable() bool
func IsValidRuntimeType(runtimeType string) bool

// Supported runtime types
const (
    RuntimeTypeBun  = "bun"
    RuntimeTypeNode = "node"
)

var SupportedRuntimeTypes = []string{
    RuntimeTypeBun,
    RuntimeTypeNode,
}
```

---

## 🧪 Testing

Run the test suite:

```bash
# Run all runtime tests
go test ./engine/runtime/...

# Run specific test
go test -v ./engine/runtime -run TestBunManager_ExecuteTool

# Run tests with coverage
go test -cover ./engine/runtime/...

# Run integration tests
go test -tags=integration ./engine/runtime/...
```

### Test Examples

```go
func TestRuntimeExecution(t *testing.T) {
    t.Run("Should execute tool successfully", func(t *testing.T) {
        ctx := context.Background()

        // Create test runtime
        factory := runtime.NewDefaultFactory("./testdata")
        config := runtime.TestConfig()
        rt, err := factory.CreateRuntime(ctx, config)
        require.NoError(t, err)

        // Execute tool
        input := &core.Input{
            Raw: map[string]any{"message": "test"},
        }

        result, err := rt.ExecuteTool(
            ctx,
            "testTool",
            core.NewID(),
            input,
            core.EnvMap{},
        )

        require.NoError(t, err)
        assert.NotNil(t, result)
    })

    t.Run("Should handle timeout", func(t *testing.T) {
        ctx := context.Background()

        // Create runtime with short timeout
        factory := runtime.NewDefaultFactory("./testdata")
        config := runtime.TestConfig()
        config.ToolExecutionTimeout = 100 * time.Millisecond

        rt, err := factory.CreateRuntime(ctx, config)
        require.NoError(t, err)

        // Execute slow tool
        result, err := rt.ExecuteTool(
            ctx,
            "slowTool",
            core.NewID(),
            input,
            core.EnvMap{},
        )

        assert.Error(t, err)
        assert.Nil(t, result)
        assert.Contains(t, err.Error(), "timeout")
    })
}
```

---

## 📦 Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md)

---

## 📄 License

BSL-1.1 License - see [LICENSE](../../LICENSE)
