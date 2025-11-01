# Package tool

SDK package for building tool configurations using functional options pattern.

## Overview

The `tool` package provides a fluent API for creating tool configurations that extend AI agent capabilities with external systems, APIs, and custom business logic.

## Installation

```go
import "github.com/compozy/compozy/sdk/tool"
```

## Usage

### Basic Example

```go
cfg, err := tool.New(
    ctx,
    "file-reader",
    tool.WithName("File Reader"),
    tool.WithDescription("Read and parse various file formats"),
    tool.WithRuntime("bun"),
    tool.WithCode(`
        export default async function(input) {
            const fs = require('fs');
            return fs.readFileSync(input.path, 'utf8');
        }
    `),
)
if err != nil {
    log.Fatal(err)
}
```

### Full Configuration

```go
inputSchema := &schema.Schema{
    "type": "object",
    "properties": map[string]any{
        "path": map[string]any{
            "type": "string",
            "description": "File path to read",
        },
        "format": map[string]any{
            "type": "string",
            "enum": []string{"json", "yaml", "csv"},
        },
    },
    "required": []string{"path"},
}

outputSchema := &schema.Schema{
    "type": "object",
    "properties": map[string]any{
        "content": map[string]any{
            "type": "string",
        },
    },
}

cfg, err := tool.New(
    ctx,
    "api-client",
    tool.WithName("API Client"),
    tool.WithDescription("HTTP API client with retry logic"),
    tool.WithRuntime("bun"),
    tool.WithCode(toolCode),
    tool.WithTimeout("30s"),
    tool.WithInputSchema(inputSchema),
    tool.WithOutputSchema(outputSchema),
    tool.WithWith(&core.Input{
        "base_url": "https://api.example.com",
    }),
    tool.WithConfig(&core.Input{
        "retry_count": 3,
        "timeout": 10,
    }),
    tool.WithEnv(&core.EnvMap{
        "API_KEY": "{{ .env.SECRET_API_KEY }}",
    }),
)
```

## API Reference

### Constructor

```go
func New(ctx context.Context, id string, opts ...Option) (*engine.Config, error)
```

Creates a new tool configuration with the given ID and options.

**Parameters:**

- `ctx`: Context for logging and cancellation
- `id`: Unique tool identifier (kebab-case recommended)
- `opts`: Variadic functional options

**Returns:**

- `*engine.Config`: Deep-copied tool configuration
- `error`: Validation errors if any

### Options

#### WithName(name string) Option

Sets the human-readable name for the tool.

#### WithDescription(description string) Option

Sets the detailed description of tool capabilities.

#### WithRuntime(runtime string) Option

Sets the execution runtime (currently supports "bun").

#### WithCode(code string) Option

Sets the inline source code executed by the runtime.

#### WithTimeout(timeout string) Option

Sets the maximum execution time (e.g., "30s", "5m", "1h").

#### WithInputSchema(schema \*schema.Schema) Option

Sets the JSON schema for input validation.

#### WithOutputSchema(schema \*schema.Schema) Option

Sets the JSON schema for output validation.

#### WithWith(input \*core.Input) Option

Sets default input parameters merged with runtime parameters.

#### WithConfig(config \*core.Input) Option

Sets configuration parameters for tool initialization.

#### WithEnv(env \*core.EnvMap) Option

Sets environment variables for tool execution.

## Migration Guide

### Before (Old SDK)

```go
cfg, err := tool.New("file-reader").
    WithName("File Reader").
    WithDescription("Read files").
    WithRuntime("bun").
    WithCode(code).
    Build(ctx)
```

### After (New SDK)

```go
cfg, err := tool.New(
    ctx,
    "file-reader",
    tool.WithName("File Reader"),
    tool.WithDescription("Read files"),
    tool.WithRuntime("bun"),
    tool.WithCode(code),
)
```

### Key Changes

1. ✅ `ctx` moved to first parameter
2. ✅ No `.Build(ctx)` call needed
3. ✅ Options passed as variadic arguments
4. ✅ Validation happens immediately in constructor
5. ✅ All string fields automatically trimmed

## Examples

### Tool with Schema Validation

```go
schema := &schema.Schema{
    "type": "object",
    "properties": map[string]any{
        "query": map[string]any{
            "type": "string",
            "minLength": 1,
        },
    },
    "required": []string{"query"},
}

cfg, err := tool.New(
    ctx,
    "search-tool",
    tool.WithName("Search"),
    tool.WithDescription("Search documents"),
    tool.WithRuntime("bun"),
    tool.WithCode(searchCode),
    tool.WithInputSchema(schema),
)
```

### Tool with Timeout

```go
cfg, err := tool.New(
    ctx,
    "heavy-processor",
    tool.WithName("Data Processor"),
    tool.WithDescription("Process large datasets"),
    tool.WithRuntime("bun"),
    tool.WithCode(processorCode),
    tool.WithTimeout("5m"),
)
```

## Validation Rules

- **ID**: Required, non-empty, must follow identifier format
- **Name**: Required, non-empty after trimming
- **Description**: Required, non-empty after trimming
- **Runtime**: Required, must be "bun" (case-insensitive)
- **Code**: Required, non-empty after trimming
- **Timeout**: Optional, must be valid Go duration format if provided
- **Timeout**: Must be positive if specified

## Error Handling

The constructor collects all validation errors and returns them together:

```go
cfg, err := tool.New(ctx, "", tool.WithName(""))
if err != nil {
    // err contains all validation failures:
    // - "tool id is invalid"
    // - "tool name cannot be empty"
    fmt.Println(err.Error())
}
```

## Testing

Run tests with:

```bash
gotestsum --format pkgname -- -race -parallel=4 ./sdk/tool
```

Run linting:

```bash
golangci-lint run --fix --allow-parallel-runners ./sdk/tool/...
```
