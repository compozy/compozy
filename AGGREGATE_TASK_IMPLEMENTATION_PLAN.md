# Aggregate Task Implementation Plan

## Overview

Implement a new "aggregate" task type that combines results from multiple tasks into a single output using template-based transformations. This task type will be purely template-driven and leverage existing infrastructure.

## Key Design Decisions

### 1. No New Struct Required

- **Rationale**: Aggregate tasks only need template-based output processing
- **Implementation**: Use existing `BaseConfig.Outputs` field with `TaskTypeAggregate` type
- **Benefits**: Minimal code changes, consistent with existing patterns

### 2. Template-Only Execution

- **No external execution**: Pure in-memory template evaluation
- **Reuse existing template engine**: Leverage `tplengine.ProcessOutputs()`
- **Security**: Inherits existing template sandboxing and timeout protection

### 3. Standard Task Lifecycle

- **Validation**: Check for required `outputs` field and template syntax
- **Execution**: Process templates using workflow context
- **Integration**: Follow same patterns as other task types

## Implementation Phases

### Phase 1: Core Infrastructure

#### 1.1 Add Task Type Constant

**File**: `engine/task/config.go`

```go
const (
    TaskTypeBasic      Type = "basic"
    TaskTypeRouter     Type = "router"
    TaskTypeParallel   Type = "parallel"
    TaskTypeCollection Type = "collection"
    TaskTypeAggregate  Type = "aggregate"  // Add this line
)
```

#### 1.2 Update Execution Type Mapping

**File**: `engine/task/config.go` - Update `GetExecType()` method

```go
func (t *Config) GetExecType() ExecutionType {
    taskType := t.Type
    if taskType == "" {
        taskType = TaskTypeBasic
    }
    var executionType ExecutionType
    switch taskType {
    case TaskTypeRouter:
        executionType = ExecutionRouter
    case TaskTypeParallel:
        executionType = ExecutionParallel
    case TaskTypeCollection:
        executionType = ExecutionCollection
    case TaskTypeAggregate:
        executionType = ExecutionBasic  // Aggregate uses basic execution
    default:
        executionType = ExecutionBasic
    }
    return executionType
}
```

### Phase 2: Validation

#### 2.1 Update Task Type Validator

**File**: `engine/task/validators.go` - Update `TypeValidator.Validate()` method

```go
func (v *TypeValidator) Validate() error {
    switch v.config.Type {
    case TaskTypeBasic:
        return v.validateBasicTaskWithRef()
    case TaskTypeRouter:
        return v.validateRouterTask()
    case TaskTypeParallel:
        return v.validateParallelTask()
    case TaskTypeCollection:
        return v.validateCollectionTask()
    case TaskTypeAggregate:
        return v.validateAggregateTask()  // Add this case
    default:
        return fmt.Errorf("invalid task type: %s", v.config.Type)
    }
}
```

#### 2.2 Add Aggregate Task Validation

**File**: `engine/task/validators.go`

```go
func (v *TypeValidator) validateAggregateTask() error {
    if v.config.Outputs == nil || len(*v.config.Outputs) == 0 {
        return fmt.Errorf("aggregate tasks must have outputs defined")
    }

    // Validate template syntax in outputs
    for key, template := range *v.config.Outputs {
        if templateStr, ok := template.(string); ok {
            if err := validateTemplateString(templateStr); err != nil {
                return fmt.Errorf("invalid template in output '%s': %w", key, err)
            }
        }
    }

    // Aggregate tasks should not have action, agent, or tool
    if v.config.Action != "" {
        return fmt.Errorf("aggregate tasks cannot have an action field")
    }
    if v.config.Agent != nil {
        return fmt.Errorf("aggregate tasks cannot have an agent")
    }
    if v.config.Tool != nil {
        return fmt.Errorf("aggregate tasks cannot have a tool")
    }

    return nil
}

// Helper function to validate template syntax
func validateTemplateString(template string) error {
    // Use existing template validation logic from tplengine
    // This is a placeholder - implement based on existing validation patterns
    if template == "" {
        return fmt.Errorf("template cannot be empty")
    }
    return nil
}
```

### Phase 3: Execution Activity

#### 3.1 Create Execution Activity

**File**: `engine/task/activities/exec_aggregate.go`

