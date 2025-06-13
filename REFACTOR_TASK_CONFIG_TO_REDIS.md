# Refactoring Task.Config from Temporal Activities to Redis-based Retrieval

## Executive Summary

This document outlines a comprehensive refactoring plan to eliminate the practice of passing large `task.Config` objects between Temporal activities. Instead, we will leverage the existing Redis-based `ConfigStore` to store and retrieve configurations using unique task execution IDs.

**Note**: Since Compozy is in development/alpha phase, backward compatibility is NOT required. This allows us to make clean, breaking changes for optimal architecture.

### Key Benefits

- **90%+ reduction in workflow history size** (from MB to KB scale)
- **Faster workflow replay and recovery**
- **Lower memory footprint on workers**
- **Better separation between control flow (IDs) and data (configs)**

## Current State Analysis

### Problem Areas Identified

#### 1. Activities Receiving task.Config Directly (7 instances)

```
├── task/activities/exec_basic.go      → ExecuteBasicInput.TaskConfig
├── task/activities/exec_router.go     → ExecuteRouterInput.TaskConfig
├── task/activities/parallel_state.go  → CreateParallelStateInput.TaskConfig
├── task/activities/collection_state.go→ CreateCollectionStateInput.TaskConfig
├── task/activities/parallel_resp.go   → GetParallelResponseInput.TaskConfig
├── task/activities/collection_resp.go → GetCollectionResponseInput.TaskConfig
└── task/activities/exec_subtask.go    → ✅ Already uses Redis pattern
```

#### 2. Workflow Call Sites Passing task.Config (12 instances in acts_task.go)

- ExecuteBasicTask()
- ExecuteRouterTask()
- CreateParallelState()
- CreateCollectionState()
- GetParallelResponse()
- GetCollectionResponse()
- Plus 6 internal chained calls

#### 3. Config Size Impact

- **Basic task**: ~6-8 KB per serialization
- **Parallel task** (3 children): ~25-30 KB
- **Collection task**: ~40-50 KB
- In long workflows, the same config is serialized multiple times, causing history bloat

## Target Architecture

### Design Pattern

All activities should follow the pattern already implemented in `ExecuteParallelTask`:

```go
// Input contains only IDs
type ActivityInput struct {
    WorkflowID     string
    WorkflowExecID core.ID
    TaskExecID     string   // Key for Redis lookup
}

// Activity fetches config from Redis
func (a *Activity) Run(ctx context.Context, input *ActivityInput) (*Response, error) {
    taskConfig, err := a.configStore.Get(ctx, input.TaskExecID)
    if err != nil {
        return nil, fmt.Errorf("failed to load config: %w", err)
    }
    // Process with taskConfig...
}
```

## Implementation Plan

### Phase 1: Core Infrastructure Changes

#### 1.1 Extend CreateState UC to Save Configs

**File**: `engine/task/uc/create_state.go`

```go
func (uc *CreateState) Execute(ctx context.Context, input *CreateStateInput) (*task.State, error) {
    // Existing: Create state in database
    state, err := uc.taskRepo.CreateState(...)
    if err != nil {
        return nil, err
    }

    // NEW: Save config to Redis
    err = uc.configManager.SaveTaskConfig(ctx, state.TaskExecID, input.TaskConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to save task config: %w", err)
    }

    return state, nil
}
```

#### 1.2 Update ConfigManager

**File**: `engine/task/services/config_manager.go`

```go
// Generalize SaveChildConfig to SaveTaskConfig
func (cm *ConfigManager) SaveTaskConfig(ctx context.Context, taskExecID core.ID, config *task.Config) error {
    return cm.configStore.Save(ctx, string(taskExecID), config)
}
```

### Phase 2: Activity Refactoring

#### 2.1 Pattern for Each Activity

For each activity that currently receives `task.Config`:

1. **Add ConfigStore dependency**

```go
type ActivityStruct struct {
    // existing fields...
    configStore services.ConfigStore // NEW
}
```

2. **Update constructor**

```go
func NewActivity(..., configStore services.ConfigStore) *Activity {
    return &Activity{
        // existing fields...
        configStore: configStore,
    }
}
```

3. **Remove TaskConfig from input**

```go
// Before
type ActivityInput struct {
    ParentState *task.State
    TaskConfig  *task.Config // REMOVE
}

// After
type ActivityInput struct {
    ParentState *task.State
}
```

4. **Fetch config in Run method**

```go
func (a *Activity) Run(ctx context.Context, input *ActivityInput) (*Response, error) {
    // NEW: Fetch config from Redis
    taskConfig, err := a.configStore.Get(ctx, input.ParentState.TaskExecID.String())
    if err != nil {
        return nil, fmt.Errorf("failed to get task config: %w", err)
    }

    // Continue with existing logic using taskConfig
}
```

### Phase 3: Workflow Updates

#### 3.1 Update Activity Calls

**File**: `engine/worker/acts_task.go`

Example for GetCollectionResponse:

