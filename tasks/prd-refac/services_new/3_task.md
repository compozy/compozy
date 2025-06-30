---
status: pending
---

<task_context>
<domain>engine/task2/core</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>config_store,json_marshaling,existing_metadata_types</dependencies>
</task_context>

# Task 3.0: TaskConfigRepository Infrastructure Service

## Overview

Implement the TaskConfigRepository infrastructure service that handles simple CRUD operations for task metadata storage. This replaces the storage logic from ConfigManager's PrepareParallelConfigs, PrepareCompositeConfigs, and related load methods.

## Subtasks

- [ ] 3.1 TaskConfigRepository interface fully implemented
- [ ] 3.2 All metadata storage/retrieval logic extracted from ConfigManager
- [ ] 3.3 Parallel, composite, and collection metadata handling
- [ ] 3.4 Generic task config save/get/delete operations
- [ ] 3.5 JSON marshaling/unmarshaling preserved exactly
- [ ] 3.6 Error handling with proper context
- [ ] 3.7 Strategy extraction and validation logic
- [ ] 3.8 CWD propagation to child configs
- [ ] 3.9 >70% test coverage including error scenarios

## Implementation Details

### Files to Create

1. `engine/task2/core/config_repo.go` - Main implementation
2. `engine/task2/core/config_repo_test.go` - Comprehensive tests

### Core Logic to Extract

**From ConfigManager storage methods:**

```go
// PrepareParallelConfigs metadata storage (lines 117-134)
metadata := &ParallelTaskMetadata{...}
key := cm.buildParallelMetadataKey(parentStateID)
metadataBytes, err := json.Marshal(metadata)
cm.configStore.SaveMetadata(ctx, key, metadataBytes)

// LoadParallelTaskMetadata retrieval (lines 213-228)
key := cm.buildParallelMetadataKey(parentStateID)
metadataBytes, err := cm.configStore.GetMetadata(ctx, key)
json.Unmarshal(metadataBytes, &metadata)
```

### Implementation Structure

```go
type taskConfigRepository struct {
    configStore ConfigStore
}

func (r *taskConfigRepository) StoreParallelMetadata(ctx context.Context, parentStateID core.ID, metadata *ParallelTaskMetadata) error {
    key := r.buildParallelMetadataKey(parentStateID)
    metadataBytes, err := json.Marshal(metadata)
    if err != nil {
        return fmt.Errorf("failed to marshal parallel metadata: %w", err)
    }
    return r.configStore.SaveMetadata(ctx, key, metadataBytes)
}
```

### Methods to Implement

**Parallel Task Methods:**

- `StoreParallelMetadata()`
- `LoadParallelMetadata()`
- `buildParallelMetadataKey()`

**Collection Task Methods:**

- `StoreCollectionMetadata()`
- `LoadCollectionMetadata()`
- `buildCollectionMetadataKey()`

**Composite Task Methods:**

- `StoreCompositeMetadata()`
- `LoadCompositeMetadata()`
- `buildCompositeMetadataKey()`

**Generic Config Methods:**

- `SaveTaskConfig()`
- `GetTaskConfig()`
- `DeleteTaskConfig()`

**Strategy Management Methods:**

- `ExtractParallelStrategy()` - Safe strategy extraction with fallback
- `ValidateStrategy()` - Strategy validation and normalization
- `CalculateMaxWorkers()` - Worker calculation based on task type

### Strategy Extraction Implementation

**Parallel Strategy Extraction with Graceful Degradation:**

