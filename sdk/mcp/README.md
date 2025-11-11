# MCP Package

Model Context Protocol (MCP) configuration using functional options pattern.

## Installation

```go
import "github.com/compozy/compozy/sdk/mcp"
```

## Usage

### Basic Stdio MCP

```go
cfg, err := mcp.New(ctx, "filesystem",
    mcp.WithCommand("mcp-server-filesystem"),
)
```

### Basic HTTP MCP

```go
cfg, err := mcp.New(ctx, "github",
    mcp.WithURL("https://api.github.com/mcp"),
)
```

### Full Stdio Configuration

```go
cfg, err := mcp.New(ctx, "filesystem",
    mcp.WithCommand("mcp-server-filesystem"),
    mcp.WithArgs([]string{"--root", "/data"}),
    mcp.WithEnv(map[string]string{
        "LOG_LEVEL": "debug",
        "ROOT_DIR":  "/workspace",
    }),
    mcp.WithStartTimeout(30*time.Second),
    mcp.WithMaxSessions(5),
    mcp.WithProto("2025-03-26"),
)
```

### Full HTTP Configuration

```go
cfg, err := mcp.New(ctx, "github",
    mcp.WithURL("https://api.github.com/mcp"),
    mcp.WithHeaders(map[string]string{
        "Authorization": "Bearer token123",
    }),
    mcp.WithTransport(mcpproxy.TransportStreamableHTTP),
    mcp.WithMaxSessions(10),
)
```

## API Reference

### Constructor

```go
func New(ctx context.Context, id string, opts ...Option) (*enginemcp.Config, error)
```

Creates a new MCP configuration with the given ID and options.

**Parameters:**

- `ctx` - Context for validation and logging
- `id` - Unique identifier for the MCP server (required, non-empty)
- `opts` - Variadic functional options

**Returns:**

- `*enginemcp.Config` - Deep copied configuration
- `error` - Validation errors

### Options

#### WithResource

```go
func WithResource(resource string) Option
```

Sets the resource identifier (defaults to ID).

#### WithCommand

```go
func WithCommand(command string) Option
```

Sets the command for stdio transport.

#### WithArgs

```go
func WithArgs(args []string) Option
```

Sets command arguments for stdio transport.

#### WithURL

```go
func WithURL(url string) Option
```

Sets the URL for HTTP-based transports.

#### WithHeaders

```go
func WithHeaders(headers map[string]string) Option
```

Sets HTTP headers for URL-based MCPs.

#### WithEnv

```go
func WithEnv(env map[string]string) Option
```

Sets environment variables for command-based MCPs.

#### WithTransport

```go
func WithTransport(transport mcpproxy.TransportType) Option
```

Sets the transport type (stdio, sse, streamable-http).

#### WithProto

```go
func WithProto(proto string) Option
```

Sets the MCP protocol version (format: YYYY-MM-DD).

#### WithStartTimeout

```go
func WithStartTimeout(startTimeout time.Duration) Option
```

Sets startup timeout for command-based MCPs.

#### WithMaxSessions

```go
func WithMaxSessions(maxSessions int) Option
```

Sets maximum concurrent sessions (0 = unlimited).

## Migration Guide

### Before (Old SDK)

```go
cfg, err := mcp.New("filesystem").
    WithCommand("mcp-server-filesystem", "--root", "/data").
    WithEnvVar("LOG_LEVEL", "debug").
    WithStartTimeout(30 * time.Second).
    Build(ctx)
```

### After (New SDK)

```go
cfg, err := mcp.New(ctx, "filesystem",
    mcp.WithCommand("mcp-server-filesystem"),
    mcp.WithArgs([]string{"--root", "/data"}),
    mcp.WithEnv(map[string]string{
        "LOG_LEVEL": "debug",
    }),
    mcp.WithStartTimeout(30*time.Second),
)
```

### Key Changes

1. **Context First**: `ctx` moved to first parameter
2. **No Build()**: Configuration validated immediately
3. **Separate Args**: Command and args are separate options
4. **Map-Based**: Env and Headers use maps instead of individual methods
5. **Variadic Options**: All options passed as variadic arguments

## Validation

The constructor validates:

- ID is non-empty and valid format
- Either command OR url is configured (not both)
- Transport type is valid (stdio, sse, streamable-http)
- Command is required for stdio transport
- URL is required for HTTP transports

## Examples

See `constructor_test.go` for comprehensive usage examples.
