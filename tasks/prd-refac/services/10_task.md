---
status: pending # Options: pending, in-progress, completed, excluded
---

<task_context>
<domain>engine/task2/collection</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>interfaces,shared,collection_components</dependencies>
</task_context>

# Task 10.0: Collection Task Orchestrator

## Overview

Implement the collection task orchestrator that iterates over dynamic collections, creating child tasks for each item. This orchestrator uses the components from Task 9.0 to provide complete collection task functionality.

## Subtasks

- [ ] 10.1 Create CollectionOrchestrator implementing ChildTaskManager
- [ ] 10.2 Implement PrepareChildren with item expansion and filtering
- [ ] 10.3 Implement CreateChildren for dynamic child creation
- [ ] 10.4 Implement batch-aware child creation
- [ ] 10.5 Add collection-specific status aggregation
- [ ] 10.6 Implement GetChildrenMetadata with collection info
- [ ] 10.7 Write comprehensive unit tests
- [ ] 10.8 Create integration tests with various collection sources

## Implementation Details

### Collection Orchestrator (engine/task2/collection/orchestrator.go)

```go
type Orchestrator struct {
    *shared.BaseOrchestrator
    itemExpander *ItemExpander
    itemFilter   *ItemFilter
    childBuilder *ChildBuilder
}

func (o *Orchestrator) PrepareChildren(ctx context.Context, parent *task.State, config *task.Config) error {
    // Expand items from source
    items, err := o.itemExpander.ExpandItems(ctx, config.CollectionConfig, parent)
    if err != nil {
        return fmt.Errorf("failed to expand collection items: %w", err)
    }

    // Apply filter if configured
    filtered, err := o.itemFilter.FilterItems(ctx, items, config.CollectionConfig)
    if err != nil {
        return fmt.Errorf("failed to filter items: %w", err)
    }

    // Build child configs from filtered items
    children, err := o.childBuilder.BuildChildren(ctx, filtered, config, parent)
    if err != nil {
        return fmt.Errorf("failed to build child configs: %w", err)
    }

    // Store with collection metadata
    metadata := CollectionMetadata{
        OriginalCount: len(items),
        FilteredCount: len(filtered),
        BatchSize:     config.CollectionConfig.BatchSize,
        ItemSource:    o.getItemSource(config.CollectionConfig),
    }

    return o.Storage.Store(ctx, o.childrenKey(parent.TaskExecID), children)
}

func (o *Orchestrator) CreateChildren(ctx context.Context, parentID core.ID) ([]*task.State, error) {
    children, err := o.loadPreparedChildren(ctx, parentID)
    if err != nil {
        return nil, err
    }

    metadata, err := o.GetChildrenMetadata(ctx, parentID)
    if err != nil {
        return nil, err
    }

    // Create children respecting batch size
    batchSize := metadata.CustomFields["batchSize"].(int)
    if batchSize <= 0 {
        batchSize = len(children) // No batching
    }

    states := make([]*task.State, 0, min(batchSize, len(children)))
    for i := 0; i < len(children) && i < batchSize; i++ {
        state, err := o.createChildState(ctx, parentID, children[i])
        if err != nil {
            return nil, fmt.Errorf("failed to create child %d: %w", i, err)
        }
        states = append(states, state)
    }

    return states, nil
}
```

### Key Implementation Points

- Dynamic item expansion from various sources
- Expression-based filtering
- Template-based child task configuration
- Batch processing support
- Item context injection into child tasks
- Proper metadata tracking

## Success Criteria

- Collection orchestrator properly expands and filters items
- Child tasks correctly created for each item
- Item context properly injected into child configs
- Batch processing works as configured
- Status aggregation handles dynamic child count
- Integration with various collection sources works
- Performance scales with large collections
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
