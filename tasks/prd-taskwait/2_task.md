---
status: pending
---

<task_context>
<domain>engine/task</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>task_config</dependencies>
</task_context>

# Task 2.0: Define Core Interfaces

## Overview

Create interface definitions following Interface Segregation Principle for signal processing, condition evaluation, and storage. This task establishes the contract boundaries that enable dependency injection and testability.

## Subtasks

- [ ] 2.1 Define SignalProcessor interface for signal processing operations
- [ ] 2.2 Define ConditionEvaluator interface for CEL expression evaluation
- [ ] 2.3 Define SignalStorage interface for deduplication and persistence
- [ ] 2.4 Define WaitTaskExecutor interface for task execution
- [ ] 2.5 Create SignalEnvelope and related data structures
- [ ] 2.6 Add proper JSON tags and validation to data structures

## Implementation Details

Define interfaces in separate files following ISP:

```go
type SignalProcessor interface {
    Process(ctx context.Context, signal *SignalEnvelope) (*ProcessorOutput, error)
}

type ConditionEvaluator interface {
    Evaluate(ctx context.Context, expression string, context map[string]any) (bool, error)
}

type SignalStorage interface {
    IsDuplicate(ctx context.Context, signalID string) (bool, error)
    MarkProcessed(ctx context.Context, signalID string) error
    Close() error
}

type WaitTaskExecutor interface {
    Execute(ctx context.Context, config *WaitTaskConfig) (*WaitTaskResult, error)
}
```

Create supporting data structures:

```go
type SignalEnvelope struct {
    Payload  map[string]any  `json:"payload"`     // User-provided data
    Metadata SignalMetadata  `json:"metadata"`    // System-generated
}

type SignalMetadata struct {
    SignalID      string    `json:"signal_id"`        // UUID for deduplication
    ReceivedAtUTC time.Time `json:"received_at_utc"`  // Server timestamp
    WorkflowID    string    `json:"workflow_id"`      // Target workflow
    Source        string    `json:"source"`           // Signal source
}
```

## Success Criteria

- [ ] All interfaces follow single responsibility principle
- [ ] Interfaces are small and focused (ISP compliance)
- [ ] Data structures have proper JSON marshaling support
- [ ] Context is properly propagated in all interface methods
- [ ] Error handling follows established patterns
- [ ] Mock implementations can be easily created for testing

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
