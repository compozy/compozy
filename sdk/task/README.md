# Task Package - Functional Options API

The `task` package provides a clean, type-safe API for creating task configurations using the functional options pattern. It replaces the previous builder pattern with ~70% code reduction while maintaining full functionality.

## Overview

This package provides 7 specialized constructors for different task types, all returning `*task.Config`:

- **New** - Basic tasks with agent or tool execution
- **NewRouter** - Conditional routing based on expressions
- **NewParallel** - Parallel task execution with strategies
- **NewCollection** - Iterate over collections
- **NewWait** - Wait for signals or events
- **NewSignal** - Send signals to other tasks
- **NewMemory** - Memory operations (read/write/append/delete)

## Installation

```go
import "github.com/compozy/compozy/sdk/v2/task"
```

## Basic Task

Create tasks that execute agents or tools:

```go
import (
    "context"
    "github.com/compozy/compozy/sdk/v2/task"
    "github.com/compozy/compozy/engine/agent"
)

// Minimal agent-based task
cfg, err := task.New(ctx, "process-data",
    task.WithAgent(&agent.Config{ID: "data-processor"}),
)

// Full configuration with all options
cfg, err := task.New(ctx, "analyze",
    task.WithAgent(&agent.Config{ID: "analyzer"}),
    task.WithAction("analyze"),
    task.WithPrompt("Analyze the data"),
    task.WithTimeout("30s"),
    task.WithRetries(3),
    task.WithSleep("1s"),
    task.WithOnSuccess(&core.SuccessTransition{
        Next: strPtr("next-task"),
    }),
    task.WithOnError(&core.ErrorTransition{
        Next: strPtr("error-handler"),
    }),
)
```

## Router Task

Route execution based on conditions:

```go
routes := map[string]any{
    "true":  "approved-path",
    "false": "rejected-path",
}

cfg, err := task.NewRouter(ctx, "approval-router",
    task.WithCondition("input.approved == true"),
    task.WithRoutes(routes),
)
```

## Parallel Task

Execute multiple tasks concurrently:

```go
import enginetask "github.com/compozy/compozy/engine/task"

tasks := []enginetask.Config{
    {BaseConfig: enginetask.BaseConfig{ID: "task-1", Type: enginetask.TaskTypeBasic}},
    {BaseConfig: enginetask.BaseConfig{ID: "task-2", Type: enginetask.TaskTypeBasic}},
    {BaseConfig: enginetask.BaseConfig{ID: "task-3", Type: enginetask.TaskTypeBasic}},
}

// Default strategy: WaitAll
cfg, err := task.NewParallel(ctx, "parallel-processing", tasks)

// Custom strategy and workers
cfg, err := task.NewParallel(ctx, "parallel-processing", tasks,
    task.WithStrategy(enginetask.StrategyFailFast),
    task.WithMaxWorkers(5),
)
```

### Parallel Strategies

- **StrategyWaitAll** (default) - Wait for all tasks to complete
- **StrategyFailFast** - Stop on first failure
- **StrategyBestEffort** - Continue despite failures
- **StrategyRace** - Stop after first success

## Collection Task

Process items in a collection:

```go
taskTemplate := &enginetask.Config{
    BaseConfig: enginetask.BaseConfig{
        ID:   "process-item",
        Type: enginetask.TaskTypeBasic,
    },
}

// Sequential processing
cfg, err := task.NewCollection(ctx, "process-users", "workflow.users",
    task.WithTask(taskTemplate),
    task.WithMode(enginetask.CollectionModeSequential),
)

// Parallel processing with batching
cfg, err := task.NewCollection(ctx, "process-items", "data.items",
    task.WithTask(taskTemplate),
    task.WithMode(enginetask.CollectionModeParallel),
    task.WithBatch(10),
)
```

## Wait Task

Wait for signals or events:

```go
// Wait indefinitely
cfg, err := task.NewWait(ctx, "wait-approval", "user-approval")

// Wait with timeout
cfg, err := task.NewWait(ctx, "wait-payment", "payment-complete",
    task.WithTimeout("5m"),
)
```

## Signal Task

Send signals to waiting tasks:

```go
// Simple signal
cfg, err := task.NewSignal(ctx, "notify-completion", "process-complete")

// Signal with payload
payload := map[string]any{
    "status": "success",
    "result": "data-processed",
}
cfg, err := task.NewSignal(ctx, "send-result", "task-done",
    task.WithPayload(payload),
)
```

## Memory Task

Perform memory operations:

