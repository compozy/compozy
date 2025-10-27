# Compozy SDK v2 Examples

This directory contains examples demonstrating the SDK v2 functional options API.

## Overview

SDK v2 uses functional options pattern with these key changes from SDK v1:

- **Context-first**: `ctx` is always the first parameter
- **No Build() call**: Validation happens in constructor
- **Functional options**: Use `WithX()` functions instead of builder methods
- **Type-safe**: Full Go type checking at compile time

## Running Examples

Each example demonstrates a specific SDK v2 pattern:

### Simple Workflow

```bash
go run sdk2/examples --example simple-workflow
```

Demonstrates:

- Basic agent creation with `agent.New()`
- Workflow configuration with `workflow.New()`
- Project setup with `project.New()`

### Parallel Tasks

```bash
go run sdk2/examples --example parallel-tasks
```

Demonstrates:

- Multiple agents working in parallel
- Parallel task configuration
- Result aggregation

### Knowledge RAG

```bash
go run sdk2/examples --example knowledge-rag
```

Demonstrates:

- Knowledge base configuration
- Embedder setup
- Vector DB integration
- Agent with knowledge binding

### Memory Conversation

```bash
go run sdk2/examples --example memory-conversation
```

Demonstrates:

- Memory configuration
- Conversation persistence
- Agent with memory

### Runtime Native Tools

```bash
go run sdk2/examples --example runtime-native-tools
```

Demonstrates:

- Runtime configuration
- Native tool definitions
- Tool execution

### Scheduled Workflow

```bash
go run sdk2/examples --example scheduled-workflow
```

Demonstrates:

- Schedule configuration
- Cron-based execution
- Workflow scheduling

### Signal Communication

```bash
go run sdk2/examples --example signal-communication
```

Demonstrates:

- Signal tasks
- Workflow communication patterns
- Wait and signal coordination

### Complete Project

```bash
go run sdk2/examples --example complete-project
```

Demonstrates:

- Full project with all components
- Multiple workflows
- Complex integrations

### Debugging

```bash
go run sdk2/examples --example debugging
```

Demonstrates:

- Debugging configuration
- Retry logic
- Timeout handling
- Error recovery patterns

## API Migration Guide

### Before (SDK v1 - Builder Pattern)

```go
cfg, err := agent.New("assistant").
    WithInstructions("You are helpful").
    WithModel("openai", "gpt-4").
    Build(ctx)
```

### After (SDK v2 - Functional Options)

```go
cfg, err := agent.New(ctx, "assistant",
    agent.WithInstructions("You are helpful"),
    agent.WithModel(engineagent.Model{
        Config: core.ProviderConfig{
            Provider: core.ProviderOpenAI,
            Model:    "gpt-4",
        },
    }),
)
```

### Key Differences

1. **Context First**: `ctx` moved to first parameter
2. **No Build()**: Validation happens in constructor, not at Build()
3. **Type Safety**: `WithModel()` takes struct, not separate strings
4. **Collections**: Use plural names with slices:
   - `WithActions([]*ActionConfig{...})` not `AddAction(...)`
   - `WithTools([]ToolConfig{...})` not `AddTool(...)`
5. **Variadic Options**: All options passed as variadic arguments

## Development

### Adding New Examples

1. Add function to `main.go`:

```go
func RunMyExample(ctx context.Context) error {
    // Implementation
}
```

2. Register in `runExample()` function:

```go
examples := map[string]func(context.Context) error{
    "my-example": RunMyExample,
    // ...
}
```

3. Add to help text in `showHelp()` function

### Testing Examples

```bash
# Run specific example
go run sdk2/examples --example simple-workflow

# Show available examples
go run sdk2/examples --help

# Build for testing
go build -o /tmp/sdk2-examples sdk2/examples/main.go
/tmp/sdk2-examples --example simple-workflow
```

## Environment Variables

Most examples require API keys:

```bash
export OPENAI_API_KEY="your-key-here"
export ANTHROPIC_API_KEY="your-key-here" # If using Claude models
```

## See Also

- [SDK v2 Migration Guide](../MIGRATION_GUIDE.md)
- [Code Generation Documentation](../internal/codegen/README.md)
- [Individual Package READMEs](../)
