---
status: pending # Options: pending, in-progress, completed, excluded
---

<task_context>
<domain>engine/task2/wait</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>interfaces,shared</dependencies>
</task_context>

# Task 4.0: Wait Task Orchestrator

## Overview

Implement the wait task orchestrator that handles signal-based task execution. Wait tasks pause execution until they receive a specific signal, then complete with the signal data as output.

## Subtasks

- [ ] 4.1 Create WaitOrchestrator struct implementing SignalHandler interface
- [ ] 4.2 Implement CreateState with wait-specific configuration
- [ ] 4.3 Implement ValidateSignal to check signal name and correlation
- [ ] 4.4 Implement ProcessSignal to update state with signal data
- [ ] 4.5 Create SignalValidator helper for signal validation logic
- [ ] 4.6 Implement HandleResponse for wait task completion
- [ ] 4.7 Write unit tests for signal validation scenarios
- [ ] 4.8 Write integration tests with signal processing

## Implementation Details

### Wait Orchestrator (engine/task2/wait/orchestrator.go)

```go
type Orchestrator struct {
    *shared.BaseOrchestrator
    signalValidator *SignalValidator
}

func (o *Orchestrator) ValidateSignal(ctx context.Context, state *task.State, signal interfaces.Signal) error {
    config, err := o.LoadTaskConfig(ctx, state.TaskExecID)
    if err != nil {
        return err
    }

    if config.WaitFor != signal.Name {
        return fmt.Errorf("task waiting for '%s', got '%s'", config.WaitFor, signal.Name)
    }

    return o.signalValidator.Validate(signal, config.WaitConfig)
}

func (o *Orchestrator) ProcessSignal(ctx context.Context, state *task.State, signal interfaces.Signal) (*task.State, error) {
    state.Status = core.StatusSuccess
    if state.Output == nil {
        state.Output = &core.Output{}
    }
    (*state.Output)["signal"] = signal.Payload
    (*state.Output)["signal_received_at"] = signal.Timestamp

    return state, o.TaskRepo.UpdateState(ctx, state)
}
```

### Key Features

- Implements SignalHandler interface
- Validates incoming signals against configuration
- Updates task state with signal data
- Handles correlation IDs for signal matching
- Supports timeout configuration

## Success Criteria

- Wait orchestrator properly implements SignalHandler interface
- Signal validation works correctly for all scenarios
- State updates correctly with signal data
- Correlation ID matching functions properly
- Timeout handling is implemented
- Comprehensive test coverage for signal scenarios
- Integration with existing signal infrastructure

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
