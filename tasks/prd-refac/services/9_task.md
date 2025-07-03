---
status: pending # Options: pending, in-progress, completed, excluded
---

<task_context>
<domain>engine/task2/collection</domain>
<type>implementation</type>
<scope>middleware</scope>
<complexity>high</complexity>
<dependencies>interfaces,shared</dependencies>
</task_context>

# Task 9.0: Collection Task Components

## Overview

Implement the supporting components for collection task orchestration, including item expansion, filtering, and child task building. These components enable dynamic iteration over collections.

## Subtasks

- [ ] 9.1 Create ItemExpander for collection item expansion
- [ ] 9.2 Implement ItemFilter with expression evaluation
- [ ] 9.3 Create ChildBuilder for dynamic child task creation
- [ ] 9.4 Implement collection metadata tracking
- [ ] 9.5 Create batch processing support
- [ ] 9.6 Implement collection validation logic
- [ ] 9.7 Write unit tests for each component
- [ ] 9.8 Create test fixtures for various collection types

## Implementation Details

### Item Expander (engine/task2/collection/item_expander.go)

```go
type ItemExpander struct {
    templateEngine *tplengine.TemplateEngine
}

func (e *ItemExpander) ExpandItems(ctx context.Context, config *task.CollectionConfig, state *task.State) ([]interface{}, error) {
    // Handle different sources
    switch {
    case config.Items != nil:
        return config.Items, nil
    case config.ItemsFrom != "":
        return e.expandFromPath(ctx, config.ItemsFrom, state)
    case config.ItemsFromExpression != "":
        return e.expandFromExpression(ctx, config.ItemsFromExpression, state)
    default:
        return nil, fmt.Errorf("no collection source specified")
    }
}

func (e *ItemExpander) expandFromPath(ctx context.Context, path string, state *task.State) ([]interface{}, error) {
    // Extract items from state output path
    value, err := extractPath(state.Output, path)
    if err != nil {
        return nil, fmt.Errorf("failed to extract items from path %s: %w", path, err)
    }

    // Convert to slice
    return toSlice(value)
}
```

### Item Filter (engine/task2/collection/item_filter.go)

```go
type ItemFilter struct {
    expressionEngine *expression.Engine
}

func (f *ItemFilter) FilterItems(ctx context.Context, items []interface{}, config *task.CollectionConfig) ([]interface{}, error) {
    if config.Filter == "" {
        return items, nil // No filter
    }

    filtered := make([]interface{}, 0, len(items))
    for i, item := range items {
        context := map[string]interface{}{
            "item":  item,
            "index": i,
        }

        match, err := f.expressionEngine.Evaluate(config.Filter, context)
        if err != nil {
            return nil, fmt.Errorf("filter evaluation failed for item %d: %w", i, err)
        }

        if match.(bool) {
            filtered = append(filtered, item)
        }
    }

    return filtered, nil
}
```

### Key Components

- **ItemExpander**: Expands collection from various sources
- **ItemFilter**: Filters items based on expressions
- **ChildBuilder**: Builds child task configs from items
- **BatchProcessor**: Handles batch size configuration
- **CollectionValidator**: Validates collection configuration

## Success Criteria

- Item expansion works for all supported sources
- Filter expressions correctly evaluate for each item
- Child task configs properly built with item context
- Batch processing respects configured limits
- Metadata tracking captures collection details
- Components are independently testable
- High test coverage with edge cases

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