```go
package activities

import (
    "context"
    "fmt"
    "time"

    "github.com/compozy/compozy/engine/core"
    "github.com/compozy/compozy/engine/task"
    "github.com/compozy/compozy/engine/task/services"
    "github.com/compozy/compozy/engine/task/uc"
    "github.com/compozy/compozy/engine/workflow"
    "github.com/compozy/compozy/pkg/tplengine"
)

const ExecuteAggregateLabel = "ExecuteAggregateTask"

type ExecuteAggregateInput struct {
    WorkflowID     string       `json:"workflow_id"`
    WorkflowExecID core.ID      `json:"workflow_exec_id"`
    TaskConfig     *task.Config `json:"task_config"`
}

type ExecuteAggregate struct {
    loadWorkflowUC *uc.LoadWorkflow
    createStateUC  *uc.CreateState
    taskResponder  *services.TaskResponder
}

func NewExecuteAggregate(
    workflows []*workflow.Config,
    workflowRepo workflow.Repository,
    taskRepo task.Repository,
    configStore services.ConfigStore,
    cwd *core.PathCWD,
) *ExecuteAggregate {
    configManager := services.NewConfigManager(configStore, cwd)
    return &ExecuteAggregate{
        loadWorkflowUC: uc.NewLoadWorkflow(workflows, workflowRepo),
        createStateUC:  uc.NewCreateState(taskRepo, configManager),
        taskResponder:  services.NewTaskResponder(workflowRepo, taskRepo),
    }
}

func (a *ExecuteAggregate) Run(ctx context.Context, input *ExecuteAggregateInput) (*task.MainTaskResponse, error) {
    // Validate task type
    if input.TaskConfig.Type != task.TaskTypeAggregate {
        return nil, fmt.Errorf("unsupported task type: %s", input.TaskConfig.Type)
    }

    // Load workflow state and config
    workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
        WorkflowID:     input.WorkflowID,
        WorkflowExecID: input.WorkflowExecID,
    })
    if err != nil {
        return nil, err
    }

    // Normalize task config
    normalizer := uc.NewNormalizeConfig()
    normalizeInput := &uc.NormalizeConfigInput{
        WorkflowState:  workflowState,
        WorkflowConfig: workflowConfig,
        TaskConfig:     input.TaskConfig,
    }
    err = normalizer.Execute(ctx, normalizeInput)
    if err != nil {
        return nil, err
    }

    // Create task state
    taskState, err := a.createStateUC.Execute(ctx, &uc.CreateStateInput{
        WorkflowState:  workflowState,
        WorkflowConfig: workflowConfig,
        TaskConfig:     input.TaskConfig,
    })
    if err != nil {
        return nil, err
    }

    // Execute aggregate logic with timeout protection
    output, executionError := a.executeAggregateWithTimeout(ctx, input.TaskConfig, workflowState, workflowConfig)
    taskState.Output = output

    // Handle main task response
    response, handleErr := a.taskResponder.HandleMainTask(ctx, &services.MainTaskResponseInput{
        WorkflowConfig: workflowConfig,
        TaskState:      taskState,
        TaskConfig:     input.TaskConfig,
        ExecutionError: executionError,
    })
    if handleErr != nil {
        return nil, handleErr
    }

    // If there was an execution error, the task should be considered failed
    if executionError != nil {
        return response, executionError
    }

    return response, nil
}

func (a *ExecuteAggregate) executeAggregateWithTimeout(
    ctx context.Context,
    taskConfig *task.Config,
    workflowState *workflow.State,
    workflowConfig *workflow.Config,
) (*core.Output, error) {
    // Create timeout context (30 seconds max)
    timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // Execute in goroutine to respect timeout
    resultChan := make(chan struct {
        output *core.Output
        err    error
    }, 1)

    go func() {
        output, err := a.executeAggregate(timeoutCtx, taskConfig, workflowState, workflowConfig)
        resultChan <- struct {
            output *core.Output
            err    error
        }{output, err}
    }()

    select {
    case result := <-resultChan:
        return result.output, result.err
    case <-timeoutCtx.Done():
        return nil, fmt.Errorf("aggregate task execution timed out after 30 seconds")
    }
}

func (a *ExecuteAggregate) executeAggregate(
    ctx context.Context,
    taskConfig *task.Config,
    workflowState *workflow.State,
    workflowConfig *workflow.Config,
) (*core.Output, error) {
    if taskConfig.Outputs == nil || len(*taskConfig.Outputs) == 0 {
        return nil, fmt.Errorf("aggregate task has no outputs defined")
    }

    // Use existing template engine to process outputs
    processedOutputs, err := tplengine.ProcessOutputs(
        ctx,
        taskConfig.Outputs,
        workflowState,
        workflowConfig,
        taskConfig,
    )
    if err != nil {
        return nil, fmt.Errorf("failed to process aggregate outputs: %w", err)
    }

    return processedOutputs, nil
}
```

