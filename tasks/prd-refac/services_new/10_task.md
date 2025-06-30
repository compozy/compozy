---
status: pending
---

<task_context>
<domain>engine/task/services</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>activities_integration,legacy_services,build_system</dependencies>
</task_context>

# Task 10.0: Legacy Service Removal

## Overview

Remove the legacy TaskResponder and ConfigManager services from the codebase after successful integration of the new modular components. This is the final cleanup phase that eliminates the old monolithic services and completes the refactoring process.

## Subtasks

- [ ] 10.1 TaskResponder service completely removed from codebase
- [ ] 10.2 ConfigManager service completely removed from codebase
- [ ] 10.3 All references and imports updated to use new components
- [ ] 10.4 Build succeeds without any compilation errors
- [ ] 10.5 All tests pass with new component integration
- [ ] 10.6 Code size reduction of 1,225 LOC achieved
- [ ] 10.7 Documentation updated to reflect new architecture

## Implementation Details

### Files to Remove

**Primary Service Files:**

1. `engine/task/services/task_responder.go` (732 LOC)
2. `engine/task/services/config_manager.go` (493 LOC)
3. `engine/task/services/task_responder_test.go`
4. `engine/task/services/config_manager_test.go`

**Legacy Integration Points:**

1. Remove from `engine/worker/activities.go` constructor
2. Remove from dependency injection configuration
3. Update imports in all activity implementations

### Activities.go Cleanup

**Remove Legacy Dependencies:**

```go
// BEFORE: Activities struct with legacy services
type Activities struct {
    // ... other fields ...
    task2Factory     task2.ExtendedFactory
    configManager    *services.ConfigManager  // REMOVE
    taskResponder    *services.TaskResponder  // REMOVE
}

// AFTER: Clean Activities struct
type Activities struct {
    // ... other fields ...
    task2Factory     task2.ExtendedFactory
    // Legacy services removed
}
```

**Update Constructor:**

```go
// BEFORE: Constructor with legacy services
func NewActivities(...) *Activities {
    configManager, err := services.NewConfigManager(configStore, cwd)
    taskResponder := services.NewTaskResponder(...)

    return &Activities{
        // ... existing fields ...
        task2Factory:     task2Factory,
        configManager:    configManager,  // REMOVE
        taskResponder:    taskResponder,  // REMOVE
    }
}

// AFTER: Clean constructor
func NewActivities(...) *Activities {
    return &Activities{
        // ... existing fields ...
        task2Factory:     task2Factory,
        // Legacy services removed
    }
}
```

### Import Cleanup

**Remove Legacy Imports:**

```go
// Remove these imports from all files:
import "path/to/engine/task/services"

// Update to use only new components:
import "path/to/engine/task2/shared"
import "path/to/engine/task2/collection"
// ... other new component imports
```

### Activity Implementation Cleanup

**Remove Legacy Method Calls:**

```go
// BEFORE: Activity using legacy services
func (a *Activities) CreateCollectionState(...) (*task.State, error) {
    act, err := tkfacts.NewCreateCollectionState(
        a.workflows,
        a.workflowRepo,
        a.taskRepo,
        a.configStore,
        a.projectConfig.CWD,
    )
    // Legacy: act uses ConfigManager internally
}

// AFTER: Clean activity implementation (already updated in Task 9)
func (a *Activities) CreateCollectionState(...) (*task.State, error) {
    expander := a.task2Factory.CreateCollectionExpander()
    repository := a.task2Factory.CreateTaskConfigRepository(a.configStore)

    act, err := tkfacts.NewCreateCollectionState(
        a.workflows,
        a.workflowRepo,
        a.taskRepo,
        expander,
        repository,
        a.projectConfig.CWD,
    )
}
```

### Individual Activity File Updates

**Files to Update:**

1. `engine/task/activities/create_collection_state.go`
2. `engine/task/activities/get_collection_response.go`
3. `engine/task/activities/create_parallel_state.go`
4. `engine/task/activities/get_parallel_response.go`
5. `engine/task/activities/execute_basic_task.go`
6. `engine/task/activities/create_composite_state.go`
7. `engine/task/activities/get_composite_response.go`
8. `engine/task/activities/execute_router_task.go`
9. `engine/task/activities/execute_signal_task.go`
10. `engine/task/activities/execute_wait_task.go`
11. `engine/task/activities/execute_aggregate_task.go`

**Remove Legacy Constructor Parameters:**

