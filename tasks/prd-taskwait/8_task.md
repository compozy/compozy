---
status: pending
---

<task_context>
<domain>engine/task</domain>
<type>implementation</type>
<scope>configuration</scope>
<complexity>medium</complexity>
<dependencies>task_registry</dependencies>
</task_context>

# Task 8.0: Implement Task Factory and Registration

## Overview

Create factory pattern for wait task creation and registration with task registry. This component provides the dependency injection setup and task type registration following established Compozy patterns.

## Subtasks

- [ ] 8.1 Create WaitTaskFactory struct with dependency management
- [ ] 8.2 Implement NewWaitTaskFactory constructor
- [ ] 8.3 Implement CreateWaitTask method for task instantiation
- [ ] 8.4 Implement CreateWaitTaskService method for service creation
- [ ] 8.5 Create RegisterWaitTask function for registry integration
- [ ] 8.6 Avoid init() patterns, use constructor-based dependency injection

## Implementation Details

Implement WaitTaskFactory with dependency injection:

```go
type WaitTaskFactory struct {
    celEvaluator    ConditionEvaluator
    signalStorage   SignalStorage
    signalProcessor SignalProcessor
    logger          log.Logger
}

func NewWaitTaskFactory(
    celEvaluator ConditionEvaluator,
    signalStorage SignalStorage,
    signalProcessor SignalProcessor,
    logger log.Logger,
) *WaitTaskFactory {
    return &WaitTaskFactory{
        celEvaluator:    celEvaluator,
        signalStorage:   signalStorage,
        signalProcessor: signalProcessor,
        logger:          logger,
    }
}

func (f *WaitTaskFactory) CreateWaitTask() TaskConfig {
    return &WaitTaskConfig{}
}

func (f *WaitTaskFactory) CreateWaitTaskService() *WaitTaskService {
    executor := NewWaitTaskWorkflow()
    return NewWaitTaskService(executor, f.celEvaluator, f.signalStorage, f.logger)
}

func RegisterWaitTask(registry TaskRegistry, factory *WaitTaskFactory) {
    registry.RegisterTaskType("wait", factory.CreateWaitTask)
}
```

Key factory responsibilities:

- Manage dependency lifecycle and injection
- Provide task instance creation
- Enable service creation with proper wiring
- Support task registry integration
- Follow established factory patterns

## Success Criteria

- [ ] Factory follows established dependency injection patterns
- [ ] Dependencies are properly managed and injected
- [ ] Task creation works correctly with registry
- [ ] Service creation properly wires all dependencies
- [ ] No init() patterns are used (constructor-based only)
- [ ] Factory can be easily tested with mock dependencies
- [ ] Registration integrates with existing task registry

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
