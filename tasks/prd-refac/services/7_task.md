---
status: pending # Options: pending, in-progress, completed, excluded
---

<task_context>
<domain>engine/task2/parallel</domain>
<type>implementation</type>
<scope>middleware</scope>
<complexity>high</complexity>
<dependencies>interfaces,shared</dependencies>
</task_context>

# Task 7.0: Parallel Task Components

## Overview

Implement the supporting components for parallel task orchestration, including child preparation, status calculation, and execution strategies. These components form the foundation for the parallel orchestrator.

## Subtasks

- [ ] 7.1 Create ChildPreparer for parallel child task preparation
- [ ] 7.2 Implement StatusCalculator with strategy pattern
- [ ] 7.3 Create parallel execution strategies (all, any, threshold)
- [ ] 7.4 Implement child task metadata storage
- [ ] 7.5 Create parallel configuration validator
- [ ] 7.6 Implement max workers limitation logic
- [ ] 7.7 Write unit tests for each component
- [ ] 7.8 Create test fixtures for parallel scenarios

## Implementation Details

### Child Preparer (engine/task2/parallel/child_preparer.go)

```go
type ChildPreparer struct {
    storage shared.Storage
}

func (p *ChildPreparer) PrepareChildren(ctx context.Context, parent *task.State, config *task.Config) error {
    children := make([]ChildConfig, len(config.Tasks))
    for i, taskCfg := range config.Tasks {
        children[i] = ChildConfig{
            Config:   taskCfg,
            Index:    i,
            ParentID: parent.TaskExecID,
        }
    }

    metadata := ChildrenMetadata{
        Count:      len(children),
        Strategy:   string(config.ParallelConfig.Strategy),
        MaxWorkers: config.ParallelConfig.MaxWorkers,
    }

    return p.storage.Store(ctx, p.childrenKey(parent.TaskExecID), children)
}
```

### Status Calculator (engine/task2/parallel/status_calculator.go)

```go
type StatusCalculator struct {
    strategies map[task.ParallelStrategy]StatusStrategy
}

type StatusStrategy interface {
    Calculate(children []*task.State) core.StatusType
}

type AllStrategy struct{}

func (s *AllStrategy) Calculate(children []*task.State) core.StatusType {
    for _, child := range children {
        if child.Status == core.StatusFailed {
            return core.StatusFailed
        }
        if child.Status != core.StatusSuccess {
            return core.StatusRunning
        }
    }
    return core.StatusSuccess
}
```

### Key Components

- **ChildPreparer**: Prepares child configurations for execution
- **StatusCalculator**: Calculates parent status based on strategy
- **Strategies**: All, Any, Threshold implementations
- **ConfigValidator**: Validates parallel configuration
- **MetadataStore**: Stores child preparation metadata

## Success Criteria

- All components properly implement their interfaces
- Strategy pattern correctly implemented for status calculation
- Child preparation handles all edge cases
- Max workers limitation enforced
- Metadata storage and retrieval works correctly
- Components are independently testable
- High test coverage with various scenarios

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