```go
// BEFORE: Activity constructor with legacy service
func NewCreateCollectionState(
    workflows []*workflow.Config,
    workflowRepo workflow.Repository,
    taskRepo task.Repository,
    configStore services.ConfigStore,
    cwd *core.PathCWD,
) (*CreateCollectionState, error) {
    configManager, err := services.NewConfigManager(configStore, cwd)  // REMOVE
    if err != nil {
        return nil, err
    }
    // ... rest of constructor
}

// AFTER: Activity constructor with new components
func NewCreateCollectionState(
    workflows []*workflow.Config,
    workflowRepo workflow.Repository,
    taskRepo task.Repository,
    expander collection.CollectionExpander,
    repository services.TaskConfigRepository,
    cwd *core.PathCWD,
) (*CreateCollectionState, error) {
    return &CreateCollectionState{
        workflows:    workflows,
        workflowRepo: workflowRepo,
        taskRepo:     taskRepo,
        expander:     expander,
        repository:   repository,
        cwd:          cwd,
    }, nil
}
```

### Services Package Cleanup

**Update services/package structure:**

```go
// BEFORE: services package with legacy services
package services

type ConfigManager struct { ... }  // REMOVE
type TaskResponder struct { ... }  // REMOVE
type TaskConfigRepository struct { ... }  // KEEP - new infrastructure service

// AFTER: Clean services package
package services

type TaskConfigRepository struct { ... }  // Infrastructure service only
```

### Dependency Graph Updates

**Remove Legacy Dependencies:**

1. Remove ConfigManager dependencies on workflow/task repositories
2. Remove TaskResponder dependencies on all components
3. Update dependency injection configuration
4. Clean up any circular dependencies

### Test File Cleanup

**Remove Legacy Test Files:**

```bash
rm engine/task/services/task_responder_test.go
rm engine/task/services/config_manager_test.go
```

**Update Integration Tests:**

- Remove all references to legacy services in test setup
- Update mocking to use new components only
- Clean up test fixtures that depend on legacy services

### Build Validation

**Compilation Checks:**

```bash
# Ensure build succeeds after removal
make build

# Run all tests to ensure no regressions
make test

# Check for any remaining references
grep -r "TaskResponder" . --exclude-dir=.git
grep -r "ConfigManager" . --exclude-dir=.git
```

### Documentation Updates

**Update Architecture Documentation:**

1. Remove TaskResponder and ConfigManager from architecture diagrams
2. Update component interaction documentation
3. Revise API documentation to reflect new patterns
4. Update developer guide with new component usage

**Update Code Comments:**

- Remove TODO comments about legacy service removal
- Update package documentation
- Add migration notes if needed

## Rollback Strategy

**Emergency Rollback Plan:**

1. Git revert commits if critical issues discovered
2. Temporary re-introduction of legacy services if needed
3. Feature flag to switch back to legacy behavior
4. Monitoring for any behavior regressions

**Validation Checklist Before Removal:**

- [ ] All integration tests passing with new components
- [ ] Behavior validation tests passing
- [ ] Production deployment successful with new components
- [ ] No critical issues reported in monitoring
- [ ] User acceptance testing completed

## Dependencies

- Task 9: Activities.go integration completed and validated
- All new components fully functional and tested
- Production deployment with new components successful
- Behavior validation completed

## Validation Requirements

**Pre-Removal Validation:**

- [ ] All workflows function identically with new components
- [ ] Behavior equivalent to legacy implementation
- [ ] Error handling behavior preserved
- [ ] Integration tests passing consistently
- [ ] Production stability confirmed

**Post-Removal Validation:**

- [ ] Build succeeds without compilation errors
- [ ] All tests pass with new component integration
- [ ] No runtime errors or panics
- [ ] Memory usage within expected bounds
- [ ] Application starts and functions normally

## Code Size Reduction Metrics

**Expected Reduction:**

- TaskResponder: 732 LOC removed
- ConfigManager: 493 LOC removed
- Total: 1,225 LOC eliminated
- Net reduction after new components: ~800-900 LOC

**Quality Improvements:**

- Reduced cyclomatic complexity
- Better separation of concerns
- Improved testability
- Enhanced maintainability

## Implementation Notes

- Perform removal incrementally (one service at a time)
- Test thoroughly after each removal step
- Keep git history for potential rollback
- Document any behavioral changes
- Monitor application metrics post-removal

## Safety Measures

- Create backup branch before removal
- Deploy removal during low-traffic period
- Have rollback plan ready
- Monitor error rates and behavior consistency
- Coordinate with team for immediate response

## Success Criteria

- TaskResponder and ConfigManager completely removed
- All compilation errors resolved
- All tests passing with new components
- Build and deployment successful
- Behavior validated as equivalent
- Documentation updated
- Code review approved
- 1,225 LOC reduction achieved
- Progressive refactoring complete
- Codebase fully migrated to new modular architecture

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

- [ ] TaskResponder service completely removed from codebase
- [ ] ConfigManager service completely removed from codebase
- [ ] All import statements updated to remove legacy services
- [ ] All tests updated to use new components
- [ ] Build completes successfully with no compilation errors
- [ ] All tests pass with new architecture
- [ ] Documentation updated to reflect new architecture
- [ ] 1,225 LOC reduction verified
- [ ] No orphaned code or imports remaining
- [ ] Final code review completed and approved
