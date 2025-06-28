---
status: pending # Options: pending, in-progress, completed, excluded
---

<task_context>
<domain>engine/worker/executors</domain>
<type>integration</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>activities,orchestrators</dependencies>
</task_context>

# Task 14.0: Workflow Integration

## Overview

Update the workflow executors to use the new orchestrator-based activities, ensuring seamless integration between the workflow engine and the new task orchestration architecture.

## Subtasks

- [ ] 14.1 Update TaskExecutor to use orchestrator activities
- [ ] 14.2 Update child task creation workflows
- [ ] 14.3 Modify signal handling workflows
- [ ] 14.4 Update status update workflows
- [ ] 14.5 Add orchestrator-aware error handling
- [ ] 14.6 Update workflow registration
- [ ] 14.7 Write workflow integration tests
- [ ] 14.8 Validate performance with new architecture

## Implementation Details

### Updated Task Executor (engine/worker/executors/task_executor.go)

```go
type TaskExecutor struct {
    activities *Activities
    logger     *slog.Logger
}

func (e *TaskExecutor) Execute(ctx workflow.Context, input ExecuteTaskInput) (*ExecuteTaskOutput, error) {
    // Create task state using orchestrator
    var state *task.State
    err := workflow.ExecuteActivity(ctx, e.activities.CreateTaskState.Run, &CreateTaskStateInput{
        WorkflowState:  input.WorkflowState,
        WorkflowConfig: input.WorkflowConfig,
        TaskConfig:     input.TaskConfig,
        ParentStateID:  input.ParentStateID,
    }).Get(ctx, &state)

    if err != nil {
        return nil, fmt.Errorf("failed to create task state: %w", err)
    }

    // Execute task logic based on type
    output, err := e.executeTaskLogic(ctx, state, input)
    if err != nil {
        return nil, err
    }

    // Handle response through orchestrator
    var response *task.Response
    err = workflow.ExecuteActivity(ctx, e.activities.HandleTaskResponse.Run, &HandleTaskResponseInput{
        State:          state,
        ExecutionError: err,
        Output:         output,
    }).Get(ctx, &response)

    if err != nil {
        return nil, fmt.Errorf("failed to handle task response: %w", err)
    }

    return &ExecuteTaskOutput{
        State:    state,
        Response: response,
    }, nil
}
```

### Child Task Workflow Updates

```go
func (e *TaskExecutor) executeContainerTask(ctx workflow.Context, state *task.State) (*core.Output, error) {
    // Check if task has children through orchestrator metadata
    var metadata interfaces.ChildrenMetadata
    err := workflow.ExecuteActivity(ctx, e.activities.GetChildrenMetadata.Run, state.TaskExecID).Get(ctx, &metadata)
    if err != nil {
        return nil, fmt.Errorf("failed to get children metadata: %w", err)
    }

    if metadata.Count == 0 {
        return &core.Output{}, nil // No children
    }

    // Create child workflows based on metadata
    childSelector := workflow.NewSelector(ctx)
    childOutputs := make(map[string]*core.Output)

    for i := 0; i < metadata.Count; i++ {
        future := workflow.ExecuteChildWorkflow(ctx, e.Execute, ExecuteTaskInput{
            // Child task input
        })

        childSelector.AddFuture(future, func(f workflow.Future) {
            var output *ExecuteTaskOutput
            if err := f.Get(ctx, &output); err == nil {
                childOutputs[output.State.ID.String()] = output.Response.Output
            }
        })
    }

    // Wait based on strategy
    if metadata.Strategy == "all" {
        for i := 0; i < metadata.Count; i++ {
            childSelector.Select(ctx)
        }
    } else {
        childSelector.Select(ctx) // Any strategy
    }

    return e.aggregateOutputs(childOutputs), nil
}
```

### Signal Handling Integration

```go
func (e *TaskExecutor) handleSignalTask(ctx workflow.Context, state *task.State) (*core.Output, error) {
    signalChan := workflow.GetSignalChannel(ctx, state.WaitFor)

    var signal interfaces.Signal
    signalChan.Receive(ctx, &signal)

    // Validate and process through orchestrator
    var updatedState *task.State
    err := workflow.ExecuteActivity(ctx, e.activities.ProcessSignal.Run, &ProcessSignalInput{
        State:  state,
        Signal: signal,
    }).Get(ctx, &updatedState)

    if err != nil {
        return nil, fmt.Errorf("signal processing failed: %w", err)
    }

    return updatedState.Output, nil
}
```

### Key Integration Changes

- Activities use orchestrator adapter
- Child creation based on orchestrator metadata
- Signal handling through orchestrator
- Status updates via orchestrator interfaces
- Error handling preserves orchestrator context

## Success Criteria

- All workflows successfully use new activities
- Child task creation works for all container types
- Signal handling properly integrated
- Status updates flow through orchestrators
- No regression in workflow functionality
- Performance meets requirements
- Error handling provides proper diagnostics
- E2E tests pass with new architecture

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
