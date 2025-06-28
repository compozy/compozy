---
status: pending # Options: pending, in-progress, completed, excluded
---

<task_context>
<domain>engine/task2/basic</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>interfaces,shared</dependencies>
</task_context>

# Task 3.0: Basic Task Orchestrator

## Overview

Implement the basic task orchestrator as the simplest task type and proof of concept for the new architecture. Basic tasks have no special features - they simply execute and return results.

## Subtasks

- [ ] 3.1 Create BasicOrchestrator struct embedding BaseOrchestrator
- [ ] 3.2 Implement CreateState for basic task state creation
- [ ] 3.3 Implement PrepareExecution (likely no-op for basic tasks)
- [ ] 3.4 Implement HandleResponse for basic response processing
- [ ] 3.5 Implement GetType to return TaskTypeBasic
- [ ] 3.6 Create constructor function NewOrchestrator
- [ ] 3.7 Write comprehensive unit tests for all methods
- [ ] 3.8 Create integration test with factory

## Implementation Details

### Basic Orchestrator (engine/task2/basic/orchestrator.go)

```go
type Orchestrator struct {
    *shared.BaseOrchestrator
}

func NewOrchestrator(ctx *shared.OrchestratorContext) interfaces.TaskOrchestrator {
    return &Orchestrator{
        BaseOrchestrator: shared.NewBaseOrchestrator(ctx, task.TaskTypeBasic),
    }
}

func (o *Orchestrator) CreateState(ctx context.Context, input interfaces.CreateStateInput) (*task.State, error) {
    // Basic task state creation - no special handling needed
    return o.BaseOrchestrator.CreateState(ctx, input)
}

func (o *Orchestrator) PrepareExecution(ctx context.Context, state *task.State) error {
    // Basic tasks need no preparation
    return nil
}
```

### Key Characteristics

- No child task management
- No signal handling
- No status aggregation
- Simplest possible implementation
- Serves as baseline for other orchestrators

## Success Criteria

- Basic orchestrator implements all required interface methods
- No type-specific logic leaks into basic implementation
- Passes all interface compliance tests
- Successfully integrates with factory
- Demonstrates the pattern for other task types
- Follows project coding standards

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
