# Runtime Package

## Overview

The runtime package provides a clean, functional options API for configuring JavaScript/TypeScript runtime environments (Bun or Node.js) for workflow execution in Compozy.

## Installation

```go
import runtime "github.com/compozy/compozy/sdk/v2/runtime"
```

## Usage

### Basic Example - Bun Runtime

```go
cfg, err := runtime.New(ctx, "bun",
    runtime.WithEntrypointPath("./tools/main.ts"),
)
if err != nil {
    log.Fatal(err)
}
```

### Basic Example - Node.js Runtime

```go
cfg, err := runtime.New(ctx, "node",
    runtime.WithEntrypointPath("./tools/main.js"),
    runtime.WithNodeOptions([]string{"--max-old-space-size=4096"}),
)
if err != nil {
    log.Fatal(err)
}
```

### Full Configuration

```go
nativeTools := &engineruntime.NativeToolsConfig{
    CallAgents:    true,
    CallWorkflows: true,
}

cfg, err := runtime.New(ctx, "bun",
    // Required: entrypoint script path
    runtime.WithEntrypointPath("./tools/main.ts"),

    // Bun-specific permissions
    runtime.WithBunPermissions([]string{
        "--allow-read",
        "--allow-net=api.example.com",
        "--allow-env=API_KEY,API_SECRET",
    }),

    // Memory and performance
    runtime.WithMaxMemoryMB(1024),
    runtime.WithToolExecutionTimeout(30 * time.Second),

    // Native tools integration
    runtime.WithNativeTools(nativeTools),

    // Environment
    runtime.WithEnvironment("production"),
)
if err != nil {
    log.Fatal(err)
}
```

## API Reference

### Constructor

```go
func New(ctx context.Context, runtimeType string, opts ...Option) (*engineruntime.Config, error)
```

Creates a new runtime configuration with the specified runtime type and optional configuration.

**Parameters:**

- `ctx`: Context for logging and cancellation
- `runtimeType`: Runtime type ("bun" or "node")
- `opts`: Variadic functional options

**Returns:**

- `*engineruntime.Config`: Deep-copied runtime configuration
- `error`: Validation errors if any

**Supported Runtime Types:**

- `"bun"` - Bun JavaScript runtime (default)
- `"node"` - Node.js runtime

### Options

#### Core Configuration

**`WithEntrypointPath(path string)`**
Sets the path to the runtime entrypoint script.

```go
runtime.WithEntrypointPath("./tools/main.ts")
```

**`WithEnvironment(env string)`**
Sets the deployment environment (development, staging, production).

```go
runtime.WithEnvironment("production")
```

#### Bun-Specific Options

**`WithBunPermissions(permissions []string)`**
Configures Bun permission flags for security.

```go
runtime.WithBunPermissions([]string{
    "--allow-read",           // Allow all read operations
    "--allow-write",          // Allow all write operations
    "--allow-net",            // Allow all network access
    "--allow-env",            // Allow all environment variables
    "--allow-net=example.com", // Scoped network access
    "--allow-env=API_KEY",    // Scoped environment access
    "--allow-all",            // Allow all operations (use with caution)
})
```

#### Node.js-Specific Options

**`WithNodeOptions(options []string)`**
Configures Node.js command-line options.

```go
runtime.WithNodeOptions([]string{
    "--max-old-space-size=4096",
    "--experimental-modules",
})
```

#### Performance and Limits

**`WithMaxMemoryMB(mb int)`**
Sets the maximum memory allocation in megabytes. Default: 2048 MB (2 GB).

```go
runtime.WithMaxMemoryMB(1024) // 1 GB limit
```

**`WithToolExecutionTimeout(timeout time.Duration)`**
Sets the maximum duration for tool execution. Default: 60 seconds.

```go
runtime.WithToolExecutionTimeout(30 * time.Second)
```

**`WithMaxStderrCaptureSize(size int)`**
Sets the maximum stderr buffer size in bytes. Default: 1 MB.

```go
runtime.WithMaxStderrCaptureSize(2 * 1024 * 1024) // 2 MB
```

#### Backoff Configuration

**`WithBackoffInitialInterval(duration time.Duration)`**
Sets the initial backoff interval for retries.

```go
runtime.WithBackoffInitialInterval(100 * time.Millisecond)
```

**`WithBackoffMaxInterval(duration time.Duration)`**
Sets the maximum backoff interval.

```go
runtime.WithBackoffMaxInterval(5 * time.Second)
```

**`WithBackoffMaxElapsedTime(duration time.Duration)`**
Sets the maximum total elapsed time for backoff retries.

