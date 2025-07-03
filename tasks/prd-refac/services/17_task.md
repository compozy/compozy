---
status: pending # Options: pending, in-progress, completed, excluded
---

<task_context>
<domain>engine/task/services</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>all_orchestrators,migration_complete</dependencies>
</task_context>

# Task 17.0: Old Services Removal

## Overview

Remove the old service implementations after confirming all functionality has been successfully migrated to the orchestrator-based architecture. This includes removing type-specific methods, switch statements, and obsolete service files.

## Subtasks

- [ ] 17.1 Verify all task types migrated and tested
- [ ] 17.2 Remove ConfigManager and type-specific methods
- [ ] 17.3 Remove CreateChildTasks use case
- [ ] 17.4 Remove ParentStatusUpdater service
- [ ] 17.5 Remove SignalDispatcher service
- [ ] 17.6 Remove WaitTaskManager service
- [ ] 17.7 Clean up unused dependencies and imports
- [ ] 17.8 Run full test suite to ensure no regressions

## Implementation Details

### Pre-Removal Checklist

```go
// Verify migration completion
func VerifyMigrationComplete() error {
    metrics := migrationAdapter.GetMigrationStatus()

    for taskType, status := range metrics.TaskTypes {
        if status.OldPathCount > 0 {
            return fmt.Errorf("task type %s still using old path: %d calls", taskType, status.OldPathCount)
        }
        if status.NewPathCount == 0 {
            return fmt.Errorf("task type %s has no new path usage", taskType)
        }
    }

    return nil
}
```

### Services to Remove

#### 1. ConfigManager (engine/task/services/config_manager.go)

```go
// REMOVE ENTIRE FILE
// Methods migrated to orchestrators:
// - PrepareParallelConfigs -> parallel.Orchestrator.PrepareChildren
// - PrepareCollectionConfigs -> collection.Orchestrator.PrepareChildren
// - PrepareCompositeConfigs -> composite.Orchestrator.PrepareChildren
```

#### 2. CreateChildTasks (engine/task/uc/create_child.go)

```go
// REMOVE ENTIRE FILE
// Logic migrated to:
// - parallel.Orchestrator.CreateChildren
// - collection.Orchestrator.CreateChildren
// - composite.Orchestrator.CreateChildren
```

#### 3. ParentStatusUpdater (engine/task/services/parent_status_updater.go)

```go
// REMOVE ENTIRE FILE
// Logic migrated to StatusAggregator interface implementations
```

#### 4. SignalDispatcher (engine/task/services/signal_dispatcher.go)

```go
// REMOVE ENTIRE FILE
// Logic migrated to signal.Orchestrator
```

#### 5. WaitTaskManager (engine/task/services/wait_task_manager.go)

```go
// REMOVE ENTIRE FILE
// Logic migrated to wait.Orchestrator with SignalHandler interface
```

### Dependency Cleanup

```go
// Update engine/task/services/services.go
type Services struct {
    // REMOVE: ConfigManager      *ConfigManager
    // REMOVE: ParentStatusUpdater *ParentStatusUpdater
    // REMOVE: SignalDispatcher    *SignalDispatcher
    // REMOVE: WaitTaskManager     *WaitTaskManager

    // Keep only non-migrated services
    TaskRepo       task.Repository
    WorkflowRepo   workflow.Repository
    // ... other services
}
```

### Import Cleanup Script

```bash
#!/bin/bash
# Find and remove unused imports after service removal

# Remove references to deleted services
find . -name "*.go" -type f -exec sed -i '' \
    -e '/ConfigManager/d' \
    -e '/CreateChildTasks/d' \
    -e '/ParentStatusUpdater/d' \
    -e '/SignalDispatcher/d' \
    -e '/WaitTaskManager/d' {} \;

# Run goimports to clean up
goimports -w .
```

### Post-Removal Validation

1. Compile the entire project
2. Run all unit tests
3. Run integration tests
4. Run E2E tests
5. Check for any remaining references

## Success Criteria

- All old services successfully removed
- No compilation errors after removal
- All tests continue to pass
- No references to removed services remain
- Code coverage maintained or improved
- Documentation updated to reflect removal
- Migration adapter can be simplified
- Clean git history with atomic removal commit

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
