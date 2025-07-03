---
status: pending # Options: pending, in-progress, completed, excluded
---

<task_context>
<domain>engine/task2/signal</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>interfaces,shared</dependencies>
</task_context>

# Task 5.0: Signal Task Orchestrator

## Overview

Implement the signal task orchestrator that sends signals to other tasks. Signal tasks are the counterpart to wait tasks - they dispatch signals that waiting tasks can receive.

## Subtasks

- [ ] 5.1 Create SignalOrchestrator struct with signal dispatch capabilities
- [ ] 5.2 Implement CreateState with signal configuration validation
- [ ] 5.3 Create SignalDispatcher helper for signal sending logic
- [ ] 5.4 Implement PrepareExecution to validate signal targets
- [ ] 5.5 Implement HandleResponse to process signal dispatch results
- [ ] 5.6 Add support for broadcast signals (multiple targets)
- [ ] 5.7 Write unit tests for signal dispatch scenarios
- [ ] 5.8 Write integration tests with wait tasks

## Implementation Details

### Signal Orchestrator (engine/task2/signal/orchestrator.go)

```go
type Orchestrator struct {
    *shared.BaseOrchestrator
    signalDispatcher *SignalDispatcher
}

func (o *Orchestrator) CreateState(ctx context.Context, input interfaces.CreateStateInput) (*task.State, error) {
    state, err := o.BaseOrchestrator.CreateState(ctx, input)
    if err != nil {
        return nil, err
    }

    // Validate signal configuration
    if err := o.validateSignalConfig(input.TaskConfig); err != nil {
        return nil, fmt.Errorf("invalid signal configuration: %w", err)
    }

    return state, nil
}

func (o *Orchestrator) HandleResponse(ctx context.Context, input interfaces.HandleResponseInput) (*task.Response, error) {
    // Dispatch configured signals
    signalConfig := o.getSignalConfig(input.State)
    results, err := o.signalDispatcher.Dispatch(ctx, signalConfig, input.Output)

    if err != nil {
        return nil, fmt.Errorf("signal dispatch failed: %w", err)
    }

    // Include dispatch results in response
    response := &task.Response{
        Status: core.StatusSuccess,
        Output: input.Output,
    }
    (*response.Output)["signal_results"] = results

    return response, nil
}
```

### Key Features

- Signal configuration validation
- Target task resolution
- Payload template processing
- Broadcast signal support
- Dispatch result tracking
- Error handling for failed signals

## Success Criteria

- Signal orchestrator correctly dispatches signals
- Target resolution works for various reference types
- Template processing correctly builds signal payloads
- Broadcast signals reach all intended targets
- Failed signal dispatches are properly handled
- Integration with wait tasks functions correctly
- Comprehensive test coverage

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
