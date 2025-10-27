# Agent SDK - Functional Options Pattern

This package provides a fluent, type-safe API for creating AI agent configurations using the **functional options pattern** with **auto-generated option functions**.

## Quick Start

```go
package main

import (
    "context"
    "github.com/compozy/compozy/engine/core"
    "github.com/compozy/compozy/sdk/agent"
)

func main() {
    ctx := context.Background()
    
    agentCfg, err := agent.New(ctx, "assistant",
        agent.WithInstructions("You are a helpful AI assistant"),
        agent.WithModel(agent.Model{
            Config: core.ProviderConfig{
                Provider: core.ProviderOpenAI,
                Model:    "gpt-4",
            },
        }),
        agent.WithMaxIterations(10),
    )
    if err != nil {
        panic(err)
    }
    
    // Use agentCfg in your workflow
}
```

## Available Options

### Core Configuration
- `WithInstructions(string)` - System instructions for the agent (required)
- `WithModel(Model)` - LLM model configuration
- `WithMaxIterations(int)` - Maximum reasoning iterations (default: 5)

### Capabilities
- `WithTools([]tool.Config)` - Available tools for the agent
- `WithMCPs([]mcp.Config)` - Model Context Protocol servers
- `WithActions([]*ActionConfig)` - Structured actions with schemas

### Context & Memory
- `WithMemory([]core.MemoryReference)` - Persistent memory access
- `WithKnowledge([]core.KnowledgeBinding)` - Knowledge base integration

### Environment
- `WithWith(*core.Input)` - Default input parameters
- `WithEnv(*core.EnvMap)` - Environment variables
- `WithAttachments(attachment.Attachments)` - File attachments

## Code Generation

This package uses **auto-generated functional options** from the engine structs:

```bash
# Regenerate options when engine/agent/config.go changes
cd sdk/agent
go generate
```

The `options_generated.go` file is created automatically by parsing `engine/agent/config.go`.

## Architecture

- **`constructor.go`** - Main constructor with validation logic
- **`options_generated.go`** - Auto-generated option functions (DO NOT EDIT)
- **`generate.go`** - go:generate directive
- **`action.go`** - Action configuration helpers

## Validation

The constructor performs comprehensive validation:
- ✅ ID must be non-empty and valid
- ✅ Instructions are required
- ✅ Model must have either ref or config
- ✅ Maximum one knowledge binding
- ✅ Memory references must have IDs
- ✅ All inputs are trimmed and normalized

## Error Handling

Validation errors are collected and returned as a `BuildError`:

```go
agentCfg, err := agent.New(ctx, "",  // Empty ID
    agent.WithInstructions(""),       // Empty instructions
)

// err is *sdkerrors.BuildError with multiple validation errors
```

## Testing

Run tests:
```bash
make test          # All project tests
gotestsum --format pkgname -- -race -parallel=4 ./sdk/agent
```

All tests use the functional options pattern and validate the full configuration lifecycle.

