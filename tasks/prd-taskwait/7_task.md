---
status: pending
---

<task_context>
<domain>engine/task/uc</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>dependency_injection</dependencies>
</task_context>

# Task 7.0: Create Wait Task Service Layer

## Overview

Implement service layer with dependency injection for wait task coordination. This component provides the business logic layer that coordinates between the workflow execution and the domain interfaces.

## Subtasks

- [ ] 7.1 Create WaitTaskService struct with injected dependencies
- [ ] 7.2 Implement NewWaitTaskService constructor following DIP
- [ ] 7.3 Implement ExecuteWaitTask method with proper error handling
- [ ] 7.4 Add Close method for resource cleanup
- [ ] 7.5 Implement proper error handling using core.NewError at boundaries
- [ ] 7.6 Add structured logging and observability

## Implementation Details

Implement WaitTaskService with dependency injection:

```go
type WaitTaskService struct {
    executor  WaitTaskExecutor
    evaluator ConditionEvaluator
    storage   SignalStorage
    logger    log.Logger
}

func NewWaitTaskService(
    executor WaitTaskExecutor,
    evaluator ConditionEvaluator,
    storage SignalStorage,
    logger log.Logger,
) *WaitTaskService {
    return &WaitTaskService{
        executor:  executor,
        evaluator: evaluator,
        storage:   storage,
        logger:    logger,
    }
}

func (s *WaitTaskService) ExecuteWaitTask(ctx context.Context, config *WaitTaskConfig) (*WaitTaskResult, error) {
    result, err := s.executor.Execute(ctx, config)
    if err != nil {
        return nil, core.NewError(err, "WAIT_TASK_EXECUTION_FAILED", map[string]any{
            "task_id": config.ID,
            "wait_for": config.WaitFor,
        })
    }
    return result, nil
}

func (s *WaitTaskService) Close() error {
    if err := s.storage.Close(); err != nil {
        s.logger.Error("failed to close signal storage", "error", err)
        return fmt.Errorf("failed to close storage: %w", err)
    }
    return nil
}
```

Key service responsibilities:

- Coordinate wait task execution through executor
- Handle domain-level error wrapping
- Manage resource lifecycle and cleanup
- Provide structured logging and observability
- Abstract implementation details from callers

## Success Criteria

- [ ] Service follows dependency injection patterns (DIP compliance)
- [ ] Dependencies are abstracted through interfaces
- [ ] Error handling uses core.NewError at domain boundaries
- [ ] Resource cleanup properly implemented
- [ ] Structured logging provides good observability
- [ ] Service methods are properly documented
- [ ] Context propagation works correctly

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
