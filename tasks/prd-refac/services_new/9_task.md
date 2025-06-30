---
status: pending
---

<task_context>
<domain>engine/worker/activities</domain>
<type>integration</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>factory_integration,all_response_handlers,activities_framework</dependencies>
</task_context>

# Task 9.0: Activities.go Integration

## Overview

Update Activities.go to use the new modular components instead of the monolithic TaskResponder and ConfigManager services. This is the critical integration phase where the new architecture is deployed to production code paths while maintaining backward compatibility.

## Subtasks

- [ ] 9.1 All Activities methods updated to use new components
- [ ] 9.2 Factory integration working correctly in Activities struct
- [ ] 9.3 Dependency injection properly configured
- [ ] 9.4 All existing workflows continue to function identically
- [ ] 9.5 Error handling preserved exactly
- [ ] 9.6 Context cancellation behavior maintained
- [ ] 9.7 Integration tests passing with new components

## Implementation Details

### Files to Modify

1. `engine/worker/activities.go` - Main integration file
2. `engine/task/activities/*.go` - Individual activity implementations
3. Any dependency injection configuration files

### Activities.go Structure Changes

**Add Factory to Activities Struct:**

```go
type Activities struct {
    projectConfig    *project.Config
    workflows        []*workflow.Config
    workflowRepo     workflow.Repository
    taskRepo         task.Repository
    runtime          *runtime.Manager
    configStore      services.ConfigStore
    signalDispatcher services.SignalDispatcher
    redisCache       *cache.Cache
    celEvaluator     task.ConditionEvaluator
    memoryManager    *memory.Manager
    memoryActivities *memacts.MemoryActivities
    templateEngine   *tplengine.TemplateEngine
    logger           logger.Logger

    // NEW: Add task2 factory
    task2Factory     task2.ExtendedFactory

    // REMOVE LATER: Legacy services (keep during transition)
    configManager    *services.ConfigManager  // TODO: Remove after validation
    taskResponder    *services.TaskResponder  // TODO: Remove after validation
}
```

**Factory Initialization in Constructor:**

```go
func NewActivities(...) *Activities {
    // ... existing initialization ...

    // Create task2 factory
    factoryConfig := &task2.FactoryConfig{
        TemplateEngine: templateEngine,
        EnvMerger:      envMerger,
        WorkflowRepo:   workflowRepo,
        TaskRepo:       taskRepo,
        ConfigStore:    configStore,
    }
    task2Factory := task2.NewExtendedFactory(factoryConfig)

    return &Activities{
        // ... existing fields ...
        task2Factory:     task2Factory,
        configManager:    configManager,  // Keep during transition
        taskResponder:    taskResponder,  // Keep during transition
    }
}
```

### Collection Task Integration

**Update CreateCollectionState Activity:**

```go
// BEFORE: Using ConfigManager
func (a *Activities) CreateCollectionState(
    ctx context.Context,
    input *tkfacts.CreateCollectionStateInput,
) (*task.State, error) {
    if err := ctx.Err(); err != nil {
        return nil, err
    }
    act, err := tkfacts.NewCreateCollectionState(
        a.workflows,
        a.workflowRepo,
        a.taskRepo,
        a.configStore,
        a.projectConfig.CWD,
    )
    // ... uses ConfigManager internally
}

// AFTER: Using CollectionExpander
func (a *Activities) CreateCollectionState(
    ctx context.Context,
    input *tkfacts.CreateCollectionStateInput,
) (*task.State, error) {
    if err := ctx.Err(); err != nil {
        return nil, err
    }

    // Get new components from factory
    expander := a.task2Factory.CreateCollectionExpander()
    repository := a.task2Factory.CreateTaskConfigRepository(a.configStore)

    act, err := tkfacts.NewCreateCollectionState(
        a.workflows,
        a.workflowRepo,
        a.taskRepo,
        expander,      // NEW: CollectionExpander
        repository,    // NEW: TaskConfigRepository
        a.projectConfig.CWD,
    )
    if err != nil {
        return nil, err
    }
    return act.Run(ctx, input)
}
```