### Phase 4: Workflow Integration

#### 4.1 Update Workflow Router

**File**: `engine/workflow/activities/exec_task.go` - Add aggregate case to task execution routing

#### 4.2 Register Activity

**File**: `engine/workflow/activities/register.go` - Register the new aggregate activity

### Phase 5: Testing

#### 5.1 Unit Tests

**File**: `engine/task/activities/exec_aggregate_test.go`

- Test successful aggregation
- Test template processing errors
- Test timeout protection
- Test validation errors

#### 5.2 Integration Tests

- Test aggregate task in workflow context
- Test with various template expressions
- Test error handling and recovery

#### 5.3 YAML Test Fixtures

**File**: `engine/task/fixtures/aggregate_task.yaml`

```yaml
id: aggregate-results
type: aggregate

outputs:
    # Simple field mapping
    total_count: "{{ add .tasks.task1.output.count .tasks.task2.output.count }}"

    # Complex aggregation
    summary:
        processed_items: "{{ .tasks.task1.output.count }}"
        failed_items: "{{ .tasks.task2.output.failed_count }}"
        success_rate: "{{ div .tasks.task1.output.count (add .tasks.task1.output.count .tasks.task2.output.failed_count) }}"

    # Conditional logic
    status: "{{ if gt .tasks.task2.output.failed_count 0 }}partial_success{{ else }}success{{ end }}"

    # Array aggregation
    all_results:
        - "{{ .tasks.task1.output }}"
        - "{{ .tasks.task2.output }}"

on_success:
    next: notify-completion

on_error:
    next: handle-aggregation-error
```

## YAML Specification

### Basic Aggregate Task

```yaml
id: combine-results
type: aggregate

outputs:
    total: "{{ add .tasks.count_users.output.count .tasks.count_orders.output.count }}"
    summary: "Found {{ .total }} total items"

on_success:
    next: next-task
```

### Complex Aggregation

```yaml
id: comprehensive-summary
type: aggregate

outputs:
    metrics:
        user_count: "{{ .tasks.count_users.output.count }}"
        order_count: "{{ .tasks.count_orders.output.count }}"
        success_rate: "{{ div .tasks.count_users.output.successful (add .tasks.count_users.output.successful .tasks.count_users.output.failed) }}"

    status: "{{ if gt .tasks.count_users.output.failed 0 }}warning{{ else }}success{{ end }}"

    details:
        timestamp: "{{ now }}"
        processed_by: "{{ .workflow.id }}"
        execution_id: "{{ .workflow.execution_id }}"
```

## Security Considerations

1. **Template Sandboxing**: Reuse existing template engine security
2. **Timeout Protection**: 30-second execution limit
3. **Memory Limits**: Inherit from existing template processing
4. **Input Validation**: Validate template syntax during task validation

## Performance Optimizations

1. **Template Caching**: Leverage existing template compilation caching
2. **Memory Management**: Process templates efficiently without large intermediate objects
3. **Timeout Handling**: Graceful timeout with proper cleanup

## Error Handling

1. **Template Errors**: Clear error messages for template syntax issues
2. **Missing Data**: Graceful handling of missing task outputs
3. **Timeout Errors**: Specific timeout error messages
4. **Validation Errors**: Early validation during task configuration

## Benefits

1. **Simplicity**: No new struct, minimal code changes
2. **Consistency**: Follows existing task patterns
3. **Reusability**: Leverages existing template engine and validation
4. **Performance**: Lightweight execution with timeout protection
5. **Maintainability**: Easy to understand and extend
