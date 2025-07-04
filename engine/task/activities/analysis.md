# Engine Task Activities Analysis

## Executive Summary

The `engine/task/activities` folder contains 23 files implementing Temporal workflow activities for the Compozy task execution engine. These activities handle 9 different task types (basic, collection, parallel, router, signal, wait, memory, aggregate, composite) plus shared utilities.

## File Categorization by Task Type

### 1. Basic Task Type (1 activity)

- `exec_basic.go` - Executes basic component tasks

### 2. Collection Task Type (2 activities)

- `collection_state.go` - Creates collection task state and expands items
- `collection_resp.go` - Handles collection task response aggregation

### 3. Parallel Task Type (2 activities)

- `parallel_state.go` - Creates parallel task state
- `parallel_resp.go` - Handles parallel task response aggregation

### 4. Composite Task Type (2 activities)

- `composite_state.go` - Creates composite task state (sequential tasks)
- `composite_resp.go` - Handles composite task response

### 5. Router Task Type (1 activity)

- `exec_router.go` - Executes conditional routing logic

### 6. Signal Task Type (2 activities)

- `exec_signal.go` - Dispatches signals to other workflows
- `exec_signal_test.go` - Tests for signal execution

### 7. Wait Task Type (4 activities)

- `exec_wait.go` - Creates wait task state
- `exec_wait_test.go` - Tests for wait execution
- `wait_helpers.go` - CEL condition evaluation for wait tasks
- `wait_processor.go` - Normalizes wait task processor configs

### 8. Memory Task Type (1 activity)

- `exec_memory.go` - Executes memory operations (store/retrieve)

### 9. Aggregate Task Type (1 activity)

- `exec_aggregate.go` - Executes data aggregation without child tasks

### 10. Shared/Common Activities (7 files)

- `exec_subtask.go` - Executes child tasks for collection/parallel/composite
- `response_converter.go` - Converts responses between task2 and task formats
- `response_helpers.go` - Common response processing logic
- `response_helpers_test.go` - Tests for response helpers
- `get_progress.go` - Gets task execution progress
- `list_children.go` - Lists child task states
- `load_config.go` - Loads task configurations (4 activities)
- `update_child_state.go` - Updates child task state
- `update_parent.go` - Updates parent task status

## Activity Count by Task Type

| Task Type  | Primary Activities | Helper Activities | Total  |
| ---------- | ------------------ | ----------------- | ------ |
| Basic      | 1                  | 0                 | 1      |
| Collection | 2                  | 0                 | 2      |
| Parallel   | 2                  | 0                 | 2      |
| Composite  | 2                  | 0                 | 2      |
| Router     | 1                  | 0                 | 1      |
| Signal     | 1                  | 0                 | 1      |
| Wait       | 1                  | 2                 | 3      |
| Memory     | 1                  | 0                 | 1      |
| Aggregate  | 1                  | 0                 | 1      |
| **Shared** | **0**              | **9**             | **9**  |
| **TOTAL**  | **12**             | **11**            | **23** |

## Key Patterns and Dependencies

### 1. Execution Pattern

Tasks follow a consistent execution pattern:

- State creation activities (`*_state.go`) - Create initial task state
- Execution activities (`exec_*.go`) - Execute the task logic
- Response activities (`*_resp.go`) - Handle task completion and aggregation

### 2. Task2 Integration

All activities heavily integrate with the `task2` package:

- Use `task2.Factory` for creating normalizers and response handlers
- Use `task2.ResponseHandler` for consistent response processing
- Use `task2.CollectionExpander` for collection item expansion
- Use `shared.NormalizationContext` for template processing

### 3. Parent-Child Relationships

Three task types create child tasks:

- **Collection**: Expands items into child tasks
- **Parallel**: Creates child tasks for parallel execution
- **Composite**: Creates sequential child tasks

These share common patterns:

- Store metadata using `task2core.*TaskMetadata`
- Use `CreateChildTasksUC` to create children
- Process responses with `processParentTask` helper
- Aggregate outputs using `aggregateChildOutputs`

### 4. Shared Dependencies

Common dependencies across activities:

- `workflow.Repository` - Access workflow state/config
- `task.Repository` - Access task state
- `services.ConfigStore` - Store/retrieve task configs
- `tplengine.TemplateEngine` - Template processing
- `uc.*` - Use case implementations

### 5. Complex Dependencies

#### Response Processing Chain

1. `response_helpers.go` provides core functions:
    - `processParentTask` - Main orchestrator for parent tasks
    - `aggregateChildOutputs` - Collects child outputs
    - `buildDetailedFailureError` - Creates detailed error messages

2. `response_converter.go` provides conversion utilities:
    - `ConvertToMainTaskResponse` - Basic response conversion
    - `ConvertToCollectionResponse` - Collection-specific conversion

3. Parent task responses depend on:
    - `get_progress.go` - To check child completion
    - `list_children.go` - To get child states
    - `update_parent.go` - To update parent status

#### Configuration Loading

`load_config.go` provides 4 different loading strategies:

- `LoadTaskConfig` - Single config from workflow
- `LoadBatchConfigs` - Multiple configs from workflow
- `LoadCompositeConfigs` - From composite metadata
- `LoadCollectionConfigs` - From collection metadata

#### Wait Task Processing

Wait tasks have the most complex flow:

1. `exec_wait.go` - Creates waiting state
2. `wait_helpers.go` - Evaluates CEL conditions
3. `wait_processor.go` - Normalizes processor config with signal data

## Refactoring Considerations

### 1. Clear Separation Opportunities

- Each task type has distinct execution logic
- Response handling is task-type specific
- State creation follows similar patterns but has type-specific logic

### 2. Shared Code Challenges

- `response_helpers.go` is used by collection, parallel, and composite
- `response_converter.go` is used across multiple task types
- `exec_subtask.go` handles all child task execution
- Configuration loading is shared across parent task types

### 3. Proposed Folder Structure

```
activities/
├── basic/
│   └── exec_basic.go
├── collection/
│   ├── collection_state.go
│   └── collection_resp.go
├── parallel/
│   ├── parallel_state.go
│   └── parallel_resp.go
├── composite/
│   ├── composite_state.go
│   └── composite_resp.go
├── router/
│   └── exec_router.go
├── signal/
│   ├── exec_signal.go
│   └── exec_signal_test.go
├── wait/
│   ├── exec_wait.go
│   ├── exec_wait_test.go
│   ├── wait_helpers.go
│   └── wait_processor.go
├── memory/
│   └── exec_memory.go
├── aggregate/
│   └── exec_aggregate.go
└── shared/
    ├── exec_subtask.go
    ├── response_converter.go
    ├── response_helpers.go
    ├── response_helpers_test.go
    ├── get_progress.go
    ├── list_children.go
    ├── load_config.go
    ├── update_child_state.go
    └── update_parent.go
```

### 4. Dependency Management

The shared folder would need to be carefully managed as:

- Parent task types (collection, parallel, composite) depend heavily on shared helpers
- All task types use response_converter for output formatting
- Subtask execution is common to all parent types

### 5. Testing Considerations

- Only signal and wait tasks have dedicated test files
- Response helpers have tests that would move to shared
- Each task type folder would benefit from comprehensive tests
