---
status: pending # Options: pending, in-progress, completed, excluded
---

<task_context>
<domain>engine/task2/composite</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>interfaces,shared</dependencies>
</task_context>

# Task 11.0: Composite Task Orchestrator

## Overview

Implement the composite task orchestrator that manages sequential execution of child tasks where each child depends on the previous one's output. This creates a pipeline of tasks with data flow between them.

## Subtasks

- [ ] 11.1 Create CompositeOrchestrator implementing ChildTaskManager
- [ ] 11.2 Implement sequential child preparation logic
- [ ] 11.3 Create output propagation mechanism
- [ ] 11.4 Implement state tracking for sequential execution
- [ ] 11.5 Add error handling and rollback support
- [ ] 11.6 Implement GetChildrenMetadata for composite info
- [ ] 11.7 Write unit tests for sequential execution
- [ ] 11.8 Create integration tests with data flow

## Implementation Details

### Composite Orchestrator (engine/task2/composite/orchestrator.go)

```go
type Orchestrator struct {
    *shared.BaseOrchestrator
    sequenceManager *SequenceManager
}

func (o *Orchestrator) PrepareChildren(ctx context.Context, parent *task.State, config *task.Config) error {
    // Composite tasks prepare children sequentially
    sequence := make([]SequenceStep, len(config.Tasks))

    for i, taskConfig := range config.Tasks {
        sequence[i] = SequenceStep{
            Index:      i,
            Config:     taskConfig,
            DependsOn:  i - 1, // Each depends on previous
            ParentID:   parent.TaskExecID,
        }
    }

    metadata := CompositeMetadata{
        StepCount:    len(sequence),
        CurrentStep:  0,
        ErrorOnFail:  config.CompositeConfig.ErrorOnFail,
    }

    return o.sequenceManager.StoreSequence(ctx, parent.TaskExecID, sequence, metadata)
}

func (o *Orchestrator) CreateChildren(ctx context.Context, parentID core.ID) ([]*task.State, error) {
    metadata, err := o.GetChildrenMetadata(ctx, parentID)
    if err != nil {
        return nil, err
    }

    currentStep := metadata.CustomFields["currentStep"].(int)
    sequence, err := o.sequenceManager.LoadSequence(ctx, parentID)
    if err != nil {
        return nil, err
    }

    // Only create the next child in sequence
    if currentStep >= len(sequence) {
        return nil, nil // Sequence complete
    }

    step := sequence[currentStep]

    // Get previous child's output if not first step
    var previousOutput *core.Output
    if currentStep > 0 {
        previousState, err := o.getPreviousChildState(ctx, parentID, currentStep-1)
        if err != nil {
            return nil, err
        }
        previousOutput = previousState.Output
    }

    // Create child with injected previous output
    childState, err := o.createChildWithContext(ctx, parentID, step, previousOutput)
    if err != nil {
        return nil, err
    }

    return []*task.State{childState}, nil
}
```

### Key Features

- Sequential child execution
- Output propagation between steps
- State tracking for current step
- Error handling configuration
- Rollback support on failure
- Previous output injection

## Success Criteria

- Children execute strictly sequentially
- Each child receives previous child's output
- Current step tracking works correctly
- Error handling respects configuration
- State persistence handles restarts
- Integration with workflow engine works
- Data flow between tasks validated
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
