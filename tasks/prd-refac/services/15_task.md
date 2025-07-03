---
status: pending # Options: pending, in-progress, completed, excluded
---

<task_context>
<domain>engine/task/activities</domain>
<type>integration</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>orchestrators,activities</dependencies>
</task_context>

# Task 15.0: Direct Integration Update

## Overview

Update all integration points to directly use the new orchestrator-based architecture. Since we're in dev/alpha phase with no backwards compatibility requirements, we can directly replace the old system.

## Subtasks

- [ ] 15.1 Update all activities to use orchestrator factory directly
- [ ] 15.2 Remove all references to old services
- [ ] 15.3 Update dependency injection to provide orchestrator context
- [ ] 15.4 Clean up old service initialization code
- [ ] 15.5 Update tests to use new architecture
- [ ] 15.6 Remove old service interfaces
- [ ] 15.7 Verify all integration points updated
- [ ] 15.8 Run integration test suite

## Implementation Details

### Direct Activity Updates (engine/task/activities/activities.go)

```go
type Activities struct {
    factory     *factory.OrchestratorFactory
    orchContext *shared.OrchestratorContext
    logger      *slog.Logger
}

func NewActivities(deps *Dependencies) *Activities {
    orchContext := &shared.OrchestratorContext{
        TaskRepo:       deps.TaskRepo,
        WorkflowRepo:   deps.WorkflowRepo,
        Storage:        deps.Storage,
        TemplateEngine: deps.TemplateEngine,
    }

    return &Activities{
        factory:     factory.NewOrchestratorFactory(),
        orchContext: orchContext,
        logger:      deps.Logger,
    }
}
```

### Update CreateTaskState Activity

```go
func (a *Activities) CreateTaskState(ctx context.Context, input *CreateTaskStateInput) (*task.State, error) {
    // Direct orchestrator usage - no compatibility layer needed
    orchestrator, err := a.factory.Create(input.TaskConfig.Type, a.orchContext)
    if err != nil {
        return nil, fmt.Errorf("failed to create orchestrator for %s: %w", input.TaskConfig.Type, err)
    }

    return orchestrator.CreateState(ctx, interfaces.CreateStateInput{
        WorkflowState:  input.WorkflowState,
        WorkflowConfig: input.WorkflowConfig,
        TaskConfig:     input.TaskConfig,
        ParentStateID:  input.ParentStateID,
    })
}
```

### Remove Old Service References

```go
// DELETE: engine/task/services/services.go
// This entire file can be removed as we no longer need the old services

// UPDATE: engine/cmd/server/main.go
func setupActivities(db *sql.DB) *activities.Activities {
    // Direct dependency setup
    deps := &activities.Dependencies{
        TaskRepo:       repo.NewTaskRepository(db),
        WorkflowRepo:   repo.NewWorkflowRepository(db),
        Storage:        storage.NewRedisStorage(redis.NewClient(&redis.Options{})),
        TemplateEngine: tplengine.New(),
        Logger:         slog.Default(),
    }

    return activities.NewActivities(deps)
}
```

### Update Tests

```go
// engine/task/activities/create_state_test.go
func TestCreateTaskState(t *testing.T) {
    activities := setupTestActivities()

    tests := []struct {
        name     string
        taskType task.TaskType
    }{
        {"basic task", task.TaskTypeBasic},
        {"parallel task", task.TaskTypeParallel},
        {"wait task", task.TaskTypeWait},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            input := &CreateTaskStateInput{
                TaskConfig: &task.Config{Type: tt.taskType},
                // ... other fields
            }

            state, err := activities.CreateTaskState(context.Background(), input)
            require.NoError(t, err)
            assert.Equal(t, tt.taskType, state.Type)
        })
    }
}
```

### Key Changes

- Direct orchestrator factory usage
- No dual-path logic needed
- Simple, clean integration
- Remove all old service code
- Update dependency injection
- Straightforward testing

## Success Criteria

- All activities use orchestrators directly
- Old service code completely removed
- Tests updated and passing
- No references to old services remain
- Clean dependency injection
- Integration tests pass
- Simplified codebase
- Clear error messages

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
