---
status: pending # Options: pending, in-progress, completed, excluded
---

<task_context>
<domain>engine/task/activities</domain>
<type>integration</type>
<scope>middleware</scope>
<complexity>high</complexity>
<dependencies>orchestrators,factory</dependencies>
</task_context>

# Task 13.0: Activity Adapter Implementation

## Overview

Create the activity adapter layer that integrates the new orchestrator-based architecture with the existing Temporal workflow activities. This adapter replaces type-specific activities with a unified orchestrator-based approach.

## Subtasks

- [ ] 13.1 Create OrchestratorAdapter activity base class
- [ ] 13.2 Replace CreateTaskState activity with orchestrator-based version
- [ ] 13.3 Replace HandleTaskResponse activity with orchestrator version
- [ ] 13.4 Update PrepareExecution activity to use orchestrators
- [ ] 13.5 Create orchestrator context initialization
- [ ] 13.6 Add error handling and logging
- [ ] 13.7 Write unit tests for adapter activities
- [ ] 13.8 Create integration tests with workflows

## Implementation Details

### Orchestrator Adapter (engine/task/activities/orchestrator_adapter.go)

```go
type OrchestratorAdapter struct {
    factory     *factory.OrchestratorFactory
    orchContext *shared.OrchestratorContext
    logger      *slog.Logger
}

func NewOrchestratorAdapter(deps *Dependencies) *OrchestratorAdapter {
    orchContext := &shared.OrchestratorContext{
        TaskRepo:       deps.TaskRepo,
        WorkflowRepo:   deps.WorkflowRepo,
        Storage:        deps.Storage,
        TemplateEngine: deps.TemplateEngine,
    }

    return &OrchestratorAdapter{
        factory:     factory.NewOrchestratorFactory(),
        orchContext: orchContext,
        logger:      deps.Logger,
    }
}
```

### Create Task State Activity (engine/task/activities/create_state.go)

```go
type CreateTaskState struct {
    adapter *OrchestratorAdapter
}

func (a *CreateTaskState) Run(ctx context.Context, input *CreateTaskStateInput) (*task.State, error) {
    // Get appropriate orchestrator from factory
    orchestrator, err := a.adapter.factory.Create(input.TaskConfig.Type, a.adapter.orchContext)
    if err != nil {
        return nil, fmt.Errorf("failed to create orchestrator: %w", err)
    }

    // Log orchestrator selection
    a.adapter.logger.InfoContext(ctx, "Creating task state",
        slog.String("task_type", string(input.TaskConfig.Type)),
        slog.String("orchestrator", fmt.Sprintf("%T", orchestrator)),
    )

    // Delegate to orchestrator
    state, err := orchestrator.CreateState(ctx, interfaces.CreateStateInput{
        WorkflowState:  input.WorkflowState,
        WorkflowConfig: input.WorkflowConfig,
        TaskConfig:     input.TaskConfig,
        ParentStateID:  input.ParentStateID,
    })

    if err != nil {
        return nil, fmt.Errorf("orchestrator failed to create state: %w", err)
    }

    // Check if orchestrator implements ChildTaskManager
    if childManager, ok := orchestrator.(interfaces.ChildTaskManager); ok {
        if err := childManager.PrepareChildren(ctx, state, input.TaskConfig); err != nil {
            return nil, fmt.Errorf("failed to prepare children: %w", err)
        }
    }

    return state, nil
}
```

### Handle Task Response Activity

```go
type HandleTaskResponse struct {
    adapter *OrchestratorAdapter
}

func (a *HandleTaskResponse) Run(ctx context.Context, input *HandleTaskResponseInput) (*task.Response, error) {
    orchestrator, err := a.adapter.factory.Create(input.State.Type, a.adapter.orchContext)
    if err != nil {
        return nil, err
    }

    response, err := orchestrator.HandleResponse(ctx, interfaces.HandleResponseInput{
        State:          input.State,
        ExecutionError: input.ExecutionError,
        Output:         input.Output,
    })

    if err != nil {
        return nil, fmt.Errorf("orchestrator failed to handle response: %w", err)
    }

    // Update parent status if needed
    if aggregator, ok := orchestrator.(interfaces.StatusAggregator); ok && input.State.ParentStateID != nil {
        if aggregator.ShouldUpdateStatus(ctx, *input.State.ParentStateID, interfaces.ChildStatusUpdate{
            ChildID:   input.State.TaskExecID,
            NewStatus: response.Status,
        }) {
            // Trigger parent status update
            a.triggerParentUpdate(ctx, *input.State.ParentStateID)
        }
    }

    return response, nil
}
```

### Key Integration Points

- Factory-based orchestrator selection
- Context initialization with dependencies
- Interface checking for optional capabilities
- Error handling and logging
- Backward compatibility during migration

## Success Criteria

- All activities successfully use orchestrators
- Type-specific logic completely removed from activities
- Optional interfaces properly checked and used
- Error handling provides clear diagnostics
- Logging captures orchestrator decisions
- Integration with existing workflows maintained
- Performance meets or exceeds current system
- Migration path clearly defined

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