```go
// Before
func (e *TaskExecutor) GetCollectionResponse(
    ctx workflow.Context,
    cState *task.State,
    taskConfig *task.Config, // REMOVE
) (task.Response, error) {
    actInput := tkacts.GetCollectionResponseInput{
        ParentState:    cState,
        WorkflowConfig: e.WorkflowConfig,
        TaskConfig:     taskConfig, // REMOVE
    }
    // ...
}

// After
func (e *TaskExecutor) GetCollectionResponse(
    ctx workflow.Context,
    cState *task.State,
) (task.Response, error) {
    actInput := tkacts.GetCollectionResponseInput{
        ParentState:    cState,
        WorkflowConfig: e.WorkflowConfig,
    }
    // ...
}
```

## Detailed Refactoring Steps

### Activities to Refactor

#### 1. ExecuteBasicTask

- **File**: `task/activities/exec_basic.go`
- **Input struct**: Remove `TaskConfig` field
- **Run method**: Fetch config using `WorkflowExecID` + `TaskID`

#### 2. ExecuteRouterTask

- **File**: `task/activities/exec_router.go`
- **Input struct**: Remove `TaskConfig` field
- **Run method**: Fetch config from Redis

#### 3. CreateParallelState

- **File**: `task/activities/parallel_state.go`
- **Special**: Already saves child configs, need to save parent config too
- **Input struct**: Remove `TaskConfig` field

#### 4. CreateCollectionState

- **File**: `task/activities/collection_state.go`
- **Special**: Already saves child configs, need to save parent config too
- **Input struct**: Remove `TaskConfig` field

#### 5. GetParallelResponse

- **File**: `task/activities/parallel_resp.go`
- **Input struct**: Remove `TaskConfig` field
- **Run method**: Fetch from `ParentState.TaskExecID`

#### 6. GetCollectionResponse

- **File**: `task/activities/collection_resp.go`
- **Input struct**: Remove `TaskConfig` field
- **Run method**: Fetch from `ParentState.TaskExecID`

### Special Considerations

#### 1. Initial Task in Workflow

The first task in a workflow needs special handling:

```go
// In TriggerWorkflow activity
func (a *TriggerWorkflow) Run(ctx context.Context, input *TriggerWorkflowInput) (*TriggerWorkflowOutput, error) {
    // ... existing logic ...

    // NEW: Save initial task config
    if firstTask != nil {
        err = a.configManager.SaveTaskConfig(ctx, firstTaskState.TaskExecID, firstTask)
        if err != nil {
            return nil, fmt.Errorf("failed to save initial task config: %w", err)
        }
    }
}
```

#### 2. TTL Management

Current TTL is 24 hours. For long-running workflows:

```go
// Add TTL extension in long-running activities
func (a *LongRunningActivity) Run(ctx context.Context, input *Input) (*Output, error) {
    // Periodically extend TTL
    ticker := time.NewTicker(6 * time.Hour)
    defer ticker.Stop()

    go func() {
        for range ticker.C {
            a.configStore.ExtendTTL(ctx, input.TaskExecID, 24*time.Hour)
        }
    }()

    // ... activity logic ...
}
```

## Testing Strategy

### 1. Unit Tests

- Test each refactored activity with mocked ConfigStore
- Verify error handling when config not found
- Test TTL extension logic

### 2. Integration Tests

```go
func TestWorkflowWithRedisConfig(t *testing.T) {
    t.Run("Should reduce history size by 90%", func(t *testing.T) {
        // Run workflow with old approach
        oldHistory := runWorkflowOldWay()

        // Run workflow with Redis approach
        newHistory := runWorkflowNewWay()

        // Assert significant reduction
        reduction := (1 - float64(len(newHistory))/float64(len(oldHistory))) * 100
        assert.Greater(t, reduction, 90.0)
    })
}
```

### 3. End-to-End Tests

- Test complete workflow execution with Redis-based config
- Verify all task types work correctly
- Measure performance improvements

## Implementation Plan

Since we're in alpha phase, we can implement all changes directly:

1. **Update `CreateState` UC** to save configs to Redis
2. **Refactor all 6 activities** to fetch configs from Redis
3. **Update all 12 workflow call sites** in acts_task.go
4. **Remove old TaskConfig parameters** throughout
5. **Test comprehensively** with all task types

## Risk Mitigation

### 1. Redis Availability

- **Risk**: Redis outage causes activity failures
- **Mitigation**: Temporal's retry mechanism handles transient failures
- **Action**: Configure appropriate retry policies

### 2. Config Loss Due to TTL

- **Risk**: Long-paused workflows lose configs
- **Mitigation**: Make TTL configurable per workflow type
- **Action**: Set TTL = max_workflow_duration \* 2

### 3. Performance Impact

- **Risk**: Additional Redis calls add latency
- **Mitigation**: Redis calls are fast (~1ms), offset by smaller payloads
- **Action**: Monitor p95/p99 latencies during rollout

## Success Metrics

1. **History Size Reduction**: >90% reduction in workflow history size
2. **Replay Speed**: 5-10x faster workflow replay
3. **Worker Memory**: 30-50% reduction in worker memory usage
4. **Redis Load**: <5% CPU increase on Redis cluster
5. **Activity Latency**: <10ms increase in p95 latency

## Conclusion

This refactoring eliminates a significant anti-pattern in our Temporal usage while leveraging existing infrastructure. Since we're in alpha phase, we can make these breaking changes cleanly without worrying about backward compatibility. The end result will be a more scalable, performant, and maintainable workflow system that follows Temporal best practices.
