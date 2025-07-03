---
status: pending # Options: pending, in-progress, completed, excluded
---

<task_context>
<domain>engine/task/uc</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>orchestrators,migration_adapter</dependencies>
</task_context>

# Task 16.0: Response Handling Migration

## Overview

Migrate response handling logic from use cases to orchestrators, ensuring all task-specific response processing is encapsulated within the appropriate orchestrator implementations.

## Subtasks

- [ ] 16.1 Identify all response handling patterns in use cases
- [ ] 16.2 Move parallel task response aggregation to orchestrator
- [ ] 16.3 Move collection task response handling to orchestrator
- [ ] 16.4 Move composite task response chaining to orchestrator
- [ ] 16.5 Update error response handling in orchestrators
- [ ] 16.6 Remove response logic from use cases
- [ ] 16.7 Write tests for response handling migration
- [ ] 16.8 Validate response compatibility

## Implementation Details

### Response Handling Analysis

Current use cases contain type-specific response handling:

```go
// OLD: engine/task/uc/handle_response.go
func (uc *HandleTaskResponse) Execute(ctx context.Context, input HandleTaskResponseInput) (*task.Response, error) {
    switch input.State.Type {
    case task.TaskTypeParallel:
        return uc.handleParallelResponse(ctx, input)
    case task.TaskTypeCollection:
        return uc.handleCollectionResponse(ctx, input)
    // ... more switches
    }
}
```

### Migrated Response Handling in Orchestrators

#### Parallel Response Handling (engine/task2/parallel/orchestrator.go)

```go
func (o *Orchestrator) HandleResponse(ctx context.Context, input interfaces.HandleResponseInput) (*task.Response, error) {
    // Check if this is a child task completion
    if input.State.ParentStateID != nil {
        return o.handleChildResponse(ctx, input)
    }

    // Parent task completion - aggregate child results
    children, err := o.TaskRepo.GetChildStates(ctx, input.State.TaskExecID)
    if err != nil {
        return nil, fmt.Errorf("failed to get child states: %w", err)
    }

    // Calculate final status based on strategy
    finalStatus, err := o.CalculateStatus(ctx, input.State.TaskExecID)
    if err != nil {
        return nil, err
    }

    // Aggregate outputs
    aggregatedOutput := o.aggregateChildOutputs(children)

    return &task.Response{
        Status: finalStatus,
        Output: &aggregatedOutput,
        Metadata: map[string]interface{}{
            "completed_children": len(children),
            "strategy": o.getStrategy(input.State),
        },
    }, nil
}

func (o *Orchestrator) aggregateChildOutputs(children []*task.State) core.Output {
    outputs := make([]interface{}, 0, len(children))
    errors := make([]interface{}, 0)

    for _, child := range children {
        if child.Output != nil {
            outputs = append(outputs, *child.Output)
        }
        if child.Error != "" {
            errors = append(errors, map[string]interface{}{
                "task_id": child.ID,
                "error":   child.Error,
            })
        }
    }

    result := core.Output{
        "results": outputs,
    }

    if len(errors) > 0 {
        result["errors"] = errors
    }

    return result
}
```

#### Collection Response Handling (engine/task2/collection/orchestrator.go)

```go
func (o *Orchestrator) HandleResponse(ctx context.Context, input interfaces.HandleResponseInput) (*task.Response, error) {
    // Collection-specific response handling
    metadata, err := o.GetChildrenMetadata(ctx, input.State.TaskExecID)
    if err != nil {
        return nil, err
    }

    // Check if more batches need processing
    processedCount := o.getProcessedCount(ctx, input.State.TaskExecID)
    if processedCount < metadata.Count {
        // Return partial completion
        return &task.Response{
            Status: core.StatusRunning,
            Output: input.Output,
            Metadata: map[string]interface{}{
                "processed": processedCount,
                "total":     metadata.Count,
                "batch":     metadata.CustomFields["currentBatch"],
            },
        }, nil
    }

    // All items processed
    return &task.Response{
        Status: core.StatusSuccess,
        Output: o.finalizeCollectionOutput(ctx, input.State.TaskExecID),
    }, nil
}
```

### Use Case Cleanup

```go
// NEW: engine/task/uc/handle_response.go - SIMPLIFIED
func (uc *HandleTaskResponse) Execute(ctx context.Context, input HandleTaskResponseInput) (*task.Response, error) {
    // Delegate to orchestrator via activity
    return uc.activities.HandleTaskResponse.Run(ctx, &activities.HandleTaskResponseInput{
        State:          input.State,
        ExecutionError: input.ExecutionError,
        Output:         input.Output,
    })
}
```

### Key Migration Points

- Type-specific logic moves to orchestrators
- Use cases become thin delegation layers
- Response aggregation handled by orchestrators
- Error handling consolidated
- Metadata enrichment in orchestrators

## Success Criteria

- All response handling logic migrated to orchestrators
- Use cases no longer contain type switches
- Response format compatibility maintained
- Error handling works correctly
- Metadata properly included in responses
- All tests updated and passing
- No functionality regression
- Clean separation of concerns

<critical>
**MANDATORY REQUIREMENTS:**

- **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
- **NEVER** use workarounds, especially in tests - implement proper solutions
- **MUST** follow all established project standards:
    - Architecture patterns: `.cursor/rules/architecture.mdc`
    - Go coding standards: `.cursor/rules/go-coding-standards.mdc`
    - Testing requirements: `.cursor/rules/testing-standards.mdc`
    - API standards: `.cursor/rules/api-standards.mdc`
    - Security & quality: `.cursor/rules/quality-security.mdc`
- **MUST** run `make lint` and `make test` before completing parent tasks
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for parent tasks

**Enforcement:** Violating these standards results in immediate task rejection.
</critical>