### Response Handler Integration

**Update GetCollectionResponse Activity:**

```go
// BEFORE: Using TaskResponder
func (a *Activities) GetCollectionResponse(
    ctx context.Context,
    input *tkfacts.GetCollectionResponseInput,
) (*task.CollectionResponse, error) {
    if err := ctx.Err(); err != nil {
        return nil, err
    }
    act := tkfacts.NewGetCollectionResponse(a.workflowRepo, a.taskRepo, a.configStore)
    // ... uses TaskResponder internally
}

// AFTER: Using ResponseHandler
func (a *Activities) GetCollectionResponse(
    ctx context.Context,
    input *tkfacts.GetCollectionResponseInput,
) (*task.CollectionResponse, error) {
    if err := ctx.Err(); err != nil {
        return nil, err
    }

    // Get response handler from factory
    handler, err := a.task2Factory.CreateResponseHandler(task.TaskTypeCollection)
    if err != nil {
        return nil, fmt.Errorf("failed to create collection response handler: %w", err)
    }

    act := tkfacts.NewGetCollectionResponse(
        a.workflowRepo,
        a.taskRepo,
        a.configStore,
        handler,  // NEW: CollectionResponseHandler
    )
    return act.Run(ctx, input)
}
```

### All Activity Methods to Update

**Collection Activities:**

1. `CreateCollectionState` - Use CollectionExpander + TaskConfigRepository
2. `GetCollectionResponse` - Use CollectionResponseHandler

**Parallel Activities:** 3. `CreateParallelState` - Use TaskConfigRepository 4. `GetParallelResponse` - Use ParallelResponseHandler

**Basic Activities:** 5. `ExecuteBasicTask` - Use BasicResponseHandler

**Composite Activities:** 6. `CreateCompositeState` - Use TaskConfigRepository 7. `GetCompositeResponse` - Use CompositeResponseHandler

**Other Task Types:** 8. `ExecuteRouterTask` - Use RouterResponseHandler 9. `ExecuteSignalTask` - Use SignalResponseHandler 10. `ExecuteWaitTask` - Use WaitResponseHandler 11. `ExecuteAggregateTask` - Use AggregateResponseHandler

### Activity Implementation Updates

**Update Individual Activity Constructors:**

```go
// BEFORE: CreateCollectionState activity
func NewCreateCollectionState(
    workflows []*workflow.Config,
    workflowRepo workflow.Repository,
    taskRepo task.Repository,
    configStore services.ConfigStore,
    cwd *core.PathCWD,
) (*CreateCollectionState, error) {
    configManager, err := services.NewConfigManager(configStore, cwd)
    // ...
}

// AFTER: CreateCollectionState activity
func NewCreateCollectionState(
    workflows []*workflow.Config,
    workflowRepo workflow.Repository,
    taskRepo task.Repository,
    expander collection.CollectionExpander,     // NEW
    repository services.TaskConfigRepository,   // NEW
    cwd *core.PathCWD,
) (*CreateCollectionState, error) {
    return &CreateCollectionState{
        workflows:    workflows,
        workflowRepo: workflowRepo,
        taskRepo:     taskRepo,
        expander:     expander,     // NEW
        repository:   repository,   // NEW
        cwd:          cwd,
    }, nil
}
```

### Error Handling Integration

**Preserve Exact Error Behavior:**

```go
func (a *Activities) handleFactoryError(err error, component string) error {
    // Wrap factory errors to match existing error patterns
    return fmt.Errorf("failed to create %s: %w", component, err)
}

func (a *Activities) createResponseHandlerSafely(taskType task.Type) (shared.TaskResponseHandler, error) {
    handler, err := a.task2Factory.CreateResponseHandler(taskType)
    if err != nil {
        return nil, a.handleFactoryError(err, fmt.Sprintf("%s response handler", taskType))
    }
    return handler, nil
}
```

