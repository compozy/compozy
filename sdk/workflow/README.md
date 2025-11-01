# Package workflow

SDK for creating workflow configurations using auto-generated functional options.

## Installation

```go
import "github.com/compozy/compozy/sdk/v2/workflow"
```

## Usage

### Basic Example

```go
package main

import (
    "context"
    "github.com/compozy/compozy/engine/task"
    "github.com/compozy/compozy/sdk/v2/workflow"
)

func main() {
    cfg, err := workflow.New(context.Background(), "simple-workflow",
        workflow.WithDescription("A simple workflow"),
        workflow.WithTasks([]task.Config{
            {BaseConfig: task.BaseConfig{ID: "task1", Type: task.TaskTypeBasic}},
        }),
    )
    if err != nil {
        panic(err)
    }
    // Use cfg...
}
```

### Full Configuration

```go
package main

import (
    "context"
    "github.com/compozy/compozy/engine/agent"
    "github.com/compozy/compozy/engine/core"
    "github.com/compozy/compozy/engine/schema"
    "github.com/compozy/compozy/engine/task"
    "github.com/compozy/compozy/engine/tool"
    engineworkflow "github.com/compozy/compozy/engine/workflow"
    "github.com/compozy/compozy/sdk/v2/workflow"
)

func main() {
    author := &core.Author{
        Name:  "Your Name",
        Email: "your@email.com",
    }

    inputSchema := &schema.Schema{
        "type": "object",
        "properties": map[string]any{
            "message": map[string]any{
                "type": "string",
            },
        },
    }

    outputs := &core.Output{
        "result": "{{ .tasks.process.output }}",
    }

    schedule := &engineworkflow.Schedule{
        Cron: "0 * * * *", // Every hour
    }

    cfg, err := workflow.New(context.Background(), "advanced-workflow",
        workflow.WithVersion("1.0.0"),
        workflow.WithDescription("An advanced workflow with all features"),
        workflow.WithOpts(engineworkflow.Opts{InputSchema: inputSchema}),
        workflow.WithAuthor(author),
        workflow.WithTools([]tool.Config{
            {
                ID:          "data-processor",
                Name:        "Data Processor",
                Description: "Processes data",
                Runtime:     "bun",
                Code:        "export default () => { return 'processed'; }",
            },
        }),
        workflow.WithAgents([]agent.Config{
            {
                ID:           "assistant",
                Instructions: "You are a helpful assistant",
            },
        }),
        workflow.WithTasks([]task.Config{
            {BaseConfig: task.BaseConfig{ID: "process", Type: task.TaskTypeBasic}},
            {BaseConfig: task.BaseConfig{ID: "finalize", Type: task.TaskTypeBasic}},
        }),
        workflow.WithOutputs(outputs),
        workflow.WithSchedule(schedule),
    )
    if err != nil {
        panic(err)
    }
    // Use cfg...
}
```

## API Reference

### Constructor

```go
func New(ctx context.Context, id string, opts ...Option) (*engineworkflow.Config, error)
```

Creates a new workflow configuration with the given ID and optional configuration.

**Parameters:**

- `ctx`: Context for logging and cancellation
- `id`: Unique identifier for the workflow (required, non-empty)
- `opts`: Variadic functional options

**Returns:**

- `*engineworkflow.Config`: Deep copied workflow configuration
- `error`: Validation errors (may be `*sdkerrors.BuildError` with multiple errors)

### Options

All options are generated from `engine/workflow/config.go`:

- `WithResource(resource string)` - Resource identifier
- `WithVersion(version string)` - Workflow version
- `WithDescription(description string)` - Human-readable description
- `WithSchemas(schemas []schema.Schema)` - JSON schemas for validation
- `WithOpts(opts Opts)` - Configuration options (input schema, env vars)
- `WithAuthor(author *core.Author)` - Author information
- `WithTools(tools []tool.Config)` - External tools
- `WithAgents(agents []agent.Config)` - AI agents
- `WithKnowledgeBases(knowledgeBases []knowledge.BaseConfig)` - Knowledge bases
- `WithKnowledge(knowledge []core.KnowledgeBinding)` - Knowledge bindings
- `WithMCPs(mcps []mcp.Config)` - MCP servers
- `WithTriggers(triggers []Trigger)` - Event triggers
- `WithTasks(tasks []task.Config)` - Sequential tasks (required)
- `WithOutputs(outputs *core.Output)` - Output mappings
- `WithSchedule(schedule *Schedule)` - Schedule configuration
- `WithCWD(cwd *core.PathCWD)` - Current working directory

## Migration Guide

### Before (Old SDK)

```go
cfg, err := workflow.New("my-workflow").
    WithDescription("My workflow").
    AddTask(&task.Config{
        BaseConfig: task.BaseConfig{ID: "task1"},
    }).
    Build(ctx)
```

### After (New SDK)

```go
cfg, err := workflow.New(ctx, "my-workflow",
    workflow.WithDescription("My workflow"),
    workflow.WithTasks([]task.Config{
        {BaseConfig: task.BaseConfig{ID: "task1", Type: task.TaskTypeBasic}},
    }),
)
```

### Key Changes

1. **Context First**: `ctx` is now the first parameter
2. **No Build()**: Configuration is created immediately
3. **Collection Methods**: Use plural names with slices (e.g., `WithTasks` instead of `AddTask`)
4. **Options as Arguments**: Pass options as variadic arguments
5. **Task Structure**: Tasks require proper `BaseConfig` with `ID` and `Type` fields

## Validation

The constructor performs comprehensive validation:

- **ID Validation**: Must be non-empty and valid format
- **Task Validation**: At least one task required
- **Duplicate Detection**: Task IDs must be unique
- **Error Collection**: All validation errors collected and returned together

## Examples

See `sdk/cmd/` directory for complete workflow examples.

## Testing

```bash
gotestsum --format pkgname -- -race -parallel=4 ./sdk/v2/workflow
```