```go
type ParallelConfigData struct {
    Strategy   task.ParallelStrategy `json:"strategy"`
    MaxWorkers int                   `json:"max_workers"`
}

func (r *TaskConfigRepository) ExtractParallelStrategy(ctx context.Context, parentState *task.State) (task.ParallelStrategy, error) {
    const defaultStrategy = task.StrategyWaitAll

    if parentState.Input == nil {
        return defaultStrategy, nil
    }

    // Handle both map and JSON string formats
    var configData ParallelConfigData

    switch v := (*parentState.Input)["parallel_config"].(type) {
    case map[string]interface{}:
        // Convert map to JSON then parse
        jsonBytes, err := json.Marshal(v)
        if err != nil {
            return defaultStrategy, nil // Graceful degradation
        }
        if err := json.Unmarshal(jsonBytes, &configData); err != nil {
            return defaultStrategy, nil // Graceful degradation
        }
    case string:
        if err := json.Unmarshal([]byte(v), &configData); err != nil {
            return defaultStrategy, nil // Graceful degradation
        }
    default:
        return defaultStrategy, nil
    }

    // Validate extracted strategy
    if !r.isValidStrategy(string(configData.Strategy)) {
        return defaultStrategy, nil
    }

    return configData.Strategy, nil
}

func (r *TaskConfigRepository) isValidStrategy(strategy string) bool {
    validStrategies := []string{
        string(task.StrategyWaitAll),
        string(task.StrategyFailFast),
        string(task.StrategyBestEffort),
        string(task.StrategyRace),
    }

    for _, valid := range validStrategies {
        if strategy == valid {
            return true
        }
    }
    return false
}
```

### CWD Propagation Implementation

**Working Directory Management:**

```go
func (r *TaskConfigRepository) StoreParallelMetadata(ctx context.Context, parentStateID core.ID, metadata *ParallelTaskMetadata) error {
    // Propagate CWD to all child configs before storage
    if err := r.propagateCWDToChildren(metadata.ChildConfigs); err != nil {
        return fmt.Errorf("failed to propagate CWD for parent %s: %w", parentStateID, err)
    }

    // Store metadata with proper key
    key := r.buildParallelMetadataKey(parentStateID)
    metadataBytes, err := json.Marshal(metadata)
    if err != nil {
        return fmt.Errorf("failed to marshal parallel metadata for parent %s: %w", parentStateID, err)
    }

    return r.configStore.SaveMetadata(ctx, key, metadataBytes)
}

func (r *TaskConfigRepository) propagateCWDToChildren(childConfigs []*task.Config) error {
    return task.PropagateTaskListCWD(childConfigs, r.cwd)
}
```

## Dependencies

- Task 1: TaskConfigRepository interface
- Existing ConfigStore interface
- Existing metadata types (ParallelTaskMetadata, etc.)

## Testing Requirements

### Unit Tests

- [ ] Parallel metadata store/load cycle
- [ ] Collection metadata store/load cycle
- [ ] Composite metadata store/load cycle
- [ ] Generic task config CRUD operations
- [ ] JSON marshaling error handling
- [ ] ConfigStore error propagation
- [ ] Key generation consistency

### Error Scenarios

- [ ] JSON marshaling failures
- [ ] ConfigStore storage failures
- [ ] Invalid parent state IDs
- [ ] Missing metadata entries
- [ ] Context cancellation

### Edge Cases

- [ ] Empty metadata objects
- [ ] Large metadata payloads
- [ ] Special characters in IDs
- [ ] Concurrent access patterns

## Implementation Notes

- Pure infrastructure layer - no business logic
- Preserve exact JSON serialization format
- Maintain backward compatibility with existing metadata
- Use existing ConfigStore interface without changes
- Follow repository pattern conventions

## Error Handling Strategy

- Wrap ConfigStore errors with meaningful context
- Preserve original error types where possible
- Add parent state ID to error messages
- Handle JSON marshaling failures gracefully

## Implementation Considerations

- Minimal overhead over direct ConfigStore usage
- Efficient key generation and caching
- Proper context cancellation support
- Memory-efficient JSON operations

## Success Criteria

- All ConfigManager storage logic migrated successfully
- Backward compatibility maintained for existing metadata
- All tests pass with >70% coverage
- Error handling comprehensive and consistent
- Code review approved
- Integration tests with real ConfigStore pass
- TaskConfigRepository interface provides clean separation of concerns

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

- [ ] TaskConfigRepository interface fully defined with all methods
- [ ] All metadata store/load methods implemented (Parallel, Composite, Collection)
- [ ] Key generation follows consistent pattern
- [ ] JSON serialization/deserialization working correctly
- [ ] Error handling includes proper context and wrapping
- [ ] Integration with existing ConfigStore verified
- [ ] Transaction support validated where needed
- [ ] Unit tests cover all methods with >70% coverage
- [ ] Mock implementations created for testing
- [ ] Code passes `make lint` and `make test`