### Dependency Injection Configuration

**Factory Configuration Helper:**

```go
func (a *Activities) createTask2Factory() task2.ExtendedFactory {
    // Create environment merger if needed
    envMerger := core.NewEnvMerger()

    config := &task2.FactoryConfig{
        TemplateEngine: a.templateEngine,
        EnvMerger:      envMerger,
        WorkflowRepo:   a.workflowRepo,
        TaskRepo:       a.taskRepo,
        ConfigStore:    a.configStore,
    }

    return task2.NewExtendedFactory(config)
}
```

### Integration Testing Strategy

**Activities Integration Tests:**

```go
func TestActivities_NewComponents_Integration(t *testing.T) {
    // Setup real Activities with new components
    activities := setupRealActivities(t)

    testCases := []struct {
        name     string
        activity func() error
    }{
        {
            name: "CreateCollectionState with new components",
            activity: func() error {
                input := &tkfacts.CreateCollectionStateInput{...}
                _, err := activities.CreateCollectionState(ctx, input)
                return err
            },
        },
        {
            name: "GetCollectionResponse with new handler",
            activity: func() error {
                input := &tkfacts.GetCollectionResponseInput{...}
                _, err := activities.GetCollectionResponse(ctx, input)
                return err
            },
        },
        // ... test all activities
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            err := tc.activity()
            assert.NoError(t, err)
        })
    }
}
```

### Rollback Strategy

**Feature Flag Support:**

```go
func (a *Activities) shouldUseNewComponents() bool {
    // Check feature flag or environment variable
    return os.Getenv("USE_NEW_TASK_COMPONENTS") == "true"
}

func (a *Activities) GetCollectionResponse(ctx context.Context, input *tkfacts.GetCollectionResponseInput) (*task.CollectionResponse, error) {
    if a.shouldUseNewComponents() {
        return a.getCollectionResponseNew(ctx, input)
    }
    return a.getCollectionResponseLegacy(ctx, input)
}
```

## Dependencies

- Task 8: Behavior validation completed
- All new components fully tested and validated
- Individual activity implementations ready for update

## Testing Requirements

**Integration Test Coverage:**

- [ ] All Activities methods work with new components
- [ ] Error handling identical to legacy behavior
- [ ] Context cancellation behavior preserved
- [ ] Behavior parity validated
- [ ] Memory usage validated

**Regression Test Strategy:**

- Run existing integration test suite with new components
- Validate all workflows continue to function
- Monitor for any behavioral changes
- Behavior validation with real workloads

## Rollback Plan

**Immediate Rollback:**

- Keep legacy services available during transition
- Feature flag to switch between old and new implementations
- Monitoring to detect any issues quickly

**Gradual Migration:**

1. Deploy with feature flag off (legacy behavior)
2. Enable for specific task types incrementally
3. Monitor metrics and error rates
4. Full migration after validation

## Implementation Notes

- Update one activity method at a time
- Test each change thoroughly before proceeding
- Maintain exact error message compatibility
- Preserve all context cancellation behavior
- Document any intentional behavior changes

## Success Criteria

- All Activities methods updated to use new components
- Factory integration working correctly
- All existing integration tests passing
- Behavior validated as equivalent to legacy implementation
- Error handling behavior identical to legacy
- Rollback capability tested and available
- Code review approved
- Ready for legacy service removal
- Production code paths successfully updated to new architecture

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

## Validation Checklist

Before marking this task complete, verify:

- [ ] All 8 Activities methods updated to use new components
- [ ] Factory properly initialized in Activities constructor
- [ ] Component creation error handling implemented
- [ ] Existing integration tests still pass
- [ ] Behavior validated as identical to legacy
- [ ] Error propagation matches legacy patterns
- [ ] Context cancellation behavior preserved
- [ ] Rollback plan tested and documented
- [ ] No regressions in workflow execution
- [ ] Code passes `make lint` and `make test`
