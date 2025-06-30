---
status: pending
---

<task_context>
<domain>engine/task2/shared</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>template_engine|context_builder</dependencies>
</task_context>

# Task 1.0: Shared Interfaces & Components

## Overview

Create foundational interfaces and shared components that will be used across all new task handling components. This establishes the contracts and common functionality needed for the modular architecture.

## Subtasks

- [ ] 1.1 Define core response handler interfaces
- [ ] 1.2 Create shared response data structures
- [ ] 1.3 Implement ParentStatusManager component
- [ ] 1.4 Create ContextBuilder integration helpers
- [ ] 1.5 Define domain service interfaces
- [ ] 1.6 Implement comprehensive unit tests

## Implementation Details

### Files to Create

1. `engine/task2/shared/interfaces.go` - Core interfaces and contracts
2. `engine/task2/shared/response_handler.go` - Common response logic
3. `engine/task2/shared/parent_status_manager.go` - Shared component for status management
4. `engine/task2/shared/types.go` - Common data structures

### Key Interfaces

**TaskResponseHandler Interface:**

```go
type TaskResponseHandler interface {
    HandleResponse(ctx context.Context, input *ResponseInput) (*ResponseOutput, error)
    Type() task.Type
}
```

**CollectionExpander Interface:**

```go
type CollectionExpander interface {
    ExpandItems(ctx context.Context, config *task.Config, workflowState *workflow.State, workflowConfig *workflow.Config) (*ExpansionResult, error)
    ValidateExpansion(result *ExpansionResult) error
}
```

**TaskConfigRepository Interface:**

```go
type TaskConfigRepository interface {
    StoreParallelMetadata(ctx context.Context, parentStateID core.ID, metadata *ParallelTaskMetadata) error
    LoadParallelMetadata(ctx context.Context, parentStateID core.ID) (*ParallelTaskMetadata, error)
}
```

**ParentStatusManager Interface:**

```go
type ParentStatusManager interface {
    UpdateParentStatus(ctx context.Context, parentStateID core.ID, childStatus core.Status) error
    GetAggregatedStatus(ctx context.Context, parentStateID core.ID) (core.Status, error)
}
```

### BaseResponseHandler Logic

Extract common logic from TaskResponder.HandleMainTask:

- Task execution result processing
- State saving and context cancellation handling
- Parent status update coordination
- Transition processing and validation
- Next task determination

## Success Criteria

- All interfaces defined with clear contracts
- ParentStatusManager handles status aggregation correctly
- Context building utilities work with existing task2 components
- > 70% test coverage for all shared components
- Zero breaking changes to existing task2 infrastructure
- Components impacted by this task per Tech Spec Impact Analysis

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

- [ ] All interfaces defined in `engine/task2/shared/interfaces.go`
- [ ] ResponseInput and ResponseOutput structures include all required fields
- [ ] ParentStatusManager interface methods properly defined
- [ ] TaskResponseHandler interface follows clean architecture principles
- [ ] All shared types and constants created
- [ ] Unit tests written with >70% coverage
- [ ] Code passes `make lint` and `make test`
- [ ] No circular dependencies introduced
- [ ] Documentation comments added to all exported types
- [ ] Integration with existing task2 patterns verified
