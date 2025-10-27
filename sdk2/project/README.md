# Project Package

SDK for creating Compozy project configurations using functional options.

## Overview

The project package provides a type-safe, ergonomic API for building project configurations - the top-level orchestrator that integrates agents, workflows, tasks, tools, memory, knowledge, and schedules.

## Installation

```go
import "github.com/compozy/compozy/sdk2/project"
```

## Usage

### Minimal Project

```go
cfg, err := project.New(ctx, "my-project",
    project.WithWorkflows([]*engineproject.WorkflowSourceConfig{
        {Source: "./workflow.yaml"},
    }),
)
```

### Full Project Configuration

```go
cfg, err := project.New(ctx, "enterprise-ai-system",
    project.WithVersion("2.1.0"),
    project.WithDescription("Multi-agent system for enterprise automation"),
    project.WithAuthor(core.Author{
        Name:         "AI Team",
        Email:        "ai@company.com",
        Organization: "ACME Corp",
    }),
    project.WithWorkflows([]*engineproject.WorkflowSourceConfig{
        {Source: "./workflows/customer-support.yaml"},
        {Source: "./workflows/data-pipeline.yaml"},
    }),
    project.WithModels([]*core.ProviderConfig{
        {
            Provider: core.ProviderOpenAI,
            Model:    "gpt-4",
            APIKey:   "{{.env.OPENAI_API_KEY}}",
            Default:  true,
        },
        {
            Provider: core.ProviderAnthropic,
            Model:    "claude-3-opus",
            APIKey:   "{{.env.ANTHROPIC_API_KEY}}",
        },
    }),
    project.WithTools([]tool.Config{
        {ID: "code-analyzer", Description: "Analyzes code quality"},
        {ID: "data-processor", Description: "Processes data"},
    }),
    project.WithMemories([]*memory.Config{
        {ID: "conversation", Type: memory.TypeBuffer},
    }),
    project.WithEmbedders([]knowledge.EmbedderConfig{
        {ID: "openai-embedder", Provider: "openai"},
    }),
    project.WithVectorDBs([]knowledge.VectorDBConfig{
        {ID: "pinecone-db", Type: "pinecone"},
    }),
    project.WithKnowledgeBases([]knowledge.BaseConfig{
        {ID: "company-docs"},
    }),
    project.WithSchedules([]*projectschedule.Config{
        {
            ID:         "daily-report",
            WorkflowID: "./workflows/data-pipeline.yaml",
            Cron:       "0 9 * * *",
        },
    }),
)
```

## API Reference

### Constructor

```go
func New(ctx context.Context, name string, opts ...Option) (*engineproject.Config, error)
```

Creates a new project configuration with the given name and options.

**Parameters:**

- `ctx` - Context for cancellation and logging
- `name` - Project name (alphanumeric and hyphens only)
- `opts` - Functional options for configuration

**Returns:**

- `*engineproject.Config` - Deep copied configuration
- `error` - Validation errors (may be `*sdkerrors.BuildError` with multiple errors)

### Options

#### Core Options

- `WithVersion(version string)` - Sets semantic version (e.g., "1.0.0")
- `WithDescription(description string)` - Sets project description
- `WithAuthor(author core.Author)` - Sets author information

#### Resource Options

- `WithWorkflows(workflows []*WorkflowSourceConfig)` - Registers workflow files (required)
- `WithModels(models []*core.ProviderConfig)` - Registers LLM providers
- `WithTools(tools []tool.Config)` - Registers shared tools
- `WithMemories(memories []*memory.Config)` - Registers memory resources
- `WithSchedules(schedules []*projectschedule.Config)` - Registers scheduled workflows

#### Knowledge Options

- `WithEmbedders(embedders []knowledge.EmbedderConfig)` - Registers embedders
- `WithVectorDBs(vectorDBs []knowledge.VectorDBConfig)` - Registers vector databases
- `WithKnowledgeBases(bases []knowledge.BaseConfig)` - Registers knowledge bases
- `WithKnowledge(bindings []core.KnowledgeBinding)` - Sets knowledge binding (max 1)

#### Advanced Options

