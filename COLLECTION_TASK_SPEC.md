# Collection Task Feature Specification

## Overview

This document provides a comprehensive specification for implementing the **Collection Task** feature in Compozy. A Collection Task allows iterating over a collection of items and executing a task template for each item, supporting both parallel and sequential execution modes with advanced filtering and batching capabilities.

## Architecture Analysis

### Current System Patterns

The Compozy task system follows these established patterns:

1. **Activity-based Execution**: Each task type has dedicated activities (e.g., `ExecuteBasic`, `ExecuteRouter`, `CreateParallelState`)
2. **Use Case Pattern**: Business logic is encapsulated in use cases within the `uc` package
3. **Domain/Config Separation**: Clear separation between configuration (`config.go`) and domain logic (`domain.go`)
4. **State Management**: Type-specific state structures for different execution patterns
5. **Validation Chain**: Extensible validator pattern with type-specific validation
6. **Registration Pattern**: Activities registered in worker setup with consistent naming

### Reusable Components

The Collection Task can leverage existing infrastructure:

#### ✅ **Parallel Execution Infrastructure**
- `ParallelStrategy` enum (wait_all, fail_fast, best_effort, race)
- Parallel state management patterns
- Sub-task execution and result aggregation logic
- Worker coordination and context management

#### ✅ **Existing Use Cases**
- `LoadWorkflow` - Load workflow state and configuration
- `CreateState` - Create task state with proper initialization
- `HandleResponse` - Handle task completion and transitions
- `ExecuteTask` - Execute individual task instances
- `NormalizeConfig` - Template evaluation and normalization

#### ✅ **State Management Patterns**
- Extend existing `State` struct with `CollectionState`
- Reuse status tracking and error handling
- Leverage existing database serialization patterns

#### ✅ **Activity and Worker Patterns**
- Follow established activity structure and registration
- Reuse context building and error handling
- Leverage existing Temporal workflow patterns

### Key Differences from Parallel Tasks

1. **Dynamic Item Generation**: Items come from template evaluation rather than predefined tasks
2. **Filtering**: Apply filter expressions to items before processing
3. **Sequential Batching**: Process items in configurable batch sizes for sequential mode
4. **Variable Injection**: Inject item and index variables into task template
5. **Error Tolerance**: `continue_on_error` flag for graceful error handling

## Feature Specification

### Configuration Structure

```yaml
id: process_user_notifications
type: collection
items: "{{ .workflow.input.users }}"                    # Source collection (required)
filter: "{{ and .item.active (not .item.notified) }}"   # Optional filter expression
mode: parallel                                          # parallel | sequential (default: parallel)
batch: 5                                                # Batch size for sequential mode (default: 1)
continue_on_error: true                                 # Continue on individual item failures (default: false)

# Parallel execution options (when mode: parallel)
strategy: wait_all                                      # wait_all | fail_fast | best_effort | race
max_workers: 10                                         # Maximum concurrent workers
timeout: "5m"                                           # Overall timeout

# Variable names for template evaluation
item_var: user                                          # Variable name for current item (default: "item")
index_var: user_index                                   # Variable name for current index (default: "index")

# Task template executed for each item
task:
  id: notify_user
  type: basic
  agent:
    id: notification_agent
  action: send_notification
  with:
    user_id: "{{ .user.id }}"
    email: "{{ .user.email }}"
    index: "{{ .user_index }}"

# Optional early termination
stop_condition: "{{ gt .summary.failed 10 }}"          # Stop if more than 10 failures

# Standard task properties
input: { ... }
output: { ... }
on_success: { ... }
on_error: { ... }
```

### Execution Modes

#### Parallel Mode
- Execute all filtered items concurrently
- Respect `max_workers` limit
- Apply `strategy` for failure handling
- Complete when strategy conditions are met

#### Sequential Mode
- Execute items one by one or in batches
- `batch: 1` - Process items individually
- `batch: N` - Process N items at a time
- Useful for rate limiting and resource management