```go
runtime.WithBackoffMaxElapsedTime(30 * time.Second)
```

#### Advanced Options

**`WithNativeTools(tools *engineruntime.NativeToolsConfig)`**
Enables builtin native tools provided by the engine.

```go
nativeTools := &engineruntime.NativeToolsConfig{
    CallAgents:    true,
    CallWorkflows: true,
}
runtime.WithNativeTools(nativeTools)
```

**`WithWorkerFilePerm(perm os.FileMode)`**
Sets file permissions for worker files. Default: 0600.

```go
runtime.WithWorkerFilePerm(0600) // Owner read/write only
```

**`WithRuntimeType(runtimeType string)`**
Overrides the runtime type (generally set via constructor).

```go
runtime.WithRuntimeType("node")
```

## Migration Guide

### Before (Old SDK - Builder Pattern)

```go
cfg, err := runtime.NewBun().
    WithEntrypoint("./tools/main.ts").
    WithBunPermissions("--allow-read", "--allow-net").
    WithToolTimeout(30 * time.Second).
    WithMaxMemoryMB(512).
    Build(ctx)
```

### After (New SDK - Functional Options)

```go
cfg, err := runtime.New(ctx, "bun",
    runtime.WithEntrypointPath("./tools/main.ts"),
    runtime.WithBunPermissions([]string{"--allow-read", "--allow-net"}),
    runtime.WithToolExecutionTimeout(30 * time.Second),
    runtime.WithMaxMemoryMB(512),
)
```

### Key Changes

1. **Context First**: `ctx` is now the first parameter instead of passed to `Build()`
2. **Runtime Type**: Specified in constructor instead of separate `NewBun()` function
3. **No Build() Call**: Configuration is validated and created immediately
4. **Slice Arguments**: Collections like permissions take slices instead of variadic args
5. **Renamed Methods**: Some methods renamed for clarity (e.g., `WithToolTimeout` → `WithToolExecutionTimeout`)

## Validation Rules

The constructor validates:

1. **Runtime Type**: Must be "bun" or "node" (case-insensitive)
2. **Context**: Cannot be nil
3. **Entrypoint Path**: Trimmed of whitespace (empty is allowed)

All other fields use engine defaults and are validated at runtime.

## Error Handling

The constructor returns a `*sdkerrors.BuildError` containing all validation errors:

```go
cfg, err := runtime.New(ctx, "invalid",
    runtime.WithEntrypointPath(""),
)
if err != nil {
    var buildErr *sdkerrors.BuildError
    if errors.As(err, &buildErr) {
        for _, e := range buildErr.Errors {
            log.Printf("Validation error: %v", e)
        }
    }
}
```

## Examples

### Development Environment

```go
cfg, err := runtime.New(ctx, "bun",
    runtime.WithEntrypointPath("./dev/tools.ts"),
    runtime.WithEnvironment("development"),
    runtime.WithMaxMemoryMB(512),
)
```

### Production Environment with Security

```go
cfg, err := runtime.New(ctx, "bun",
    runtime.WithEntrypointPath("./dist/tools.js"),
    runtime.WithBunPermissions([]string{
        "--allow-net=api.production.com",
        "--allow-env=API_KEY,DB_URL",
        "--allow-read",
    }),
    runtime.WithEnvironment("production"),
    runtime.WithMaxMemoryMB(2048),
    runtime.WithToolExecutionTimeout(60 * time.Second),
)
```

### Node.js Runtime

```go
cfg, err := runtime.New(ctx, "node",
    runtime.WithEntrypointPath("./tools/index.js"),
    runtime.WithNodeOptions([]string{
        "--max-old-space-size=4096",
        "--enable-source-maps",
    }),
    runtime.WithMaxMemoryMB(4096),
)
```

## Testing

Use the test config for faster tests:

```go
func TestMyFeature(t *testing.T) {
    cfg, err := runtime.New(t.Context(), "bun",
        runtime.WithEntrypointPath("./test/tools.ts"),
        runtime.WithMaxMemoryMB(256),
        runtime.WithToolExecutionTimeout(5 * time.Second),
    )
    require.NoError(t, err)
    // ... test with cfg
}
```

## Best Practices

1. **Security**: Always use minimal Bun permissions required for your use case
2. **Memory**: Set appropriate memory limits based on your tool requirements
3. **Timeouts**: Configure realistic timeouts to prevent hanging operations
4. **Environment**: Use different configurations for dev/staging/production
5. **Deep Copy**: The returned config is deep-copied, preventing accidental mutations

## Performance

- Constructor execution: < 1µs for minimal config
- Memory allocation: < 5KB per constructor call
- No performance regression vs. builder pattern
