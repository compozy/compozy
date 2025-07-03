---
status: pending # Options: pending, in-progress, completed, excluded
---

<task_context>
<domain>engine/task2/aggregate</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>interfaces,shared</dependencies>
</task_context>

# Task 12.0: Aggregate Task Orchestrator

## Overview

Implement the aggregate task orchestrator that collects and combines outputs from multiple tasks into a single aggregated result. This is useful for gathering results from parallel or distributed operations.

## Subtasks

- [ ] 12.1 Create AggregateOrchestrator with aggregation logic
- [ ] 12.2 Implement output collection from referenced tasks
- [ ] 12.3 Create aggregation strategies (merge, concat, custom)
- [ ] 12.4 Implement task reference resolution
- [ ] 12.5 Add timeout handling for missing outputs
- [ ] 12.6 Create custom aggregation function support
- [ ] 12.7 Write unit tests for aggregation strategies
- [ ] 12.8 Create integration tests with various task types

## Implementation Details

### Aggregate Orchestrator (engine/task2/aggregate/orchestrator.go)

```go
type Orchestrator struct {
    *shared.BaseOrchestrator
    aggregator *OutputAggregator
    resolver   *TaskResolver
}

func (o *Orchestrator) HandleResponse(ctx context.Context, input interfaces.HandleResponseInput) (*task.Response, error) {
    config := o.getAggregateConfig(input.State)

    // Resolve task references to actual task states
    targetStates, err := o.resolver.ResolveReferences(ctx, config.Sources, input.State)
    if err != nil {
        return nil, fmt.Errorf("failed to resolve task references: %w", err)
    }

    // Collect outputs from resolved tasks
    outputs := make([]map[string]interface{}, 0, len(targetStates))
    for _, state := range targetStates {
        if state.Output != nil {
            outputs = append(outputs, *state.Output)
        }
    }

    // Apply aggregation strategy
    aggregated, err := o.aggregator.Aggregate(ctx, outputs, config.Strategy)
    if err != nil {
        return nil, fmt.Errorf("aggregation failed: %w", err)
    }

    response := &task.Response{
        Status: core.StatusSuccess,
        Output: &aggregated,
    }

    return response, nil
}
```

### Aggregation Strategies (engine/task2/aggregate/strategies.go)

```go
type AggregationStrategy interface {
    Aggregate(outputs []map[string]interface{}) (map[string]interface{}, error)
}

type MergeStrategy struct {
    conflictResolution ConflictResolution
}

func (s *MergeStrategy) Aggregate(outputs []map[string]interface{}) (map[string]interface{}, error) {
    result := make(map[string]interface{})

    for _, output := range outputs {
        for key, value := range output {
            if existing, exists := result[key]; exists {
                result[key] = s.conflictResolution.Resolve(existing, value)
            } else {
                result[key] = value
            }
        }
    }

    return result, nil
}

type ConcatStrategy struct {
    outputKey string
}

func (s *ConcatStrategy) Aggregate(outputs []map[string]interface{}) (map[string]interface{}, error) {
    values := make([]interface{}, 0, len(outputs))

    for _, output := range outputs {
        if value, exists := output[s.outputKey]; exists {
            values = append(values, value)
        }
    }

    return map[string]interface{}{
        s.outputKey: values,
    }, nil
}
```

### Key Features

- Multiple aggregation strategies
- Task reference resolution
- Timeout handling for incomplete data
- Custom aggregation functions
- Conflict resolution for merges
- Flexible output structure

## Success Criteria

- All aggregation strategies work correctly
- Task reference resolution handles various formats
- Timeout handling prevents indefinite waiting
- Custom aggregation functions integrate properly
- Conflict resolution works as configured
- Output structure matches expectations
- Integration with different task types works
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