```go
import enginetask "github.com/compozy/compozy/engine/task"

// Read from memory
cfg, err := task.NewMemory(ctx, "read-session", enginetask.MemoryOpRead,
    task.WithMemoryRef("session-data"),
    task.WithKeyTemplate("user-{{.user_id}}"),
)

// Write to memory
cfg, err := task.NewMemory(ctx, "save-result", enginetask.MemoryOpWrite,
    task.WithMemoryRef("results"),
    task.WithKeyTemplate("result-{{.task_id}}"),
)

// Append to list
cfg, err := task.NewMemory(ctx, "add-item", enginetask.MemoryOpAppend,
    task.WithMemoryRef("items"),
)

// Delete key
cfg, err := task.NewMemory(ctx, "remove-data", enginetask.MemoryOpDelete,
    task.WithMemoryRef("cache"),
    task.WithKeyTemplate("temp-{{.id}}"),
)
```

### Memory Operations

- **MemoryOpRead** - Read value from memory
- **MemoryOpWrite** - Write value to memory
- **MemoryOpAppend** - Append to list
- **MemoryOpDelete** - Delete key
- **MemoryOpClear** - Clear all memory
- **MemoryOpFlush** - Flush to persistent storage
- **MemoryOpHealth** - Check memory health
- **MemoryOpStats** - Get memory statistics

## Common Options

All task types support these common options:

```go
task.WithTimeout("30s")              // Task timeout
task.WithRetries(3)                  // Retry attempts
task.WithSleep("1s")                 // Delay before execution
task.WithName("Display Name")        // Human-readable name
task.WithDescription("Details...")   // Task description
task.WithLabels(map[string]string{   // Metadata labels
    "env": "prod",
    "team": "data",
})
task.WithOnSuccess(&core.SuccessTransition{
    Next: strPtr("next-task"),
})
task.WithOnError(&core.ErrorTransition{
    Next: strPtr("error-handler"),
})
```

## Validation

All constructors perform comprehensive validation:

- **ID validation** - Non-empty, trimmed IDs required
- **Type-specific validation** - Each task type validates its required fields
- **Error accumulation** - All validation errors collected and returned together
- **Whitespace trimming** - IDs and string fields automatically trimmed

Example validation errors:

```go
// Empty ID
_, err := task.New(ctx, "")
// Error: "task id is invalid: cannot be empty"

// Invalid timeout
_, err := task.New(ctx, "task-1", task.WithTimeout("invalid"))
// Error: "invalid timeout duration: invalid"

// Parallel with too few tasks
_, err := task.NewParallel(ctx, "parallel", []enginetask.Config{oneTask})
// Error: "parallel task requires at least 2 tasks"

// Multiple errors accumulated
_, err := task.NewParallel(ctx, "", []enginetask.Config{})
// Error: "task id is invalid: cannot be empty; parallel task requires at least 2 tasks"
```

## Deep Copy Behavior

All constructors return deep copies to prevent external mutation:

```go
cfg1, _ := task.New(ctx, "task-1", task.WithAgent(&agent.Config{ID: "agent-1"}))
cfg2, _ := task.New(ctx, "task-2", task.WithAgent(&agent.Config{ID: "agent-2"}))

// Modifying cfg1 doesn't affect cfg2
cfg1.ID = "modified"
// cfg2.ID is still "task-2"
```

## Helper Functions

```go
// Create pointer to string (useful for transitions)
func strPtr(s string) *string {
    return &s
}

// Usage
task.WithOnSuccess(&core.SuccessTransition{
    Next: strPtr("next-task"),
})
```

## Code Generation

This package uses code generation for functional options:

```bash
# Regenerate options (if engine/task/config.go changes)
cd sdk/task
go generate
```

The generator creates ~50 functional options from `engine/task/config.go`.

## Migration from Builder Pattern

**Before (Builder Pattern):**

```go
builder := task.NewBuilder().
    SetID("task-1").
    SetAgent(agentCfg).
    SetTimeout("30s").
    SetRetries(3)

cfg, err := builder.Build(ctx)
```

**After (Functional Options):**

```go
cfg, err := task.New(ctx, "task-1",
    task.WithAgent(agentCfg),
    task.WithTimeout("30s"),
    task.WithRetries(3),
)
```

## Benefits

✅ **70% code reduction** - From ~300 LOC to ~90 LOC per constructor
✅ **Type safety** - Compile-time validation of options
✅ **Clear intent** - Constructor names indicate task type
✅ **Immutability** - Deep copy prevents external mutation
✅ **Error accumulation** - All validation errors reported at once
✅ **Auto-generation** - Options generated from engine structs

## Testing

All constructors have comprehensive test coverage:

- Minimal valid configurations
- Full configurations with all options
- Error cases and validation
- Deep copy behavior
- Whitespace trimming
- Error accumulation
- Transition configuration

Run tests:

```bash
gotestsum -- -race -parallel=4 ./sdk/task
```
