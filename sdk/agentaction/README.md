# Agent Action SDK

The `agentaction` package provides a functional options pattern for creating agent action configurations in Compozy. Actions define structured, type-safe interfaces for specific agent capabilities.

## Features

- **Auto-generated functional options** from engine structs
- **Type-safe configuration** with compile-time validation
- **Centralized validation** in constructor
- **Deep copy safety** for configuration isolation
- **Zero boilerplate** - just run `go generate` when engine changes

## Installation

```go
import "github.com/compozy/compozy/sdk/v2/agentaction"
```

## Quick Start

### Basic Action

```go
action, err := agentaction.New(ctx, "review-code",
    agentaction.WithPrompt("Analyze code for quality and improvements"),
)
```

### Action with Tools

```go
action, err := agentaction.New(ctx, "analyze-code",
    agentaction.WithPrompt("Perform comprehensive code analysis"),
    agentaction.WithTools([]tool.Config{
        {ID: "file-reader"},
        {ID: "static-analyzer"},
    }),
    agentaction.WithTimeout("5m"),
)
```

### Action with Input/Output Schemas

```go
inputSchema := &schema.Schema{
    "type": "object",
    "properties": map[string]any{
        "code": map[string]any{"type": "string"},
        "language": map[string]any{"type": "string"},
    },
    "required": []string{"code", "language"},
}

outputSchema := &schema.Schema{
    "type": "object",
    "properties": map[string]any{
        "issues": map[string]any{
            "type": "array",
            "items": map[string]any{"type": "object"},
        },
    },
}

action, err := agentaction.New(ctx, "validate-code",
    agentaction.WithPrompt("Validate code against best practices"),
    agentaction.WithInputSchema(inputSchema),
    agentaction.WithOutputSchema(outputSchema),
)
```

### Action with Retry Policy

```go
action, err := agentaction.New(ctx, "api-call",
    agentaction.WithPrompt("Call external API"),
    agentaction.WithRetryPolicy(&core.RetryPolicyConfig{
        MaximumAttempts:    5,
        InitialInterval:    "1s",
        BackoffCoefficient: 2.0,
    }),
    agentaction.WithTimeout("30s"),
)
```

### Action with Transitions

```go
action, err := agentaction.New(ctx, "process-step",
    agentaction.WithPrompt("Process data step"),
    agentaction.WithOnSuccess(&core.SuccessTransition{
        Next: strPtr("next-step"),
    }),
    agentaction.WithOnError(&core.ErrorTransition{
        Next: strPtr("error-handler"),
    }),
)
```

## Available Options

All options are auto-generated from `engine/agent/action_config.go`:

### Core Configuration

- `WithID(id string)` - Action identifier
- `WithPrompt(prompt string)` - Action instructions (required)

### Schema Definition

- `WithInputSchema(schema *schema.Schema)` - Input validation schema
- `WithOutputSchema(schema *schema.Schema)` - Output validation schema

### Execution Control

- `WithTimeout(timeout string)` - Maximum execution duration (e.g., "30s", "5m")
- `WithRetryPolicy(policy *core.RetryPolicyConfig)` - Automatic retry behavior
- `WithTools(tools []tool.Config)` - Action-scoped tools

### Workflow Integration

- `WithOnSuccess(transition *core.SuccessTransition)` - Success handler
- `WithOnError(transition *core.ErrorTransition)` - Error handler
- `WithWith(input *core.Input)` - Default input parameters
- `WithAttachments(attachments attachment.Attachments)` - Action-level attachments

## Validation Rules

The constructor performs centralized validation:

1. **Context Required** - Must provide non-nil context
2. **ID Required** - Must be non-empty after trimming
3. **Prompt Required** - Must be non-empty after trimming
4. **Automatic Trimming** - ID and prompt are automatically trimmed

## Error Handling

The constructor returns `*sdkerrors.BuildError` containing all validation errors:

```go
action, err := agentaction.New(ctx, "")
if err != nil {
    var buildErr *sdkerrors.BuildError
    if errors.As(err, &buildErr) {
        for _, e := range buildErr.Errors {
            fmt.Printf("Validation error: %v\n", e)
        }
    }
}
```

## Code Generation

When `engine/agent/action_config.go` changes:

```bash
cd sdk/agentaction
go generate
```

This automatically regenerates `options_generated.go` with all options.

## Comparison with Old Builder Pattern

### Before (Builder Pattern)

```go
action, err := agent.NewAction("review-code").
    WithPrompt("Analyze code").
    AddTool("file-reader").
    WithTimeout(30*time.Second).
    Build(ctx)
```

### After (Functional Options)

```go
action, err := agentaction.New(ctx, "review-code",
    agentaction.WithPrompt("Analyze code"),
    agentaction.WithTools([]tool.Config{{ID: "file-reader"}}),
    agentaction.WithTimeout("30s"),
)
```

### Key Differences

1. **Context First** - `ctx` is now the first parameter
2. **No Build()** - Constructor validates immediately
3. **Collections as Slices** - `WithTools([]tool.Config)` instead of `AddTool()`
4. **Timeout as String** - `"30s"` instead of `30*time.Second`
5. **Type Safety** - All options are strongly typed

## Best Practices

1. **Always use context from test** - `ctx := t.Context()` in tests
2. **Validate inputs** - Constructor handles validation, don't skip it
3. **Use schemas for type safety** - Define input/output schemas for structured data
4. **Set timeouts** - Always specify reasonable timeout values
5. **Configure retries** - Use retry policy for unreliable operations

## Examples

See `constructor_test.go` for comprehensive usage examples covering:

- Minimal configuration
- Full configuration with all options
- Error handling and validation
- Schema definition patterns
- Retry and timeout configuration
- Transition workflows

## Architecture

- **Package:** `agentaction` (separate from `agent` to avoid option type conflicts)
- **Generated Code:** `options_generated.go` (auto-generated, do not edit)
- **Constructor:** `constructor.go` (manual validation logic)
- **Tests:** `constructor_test.go` (comprehensive test coverage)
- **Generator:** `../internal/codegen/` (shared code generation infrastructure)

## Contributing

When adding new fields to `engine/agent/action_config.go`:

1. Add the field with proper documentation
2. Run `go generate` in `sdk/agentaction/`
3. Update `constructor.go` if validation is needed
4. Add tests in `constructor_test.go`
5. Run `make lint && make test`
