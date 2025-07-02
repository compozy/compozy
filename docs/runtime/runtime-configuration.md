# Runtime Configuration

Compozy's new runtime system provides a flexible and configurable architecture for executing JavaScript tools. This guide covers all available configuration options for the runtime system.

## Overview

The runtime system uses a factory pattern with configurable options to manage JavaScript execution environments. Currently, Bun is the primary supported runtime, with Node.js support planned for future releases.

## Configuration Structure

The runtime configuration is defined in the `runtime.Config` struct:

```go
type Config struct {
    // Retry and backoff configuration
    BackoffInitialInterval time.Duration
    BackoffMaxInterval     time.Duration
    BackoffMaxElapsedTime  time.Duration

    // Security and permissions
    WorkerFilePerm         os.FileMode

    // Execution settings
    ToolExecutionTimeout   time.Duration

    // Runtime selection
    RuntimeType    string   // "bun" or "node"
    EntrypointPath string   // Path to entrypoint file

    // Runtime-specific options
    BunPermissions []string // Bun-specific permissions
    NodeOptions    []string // Node.js-specific options
}
```

## Configuration Options

### Retry and Backoff Settings

These settings control how the runtime handles execution retries and backoff strategies:

- **`BackoffInitialInterval`** (default: `100ms`)

    - Initial delay before the first retry attempt
    - Should be kept relatively short for responsive user experience

- **`BackoffMaxInterval`** (default: `5s`)

    - Maximum delay between retry attempts
    - Prevents excessive wait times during prolonged failures

- **`BackoffMaxElapsedTime`** (default: `30s`)
    - Total time limit for all retry attempts
    - After this time, the operation will fail permanently

### Security Settings

- **`WorkerFilePerm`** (default: `0600`)
    - File permissions for temporary worker files
    - Uses secure permissions by default (owner read/write only)

### Execution Settings

- **`ToolExecutionTimeout`** (default: `60s`)
    - Global timeout for tool execution
    - Can be overridden per-execution using `ExecuteToolWithTimeout`

### Runtime Selection

- **`RuntimeType`** (default: `"bun"`)

    - Specifies which JavaScript runtime to use
    - Supported values: `"bun"`, `"node"` (planned)

- **`EntrypointPath`**
    - Custom path to the entrypoint file
    - If not specified, uses default entrypoint generation

### Runtime-Specific Options

#### Bun Permissions

- **`BunPermissions`** (default: `["--allow-read"]`)
    - Array of Bun command-line flags for permissions
    - Common options:
        - `--allow-read` - Allow file system read access
        - `--allow-write` - Allow file system write access
        - `--allow-net` - Allow network access
        - `--allow-run` - Allow subprocess execution

#### Node.js Options (Future)

- **`NodeOptions`**
    - Array of Node.js command-line options
    - Will be used when Node.js runtime support is added

## Configuration Methods

### Using Default Configuration

```go
config := runtime.DefaultConfig()
manager, err := runtime.NewManager(ctx, workingDir, runtime.WithConfig(config))
```

### Using Individual Options

```go
manager, err := runtime.NewManager(ctx, workingDir,
    runtime.WithToolExecutionTimeout(30*time.Second),
    runtime.WithRuntimeType(runtime.RuntimeTypeBun),
    runtime.WithBunPermissions([]string{"--allow-read", "--allow-net"}),
)
```

### Using Builder Pattern

```go
config := &runtime.Config{
    ToolExecutionTimeout: 45 * time.Second,
    RuntimeType:         runtime.RuntimeTypeBun,
    BunPermissions:      []string{"--allow-read", "--allow-write"},
}

manager, err := runtime.NewManager(ctx, workingDir, runtime.WithConfig(config))
```

## Environment-Specific Configurations

### Development Configuration

```go
// More permissive settings for development
devConfig := &runtime.Config{
    ToolExecutionTimeout: 120 * time.Second, // Longer timeout for debugging
    BunPermissions: []string{
        "--allow-read",
        "--allow-write",
        "--allow-net",
    },
}
```

### Production Configuration

```go
// Secure settings for production
prodConfig := &runtime.Config{
    ToolExecutionTimeout: 30 * time.Second, // Shorter timeout
    BunPermissions: []string{
        "--allow-read", // Minimal permissions
    },
    BackoffMaxElapsedTime: 15 * time.Second, // Faster failure
}
```

### Test Configuration

```go
// Fast settings for testing
testConfig := runtime.TestConfig() // Built-in test configuration
// Or manually:
testConfig := &runtime.Config{
    BackoffInitialInterval: 10 * time.Millisecond,
    BackoffMaxInterval:     100 * time.Millisecond,
    BackoffMaxElapsedTime:  1 * time.Second,
    ToolExecutionTimeout:   5 * time.Second,
}
```

## Security Considerations

### File Permissions

The runtime system uses secure file permissions by default:

- Worker files: `0600` (owner read/write only)
- Generated entrypoints: Follow system default with secure handling

### Bun Permissions

Always use the minimal required permissions:

- Start with `--allow-read` only
- Add `--allow-write` only if tools need file modification
- Add `--allow-net` only if tools need network access
- Avoid `--allow-run` unless subprocess execution is required

### Environment Variable Validation

The runtime automatically validates environment variables:

- Filters dangerous variables (e.g., `LD_PRELOAD`, `NODE_OPTIONS`)
- Validates variable name format (uppercase, alphanumeric, underscores)
- Prevents injection attacks through variable values

## Best Practices

### Timeout Configuration

- **Development**: Use longer timeouts (60-120s) for debugging
- **Production**: Use shorter timeouts (15-30s) for responsiveness
- **Testing**: Use very short timeouts (1-5s) for fast test execution

### Permission Management

```go
// Good: Minimal permissions
BunPermissions: []string{"--allow-read"}

// Acceptable: Specific permissions as needed
BunPermissions: []string{"--allow-read", "--allow-net"}

// Avoid: Overly permissive
BunPermissions: []string{"--allow-all"} // Not recommended
```

### Error Handling

Always check for configuration errors:

```go
manager, err := runtime.NewManager(ctx, workingDir, options...)
if err != nil {
    return fmt.Errorf("failed to create runtime manager: %w", err)
}
```

## Migration from Legacy Configuration

If migrating from the old Deno-based system:

1. Remove all `deno.json` files
2. Update tool configurations to remove `execute` properties
3. Use the new entrypoint pattern (see [Entrypoint Pattern Guide](runtime-entrypoint.md))
4. Configure Bun permissions equivalent to your Deno permissions

## Troubleshooting

### Common Configuration Issues

- **Permission Denied**: Check `BunPermissions` includes required access
- **Timeout Errors**: Increase `ToolExecutionTimeout` or optimize tool code
- **File Access Issues**: Verify `WorkerFilePerm` and file system permissions

### Debug Configuration

```go
// Enable debug logging and extended timeouts
debugConfig := &runtime.Config{
    ToolExecutionTimeout:   300 * time.Second, // 5 minutes
    BackoffMaxElapsedTime:  60 * time.Second,  // 1 minute
    BunPermissions: []string{
        "--allow-read",
        "--allow-write",
        "--allow-net",
    },
}
```

For more troubleshooting information, see [Runtime Troubleshooting Guide](runtime-troubleshooting.md).