### Output Structure

```json
{
  "results": [
    {
      "index": 0,
      "item": { "id": "user1", "email": "user1@example.com" },
      "status": "success",
      "output": { "notification_id": "n123" }
    },
    {
      "index": 1,
      "item": { "id": "user2", "email": "user2@example.com" },
      "status": "failed",
      "error": "Invalid email address"
    }
  ],
  "summary": {
    "total_items": 100,
    "filtered_items": 85,
    "completed": 80,
    "failed": 5,
    "skipped": 15,
    "mode": "parallel",
    "strategy": "wait_all",
    "execution_time": "2m15s"
  }
}
```

## Temporal Best Practices Analysis

### Key Findings from Temporal Documentation

Based on [Temporal's best practices](https://community.temporal.io/t/determining-the-best-practices-for-business-logic-placement-within-temporal-activities-and-workflows/8951) and [production recommendations](https://medium.com/@ajayshekar01/best-practices-for-building-temporal-workflows-a-practical-guide-with-examples-914fedd2819c), our initial Collection Task design needs significant improvements:

#### ❌ **Issues with Original Design**:
1. **Monolithic Activity**: Single `ExecuteCollection` activity violates the "atomic activities" principle
2. **Large Payload Risk**: Processing entire collections in one activity could exceed Temporal's size limits
3. **Poor Scalability**: Doesn't handle large collections (millions of items) efficiently
4. **Missing Continue-as-New**: No strategy for preventing workflow history growth

#### ✅ **Temporal-Aligned Approach**:

## Revised Implementation Plan

### Phase 1: Atomic Activity Design

Following Temporal's recommendation to **"Design Activities to Be Atomic"** and **"Handle Large Data Efficiently"**:

#### 1.1 Collection Activities (Atomic & Idempotent)
```go
// engine/task/activities/collection_prepare.go
const PrepareCollectionLabel = "PrepareCollection"

type PrepareCollectionInput struct {
    WorkflowID     string       `json:"workflow_id"`
    WorkflowExecID core.ID      `json:"workflow_exec_id"`
    TaskConfig     *task.Config `json:"task_config"`
}

type PrepareCollectionResult struct {
    TaskExecID     core.ID `json:"task_exec_id"`     // Collection task execution ID
    FilteredCount  int     `json:"filtered_count"`   // Number of items after filtering
    TotalCount     int     `json:"total_count"`      // Original number of items
    BatchCount     int     `json:"batch_count"`      // Number of batches to process
    CollectionState *task.State `json:"collection_state"` // Collection state stored in DB
}

// engine/task/activities/collection_item.go
const ExecuteCollectionItemLabel = "ExecuteCollectionItem"

type ExecuteCollectionItemInput struct {
    ParentTaskExecID core.ID      `json:"parent_task_exec_id"` // Parent collection task
    ItemIndex        int          `json:"item_index"`          // Index in collection
    TaskConfig       *task.Config `json:"task_config"`         // Template task config
}

type ExecuteCollectionItemResult struct {
    ItemIndex    int             `json:"item_index"`
    TaskExecID   core.ID         `json:"task_exec_id"`    // Child task execution ID
    Status       core.StatusType `json:"status"`
    Output       *core.Output    `json:"output,omitempty"`
    Error        *core.Error     `json:"error,omitempty"`
}

// engine/task/activities/collection_aggregate.go
const AggregateCollectionLabel = "AggregateCollection"

type AggregateCollectionInput struct {
    ParentTaskExecID core.ID                        `json:"parent_task_exec_id"` // Collection task ID
    ItemResults      []ExecuteCollectionItemResult  `json:"item_results"`        // Individual results
    TaskConfig       *task.Config                   `json:"task_config"`
}
```

#### 1.2 Workflow Orchestration Pattern
```go
// engine/worker/acts_task_collection.go
func (e *TaskExecutor) HandleCollectionTask(taskConfig *task.Config) func(ctx workflow.Context) (*task.Response, error) {
    return func(ctx workflow.Context) (*task.Response, error) {
        // Step 1: Prepare collection (atomic)
        prepareResult, err := e.prepareCollection(ctx, taskConfig)
        if err != nil {
            return nil, err
        }

        // Step 2: Process items based on mode
        var itemResults []ExecuteCollectionItemResult
        if taskConfig.GetMode() == CollectionModeParallel {
            itemResults, err = e.processItemsParallel(ctx, prepareResult, taskConfig)
        } else {
            itemResults, err = e.processItemsSequential(ctx, prepareResult, taskConfig)
        }
        if err != nil {
            return nil, err
        }

        // Step 3: Aggregate results (atomic)
        return e.aggregateResults(ctx, itemResults, taskConfig)
    }
}
```

### Phase 2: Large Scale Support

Following the [sliding window pattern](https://community.temporal.io/t/best-practices-for-implementing-a-workflow-to-process-millions-of-files-concurrently-with-heavy-child-workflow-activities/13140) for millions of items:

#### 2.1 Sliding Window Implementation
```go
func (e *TaskExecutor) processItemsSequential(
    ctx workflow.Context, 
    prepareResult *PrepareCollectionResult,
    taskConfig *task.Config,
) ([]ExecuteCollectionItemResult, error) {
    batchSize := taskConfig.GetBatch()
    totalBatches := prepareResult.BatchCount
    
    // Use continue-as-new for very large collections
    if totalBatches > 1000 { // Configurable threshold
        return e.processWithContinueAsNew(ctx, prepareResult, taskConfig)
    }
    
    var allResults []ExecuteCollectionItemResult
    for batchIndex := 0; batchIndex < totalBatches; batchIndex++ {
        batchResults, err := e.processBatch(ctx, batchIndex, batchSize, prepareResult.ItemsRef, taskConfig)
        if err != nil && !taskConfig.ContinueOnError {
            return nil, err
        }
        allResults = append(allResults, batchResults...)
        
        // Check stop condition
        if e.shouldStopProcessing(allResults, taskConfig) {
            break
        }
    }
    return allResults, nil
}
```

#### 2.2 Continue-as-New for Large Collections
```go
func (e *TaskExecutor) processWithContinueAsNew(
    ctx workflow.Context,
    prepareResult *PrepareCollectionResult,
    taskConfig *task.Config,
) ([]ExecuteCollectionItemResult, error) {
    // Process in chunks that respect Temporal's history limits
    maxBatchesPerExecution := 100
    
    for startBatch := 0; startBatch < prepareResult.BatchCount; startBatch += maxBatchesPerExecution {
        endBatch := min(startBatch+maxBatchesPerExecution, prepareResult.BatchCount)
        
        // Process batch range
        results, err := e.processBatchRange(ctx, startBatch, endBatch, prepareResult, taskConfig)
        if err != nil {
            return nil, err
        }
        
        // Continue-as-new if more batches remain
        if endBatch < prepareResult.BatchCount {
            return nil, workflow.NewContinueAsNewError(ctx, "CollectionWorkflow", 
                prepareResult, taskConfig, results, endBatch)
        }
        
        return results, nil
    }
    return nil, nil
}
```

### Phase 3: Core Type System (Simplified)

#### 3.1 Minimal Configuration Changes
```go
// engine/task/config.go - Add collection type
const (
    TaskTypeBasic      Type = "basic"
    TaskTypeRouter     Type = "router"
    TaskTypeParallel   Type = "parallel"
    TaskTypeCollection Type = "collection"
)

type CollectionTask struct {
    // Core fields only - keep it simple
    Items           string           `json:"items" yaml:"items"`
    Filter          string           `json:"filter,omitempty" yaml:"filter,omitempty"`
    Task            *Config          `json:"task" yaml:"task"`
    Mode            CollectionMode   `json:"mode,omitempty" yaml:"mode,omitempty"`
    Batch           int              `json:"batch,omitempty" yaml:"batch,omitempty"`
    ContinueOnError bool             `json:"continue_on_error,omitempty" yaml:"continue_on_error,omitempty"`
    
    // Reuse existing parallel fields
    Strategy        ParallelStrategy `json:"strategy,omitempty" yaml:"strategy,omitempty"`
    MaxWorkers      int              `json:"max_workers,omitempty" yaml:"max_workers,omitempty"`
    Timeout         string           `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}
```

### Phase 4: PostgreSQL-Native Storage

Leveraging the existing PostgreSQL infrastructure and `task_states` table:

#### 4.1 Collection State Storage
```go
// engine/task/domain.go - Extend existing structures
type CollectionState struct {
    Items           []any            `json:"items"`             // Filtered collection items
    Filter          string           `json:"filter,omitempty"`  // Filter expression used
    Mode            CollectionMode   `json:"mode"`              // parallel | sequential
    Batch           int              `json:"batch"`             // Batch size for sequential
    ContinueOnError bool             `json:"continue_on_error"` // Error tolerance
    ItemVar         string           `json:"item_var"`          // Variable name for item
    IndexVar        string           `json:"index_var"`         // Variable name for index
    
    // Progress tracking
    ProcessedCount  int              `json:"processed_count"`   // Items processed so far
    CompletedCount  int              `json:"completed_count"`   // Successfully completed
    FailedCount     int              `json:"failed_count"`      // Failed items
    
    // Collection item results - stored as task_states with parent reference
    ItemResults     []string         `json:"item_results"`      // Array of child task_exec_ids
}

// Extend existing ParallelState or create CollectionState in existing JSON column
// No schema changes needed - use existing parallel_state JSONB column
```

#### 4.2 Storage in Existing task_states Table
```sql
-- Use existing table structure with parent-child relationships
-- Parent collection task
INSERT INTO task_states (
    task_exec_id, task_id, workflow_exec_id, workflow_id,
    component, status, execution_type, parallel_state, ...
) VALUES (
    'collection_123', 'process_users', 'wf_456', 'notify_workflow',
    'task', 'running', 'collection', 
    '{"items": [...], "mode": "parallel", "item_results": [...]}'::jsonb, ...
);

-- Child item tasks
INSERT INTO task_states (
    task_exec_id, task_id, workflow_exec_id, workflow_id,
    component, status, execution_type, input, ...
) VALUES (
    'item_001', 'process_users.item[0]', 'wf_456', 'notify_workflow',
    'agent', 'pending', 'basic',
    '{"user": {"id": "user1"}, "index": 0}'::jsonb, ...
);
```

#### 4.3 Advantages of PostgreSQL-Native Approach

**✅ Infrastructure Benefits:**
- **No New Dependencies**: Zero additional infrastructure components
- **ACID Transactions**: Guaranteed consistency across collection processing
- **Existing Monitoring**: Leverage current PostgreSQL monitoring and alerting
- **Backup/Recovery**: Collection state included in existing backup strategies
- **Security**: Use existing database security and access controls

**✅ Performance Benefits:**
- **JSONB Indexing**: Fast queries on collection items and metadata
- **Efficient Queries**: Parent-child relationships via existing indexes
- **Bulk Operations**: PostgreSQL's efficient JSONB array operations
- **Connection Pooling**: Reuse existing database connection management

**✅ Operational Benefits:**
- **Single Source of Truth**: All task state in one consistent location
- **Existing Tooling**: Use current database administration tools
- **Familiar Patterns**: Follows established repository and state patterns
- **Easy Debugging**: Query collection state directly via SQL for troubleshooting

**✅ Code Simplicity:**
```go
// Get collection items (no external API calls)
collectionState, err := taskRepo.GetState(ctx, collectionTaskExecID)
items := collectionState.ParallelState.(*CollectionState).Items

// Get item processing results (existing query patterns)
childStates, err := taskRepo.ListTasksInWorkflow(ctx, workflowExecID)
results := filterByParentTask(childStates, collectionTaskExecID)
```

## Implementation Guidelines (Revised)

### Temporal Alignment Principles

1. **Atomic Activities**: Each activity does one thing well
   - `PrepareCollection`: Filter items, store in PostgreSQL collection state, return task reference
   - `ExecuteCollectionItem`: Process single item atomically, store result as child task state
   - `AggregateCollection`: Query child task states and combine results efficiently

2. **PostgreSQL-Native Storage**: Leverage existing database infrastructure
   - Store filtered items in collection task's `parallel_state` JSONB column
   - Create child task states for individual item processing
   - Use parent-child relationships via `workflow_exec_id` and task naming convention
   - Query results using existing task repository methods

3. **Workflow Orchestration**: Use workflow for coordination, not computation
   - Workflow orchestrates the collection processing flow
   - All heavy lifting happens in activities
   - Simple, deterministic iteration logic in workflow

4. **Scalability Patterns**: Handle any collection size
   - Sliding window pattern for medium collections (1K-100K items)
   - Continue-as-new for large collections (100K+ items)
   - PostgreSQL JSONB indexing for efficient item queries

5. **Resource Management**: Leverage Temporal's built-in controls
   - Use `MaxConcurrentActivityExecutionSize` for resource limits
   - Respect activity timeouts and retry policies
   - Implement proper heartbeating for long-running item processing

### Performance Considerations (Updated)

1. **Memory Efficiency**: Items stored in PostgreSQL JSONB, not in workflow state
2. **History Management**: Continue-as-new prevents history growth  
3. **Resource Control**: Configurable concurrency limits per worker
4. **Progress Tracking**: Atomic progress updates via task state updates
5. **Fault Tolerance**: Individual item failures stored as separate task states
6. **Database Efficiency**: Use existing JSONB indexes and parent-child queries

## Implementation Guidelines

### Code Organization Principles

1. **Follow Existing Patterns**: Maintain consistency with current activity, use case, and domain patterns
2. **Maximize Reuse**: Leverage existing parallel task infrastructure where possible
3. **Single Responsibility**: Keep each component focused on a specific concern
4. **Error Handling**: Follow established error handling and context cancellation patterns
5. **Testing Strategy**: Follow existing test patterns with unit and integration tests

### Performance Considerations

1. **Memory Management**: Efficiently handle large collections without loading everything into memory
2. **Batch Processing**: Implement efficient batching for sequential mode
3. **Resource Limits**: Respect max_workers and timeout constraints
4. **Template Caching**: Cache compiled templates for repeated evaluations

### Security Considerations

1. **Template Safety**: Ensure template expressions are safely evaluated
2. **Resource Limits**: Prevent resource exhaustion with large collections
3. **Input Validation**: Validate collection items and filter expressions
4. **Error Information**: Avoid exposing sensitive data in error messages

## Testing Strategy

### Unit Tests
- Configuration validation
- State management operations
- Template evaluation and filtering
- Error handling scenarios

### Integration Tests
- End-to-end collection task execution
- Different execution modes (parallel/sequential)
- Error scenarios and recovery
- Performance with large collections

### Example Test Cases
```go
func TestCollectionTaskParallelExecution(t *testing.T) {
    // Test parallel execution with success scenario
}

func TestCollectionTaskSequentialBatching(t *testing.T) {
    // Test sequential execution with batching
}

func TestCollectionTaskFilteringAndContinueOnError(t *testing.T) {
    // Test filtering and error tolerance
}
```

## Migration Considerations

### Backward Compatibility
- All existing task types remain unchanged
- No breaking changes to existing APIs
- Gradual rollout with feature flags if needed

### Database Schema
- Extend task state tables to support collection state
- Migration scripts for new columns
- Backward compatibility with existing states

## Future Enhancements

### Potential Extensions
1. **Dynamic Batching**: Adjust batch size based on performance metrics
2. **Nested Collections**: Support collections of collections
3. **Streaming Processing**: Process items as they arrive
4. **Progress Reporting**: Real-time progress updates for long-running collections
5. **Conditional Execution**: Execute different task templates based on item properties

### Integration Opportunities
1. **Monitoring**: Enhanced metrics for collection task performance
2. **Alerting**: Collection-specific alerting rules
3. **Dashboard**: Visual representation of collection execution progress
4. **Logging**: Structured logging for collection task events

## Key Improvements Based on Temporal Best Practices

### Original vs. Revised Approach

| Aspect | ❌ Original Design | ✅ PostgreSQL-Optimized Design |
|--------|------------------|-------------------------------|
| **Activity Design** | Single monolithic activity | Three atomic, idempotent activities |
| **Data Handling** | Large payloads through Temporal | PostgreSQL JSONB storage with task references |
| **Scalability** | Limited by workflow history | Sliding window + continue-as-new + DB indexing |
| **Resource Management** | Custom implementation | Leverage Temporal's built-in controls |
| **Error Handling** | Complex retry logic | Simple, atomic failure handling via task states |
| **Storage Infrastructure** | New external dependencies | Existing PostgreSQL with zero new dependencies |
| **Code Reuse** | 80% reuse claimed | 95%+ reuse with existing task/parallel patterns |

### Benefits of PostgreSQL-Optimized Approach

1. **Production Ready**: Follows patterns used by [Stripe, Coinbase, and Netflix](https://docs.temporal.io/evaluate/use-cases-design-patterns)
2. **Handles Scale**: Supports millions of items using PostgreSQL JSONB and existing parallel patterns
3. **Zero New Dependencies**: Leverages existing PostgreSQL infrastructure with JSONB indexing
4. **Better Observability**: Atomic activities provide granular monitoring in Temporal Web UI
5. **Improved Maintainability**: Each activity has single responsibility and clear boundaries
6. **Resource Efficiency**: Leverages Temporal's worker resource management + PostgreSQL performance
7. **Future-Proof**: Easier to add features like progress reporting and dynamic scaling
8. **Consistency**: All state management follows existing task state patterns
9. **Transaction Safety**: Leverages PostgreSQL ACID properties for data consistency

### Recommended Configuration Examples

#### Small Collections (< 1,000 items)
```yaml
id: notify_users_small
type: collection
items: "{{ .workflow.input.users }}"
filter: "{{ .item.active }}"
mode: parallel
strategy: wait_all
max_workers: 10
continue_on_error: true

task:
  id: send_notification
  type: basic
  agent:
    id: notification_agent
  action: send_email
```

#### Large Collections (100,000+ items)  
```yaml
id: process_files_large
type: collection
items: "{{ .workflow.input.file_refs }}"  # References, not full objects
filter: "{{ gt .item.size 1000000 }}"
mode: sequential
batch: 100                                # Process in batches
continue_on_error: true

task:
  id: process_file
  type: basic
  tool:
    id: file_processor
  with:
    file_ref: "{{ .item.ref }}"           # Reference only
```

## Conclusion

This revised specification provides a production-ready plan for implementing Collection Tasks that aligns with Temporal's architecture and best practices. By following patterns used by leading organizations and leveraging Temporal's built-in capabilities, we ensure:

- **Scalability**: Handle any collection size efficiently
- **Reliability**: Atomic activities with proper error handling  
- **Maintainability**: Clean separation of concerns
- **Performance**: External storage and resource management
- **Observability**: Granular monitoring and progress tracking

The Collection Task feature will significantly enhance Compozy's capability to handle data processing pipelines, user notification campaigns, and other scenarios requiring iteration over collections of items, while following industry best practices for Temporal workflows. 