- `WithSchemas(schemas []schema.Schema)` - Registers data schemas
- `WithOpts(opts Opts)` - Sets project configuration options
- `WithRuntime(runtime RuntimeConfig)` - Sets runtime configuration
- `WithAutoLoad(autoload *autoload.Config)` - Sets autoload configuration
- `WithMCPs(mcps []mcp.Config)` - Registers MCP servers
- `WithMonitoringConfig(config *monitoring.Config)` - Sets monitoring
- `WithCWD(cwd *core.PathCWD)` - Sets working directory

### Cross-Reference Validation

```go
func ValidateCrossReferences(
    cfg *engineproject.Config,
    agents []agent.Config,
    workflows []workflow.Config,
) error
```

Validates that agents reference valid tools, memory, and knowledge bases defined in the project config.

**Note:** This performs basic structural validation. Full semantic validation (e.g., verifying agent references in tasks) happens during workflow execution.

## Validation Rules

### Required Fields

- **Name**: Non-empty, alphanumeric with hyphens, max 63 characters
- **Workflows**: At least one workflow source must be registered

### Version

- Must be valid semantic version if specified (e.g., "1.0.0", "2.1.0-alpha.1")

### Author Email

- Must be valid email format if specified

### Models

- Only one model can be marked as default

### Workflows

- Source path cannot be empty
- Source path is relative to project root

### Schedules

- Schedule IDs must be unique
- Workflow IDs must be empty
- Schedule must reference existing workflow source

### Tools

- Tool IDs must be unique and non-empty

### Memories

- Memory IDs must be unique and non-empty
- Resource field defaults to "memory" if empty

### Knowledge

- Only one knowledge binding supported (MVP)
- Embedder IDs must be unique
- VectorDB IDs must be unique
- Knowledge base IDs must be unique

## Migration Guide

### From Old SDK (Builder Pattern)

**Before:**

```go
cfg, err := project.New("my-project").
    WithVersion("1.0.0").
    WithDescription("My project").
    AddWorkflow(wf).
    AddModel(model).
    Build(ctx)
```

**After:**

```go
cfg, err := project.New(ctx, "my-project",
    project.WithVersion("1.0.0"),
    project.WithDescription("My project"),
    project.WithWorkflows([]*engineproject.WorkflowSourceConfig{
        {Source: "./workflow.yaml"},
    }),
    project.WithModels([]*core.ProviderConfig{model}),
)
```

### Key Changes

1. **Context First**: `ctx` is now the first parameter
2. **No Build()**: Configuration is created and validated immediately
3. **Slices for Collections**: Use `WithWorkflows()`, `WithModels()`, etc. with slices
4. **Immediate Validation**: Errors are returned from `New()`, not from `Build()`
5. **Deep Copy**: Returned config is automatically deep copied for safety

## Examples

See `sdk/cmd/` for complete working examples of project configurations.

## Testing

```bash
# Run tests
gotestsum -- -race -parallel=4 ./sdk2/project

# Run with coverage
go test -race -coverprofile=coverage.out ./sdk2/project
go tool cover -html=coverage.out
```

## Error Handling

The constructor returns `*sdkerrors.BuildError` when multiple validation errors occur:

```go
cfg, err := project.New(ctx, "",  // Empty name
    project.WithVersion("invalid"),  // Invalid version
)
if err != nil {
    var buildErr *sdkerrors.BuildError
    if errors.As(err, &buildErr) {
        for _, e := range buildErr.Errors {
            fmt.Println("Error:", e)
        }
    }
}
```

## Best Practices

1. **Use Context**: Always pass a valid context with logger
2. **Validate Early**: Run `New()` during application initialization
3. **Organize Workflows**: Group related workflows in subdirectories
4. **Reference by ID**: Use consistent IDs for tools, memory, and knowledge
5. **Single Default Model**: Mark only one model as default
6. **Schedule Wisely**: Use cron expressions for workflow schedules
7. **Document Resources**: Provide clear descriptions for tools and workflows

## Performance

- **Constructor Time**: < 1µs for minimal config, < 10µs for full config
- **Memory Allocation**: < 5KB per constructor call
- **Deep Copy Overhead**: Negligible (~100ns) using efficient `core.DeepCopy()`

## Thread Safety

The `New()` constructor is thread-safe. The returned configuration is a deep copy and safe for concurrent read access. Modifications to the returned config do not affect the internal state.
