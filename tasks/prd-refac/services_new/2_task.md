---
status: completed
---

<task_context>
<domain>engine/task2/collection</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>template_engine|task_repo|workflow_repo</dependencies>
</task_context>

# Task 2.0: CollectionExpander Domain Service

## Overview

Implement the CollectionExpander domain service that encapsulates the complex business logic for collection item expansion, filtering, and child config generation. This replaces the domain logic currently in ConfigManager.PrepareCollectionConfigs.

## Subtasks

- [x] 2.1 Extract collection expansion logic from ConfigManager
- [x] 2.2 Implement CollectionExpander interface
- [x] 2.3 Create collection item processing and filtering
- [x] 2.4 Implement template processing for collection items
- [x] 2.5 Add comprehensive error handling
- [x] 2.6 Collection context injection to child configs
- [x] 2.7 Custom variable naming support
- [x] 2.8 Create comprehensive unit tests

## Implementation Details

### Files to Create

1. `engine/task2/collection/expander.go` - Main implementation
2. `engine/task2/collection/expander_test.go` - Comprehensive tests

### Core Logic to Extract

**From ConfigManager.PrepareCollectionConfigs (lines 150-210):**

```go
// Collection item processing
templateContext := cm.contextBuilder.BuildCollectionContext(workflowState, workflowConfig, taskConfig)
filteredItems, skippedCount, err := cm.processCollectionItems(ctx, taskConfig, templateContext)

// Child config creation
childConfigs, err := cm.createChildConfigs(taskConfig, filteredItems, templateContext)

// Metadata creation
metadata := &services.CollectionMetadata{
    ItemCount:    len(filteredItems),
    SkippedCount: skippedCount,
    ChildStates:  make([]*task.State, 0, len(childConfigs)),
}
```

### Domain Service Implementation

**CollectionExpander Interface Implementation:**

```go
type Expander struct {
    normalizer     collection.Normalizer
    contextBuilder *shared.ContextBuilder
    configBuilder  *collection.ConfigBuilder
}

func (e *Expander) ExpandItems(ctx context.Context, config *task.Config, workflowState *workflow.State, workflowConfig *workflow.Config) (*ExpansionResult, error) {
    // Extract complex collection expansion logic
    // Build context, process items, create child configs
    // Return structured result with metadata
}
```

### Domain Logic Components

- Collection item processing and filtering
- Template context building for collections
- Child config generation with proper templates
- **Collection context injection** to child configs
- **Custom variable naming** support (\_collection_item_var)
- Metadata creation and validation
- Error handling for expansion failures

### Collection Context Injection Implementation

**Context Injection to Child Configs:**

```go
func (e *Expander) createChildConfigs(taskConfig *task.Config, filteredItems []interface{}, templateContext *shared.TemplateContext) ([]*task.Config, error) {
    childConfigs := make([]*task.Config, len(filteredItems))

    for i, item := range filteredItems {
        // Create child config from template
        childConfig, err := e.configBuilder.BuildFromTemplate(taskConfig.ChildTemplate, templateContext)
        if err != nil {
            return nil, fmt.Errorf("failed to build child config at index %d: %w", i, err)
        }

        // CRITICAL: Inject collection context metadata into child config
        if childConfig.With == nil {
            childConfig.With = &core.Input{}
        }

        // Standard collection variables
        (*childConfig.With)["_collection_item"] = item
        (*childConfig.With)["_collection_index"] = i

        // Custom variable naming support
        if itemVar := taskConfig.GetItemVar(); itemVar != "" {
            (*childConfig.With)["_collection_item_var"] = itemVar
        }
        if indexVar := taskConfig.GetIndexVar(); indexVar != "" {
            (*childConfig.With)["_collection_index_var"] = indexVar
        }

        childConfigs[i] = childConfig
    }

    return childConfigs, nil
}
```

## Success Criteria

- CollectionExpander successfully extracts all collection logic from ConfigManager
- Template processing works correctly for collection items
- **Collection context injection** properly implemented for child configs
- **Custom variable naming** support (\_collection_item_var, \_collection_index_var) working
- All edge cases handled (empty collections, template errors, missing variables)
- > 70% test coverage including edge cases and context injection
- Components impacted by this task per Tech Spec Impact Analysis

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

## Validation Checklist

Before marking this task complete, verify:

- [ ] CollectionExpander interface fully implemented
- [ ] ExpandItems extracts all logic from ConfigManager.PrepareCollectionConfigs
- [ ] Collection context variables properly injected (\_collection_item, \_collection_index)
- [ ] Custom variable naming support implemented (\_collection_item_var)
- [ ] ValidateExpansion method checks all business rules
- [ ] Empty collection handling works correctly
- [ ] Template processing errors handled gracefully
- [ ] Integration with existing collection normalizer verified
- [ ] Unit tests cover all edge cases with >70% coverage
- [ ] Code passes `make lint` and `make test`
