# Compozy Hooks Architecture

## Overview

This document provides the detailed technical architecture for implementing a comprehensive hook system in Compozy. The design follows SOLID principles, Clean Architecture patterns, and ensures compatibility with Temporal's deterministic workflow requirements.

## Key Design Decisions (MVP)

1. **Simplified Events**: Start with only 3 events: `task_started`, `task_finished`, `workflow_finished`
2. **All Hooks Async**: Every hook runs asynchronously in Temporal activities to maintain determinism
3. **No Custom Retry**: Leverage Temporal's built-in retry policies instead of custom logic
4. **dispatch_task Always Async**: Task dispatching is fire-and-forget, never blocks workflow
5. **Minimal Configuration**: Simple YAML with just type, on_error strategy, and params

## Architecture Principles

1. **Separation of Concerns**: Hooks are decoupled from core task execution logic
2. **Open/Closed Principle**: New hook types can be added without modifying existing code
3. **Dependency Inversion**: Core depends on abstractions, not concrete implementations
4. **Single Responsibility**: Each component has one clear purpose
5. **Determinism**: All hooks execute in Temporal activities to maintain workflow replay safety

## Core Components

### 1. Domain Layer (`engine/hook/`)

#### Hook Configuration

```go
// engine/hook/config.go
package hook

import (
    "github.com/compozy/compozy/core"
)

// Event represents when a hook should trigger
type Event string

const (
    // MVP Events - Start with minimal set
    EventTaskStarted     Event = "task_started"
    EventTaskFinished    Event = "task_finished"     // Includes status in context
    EventWorkflowFinished Event = "workflow_finished" // Includes final status

    // Future events can be added here based on user needs:
    // EventChildTaskComplete, EventIterationComplete, etc.
)

// ActionType defines the type of action a hook performs
type ActionType string

const (
    ActionTypeLog         ActionType = "log"
    ActionTypeWebhook     ActionType = "webhook"
    ActionTypeMetric      ActionType = "metric"
    ActionTypeDispatchTask ActionType = "dispatch_task"
    ActionTypeTransform   ActionType = "transform"
)

// Action defines a single hook action configuration
type Action struct {
    Type     ActionType     `json:"type" yaml:"type" validate:"required,oneof=log webhook metric dispatch_task transform"`
    OnError  string        `json:"on_error,omitempty" yaml:"on_error,omitempty" validate:"omitempty,oneof=continue fail_task"`
    Params   core.Params   `json:"params" yaml:"params"`
}

// Note: All hooks execute asynchronously in Temporal activities
// Retry logic is handled by Temporal's built-in RetryPolicy

// Config holds all hooks for a task or workflow
type Config map[Event][]Action
```

#### Hook Context

```go
// engine/hook/context.go
package hook

import (
    "github.com/compozy/compozy/engine/task"
    "github.com/compozy/compozy/engine/workflow"
)

// Context provides all data available to hooks during execution
type Context struct {
    Event          Event            `json:"event"`
    WorkflowID     string          `json:"workflow_id"`
    WorkflowExecID string          `json:"workflow_exec_id"`
    TaskID         string          `json:"task_id,omitempty"`
    TaskType       task.Type       `json:"task_type,omitempty"`
    TaskState      *task.State     `json:"task_state,omitempty"`
    Input          core.Params     `json:"input,omitempty"`
    Output         core.Params     `json:"output,omitempty"`
    Error          *core.Error     `json:"error,omitempty"`
    Metadata       core.Params     `json:"metadata,omitempty"`
}

// Builder provides fluent interface for creating contexts
type ContextBuilder struct {
    ctx Context
}

func NewContextBuilder(event Event) *ContextBuilder {
    return &ContextBuilder{
        ctx: Context{Event: event},
    }
}

func (b *ContextBuilder) WithWorkflow(id, execID string) *ContextBuilder {
    b.ctx.WorkflowID = id
    b.ctx.WorkflowExecID = execID
    return b
}

func (b *ContextBuilder) WithTask(state *task.State) *ContextBuilder {
    if state != nil {
        b.ctx.TaskID = state.TaskID
        b.ctx.TaskType = state.Type
        b.ctx.TaskState = state
        b.ctx.Input = state.Input
        b.ctx.Output = state.Output
    }
    return b
}

func (b *ContextBuilder) WithError(err *core.Error) *ContextBuilder {
    b.ctx.Error = err
    return b
}

func (b *ContextBuilder) Build() Context {
    return b.ctx
}
```

### 2. Executor Layer (`engine/hook/executor/`)

#### Hook Executor Interface

```go
// engine/hook/executor/executor.go
package executor

import (
    "context"
    "github.com/compozy/compozy/engine/hook"
)

// Executor handles hook execution within workflows
type Executor interface {
    // ExecuteHooks runs all hooks for the given event asynchronously in activities
    // This method MUST be called from within a workflow context
    ExecuteHooks(ctx workflow.Context, event hook.Event, hookCtx hook.Context, config hook.Config) error
}

// Result represents the outcome of a hook execution
type Result struct {
    Event     hook.Event     `json:"event"`
    Action    hook.Action    `json:"action"`
    Success   bool          `json:"success"`
    Error     error         `json:"error,omitempty"`
    StartTime int64         `json:"start_time"`
    EndTime   int64         `json:"end_time"`
    Duration  int64         `json:"duration_ms"`
}
```

#### Default Implementation

```go
// engine/hook/executor/default.go
package executor

import (
    "context"
    "fmt"
    "time"

    "github.com/compozy/compozy/engine/hook"
    "github.com/compozy/compozy/engine/hook/sideeffect"
    "github.com/compozy/compozy/pkg/logger"
    "go.temporal.io/sdk/workflow"
)

type defaultExecutor struct {
    registry sideeffect.Registry
    logger   logger.Logger
}

func New(registry sideeffect.Registry, logger logger.Logger) Executor {
    return &defaultExecutor{
        registry: registry,
        logger:   logger,
    }
}

func (e *defaultExecutor) ExecuteHooks(ctx workflow.Context, event hook.Event, hookCtx hook.Context, config hook.Config) error {
    actions, ok := config[event]
    if !ok || len(actions) == 0 {
        return nil
    }

    // All hooks run asynchronously in activities to maintain determinism
    for _, action := range actions {
        e.executeHookAsync(ctx, event, hookCtx, action)
    }

    return nil
}

func (e *defaultExecutor) executeHookAsync(ctx workflow.Context, event hook.Event, hookCtx hook.Context, action hook.Action) {
    // All hooks run asynchronously in activities for determinism
    workflow.Go(ctx, func(ctx workflow.Context) {
        activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
            StartToCloseTimeout: 30 * time.Second,
            RetryPolicy: &temporal.RetryPolicy{
                MaximumAttempts: 3,
                InitialInterval: time.Second,
                BackoffCoefficient: 2.0,
                MaximumInterval: 100 * time.Second,
            },
        })

        var result Result
        err := workflow.ExecuteActivity(activityCtx, ExecuteHookActivity, event, hookCtx, action).Get(ctx, &result)
        if err != nil {
            // Use Temporal's deterministic logger
            workflow.GetLogger(ctx).Error("Hook execution failed",
                "event", event,
                "action", action.Type,
                "error", err)

            // Only fail the workflow if on_error is "fail_task"
            if action.OnError == "fail_task" {
                // Signal the main workflow about the failure
                // This would need to be implemented based on your workflow design
            }
        }
    })
}

// ExecuteHookActivity is the actual activity that runs the hook
func ExecuteHookActivity(ctx context.Context, event hook.Event, hookCtx hook.Context, action hook.Action) (*Result, error) {
    // This runs in an activity worker, so it's safe to do I/O
    registry := GetSideEffectRegistry() // Singleton or injected

    sideEffect, err := registry.Get(action.Type)
    if err != nil {
        return nil, fmt.Errorf("side effect not found: %w", err)
    }

    start := time.Now()
    err = sideEffect.Execute(ctx, hookCtx, action)
    duration := time.Since(start)

    result := &Result{
        Event:     event,
        Action:    action,
        Success:   err == nil,
        Error:     err,
        StartTime: start.UnixMilli(),
        EndTime:   time.Now().UnixMilli(),
        Duration:  duration.Milliseconds(),
    }

    // Log to external system (safe in activity)
    logger.Info("Hook executed",
        "event", event,
        "action", action.Type,
        "duration_ms", duration.Milliseconds(),
        "success", err == nil)

    return result, err
}
```

### 3. SideEffect Layer (`engine/hook/sideeffect/`)

#### SideEffect Interface

```go
// engine/hook/sideeffect/sideeffect.go
package sideeffect

import (
    "context"
    "github.com/compozy/compozy/engine/hook"
)

// SideEffect executes a specific type of hook action
type SideEffect interface {
    // Execute performs the hook action
    Execute(ctx context.Context, hookCtx hook.Context, action hook.Action) error

    // Type returns the action type this side effect supports
    Type() hook.ActionType
}

// Registry manages hook side effects
type Registry interface {
    // Register adds a side effect to the registry
    Register(sideEffect SideEffect) error

    // Get retrieves a side effect by action type
    Get(actionType hook.ActionType) (SideEffect, error)
}
```

#### Log SideEffect Example

```go
// engine/hook/sideeffect/log.go
package sideeffect

import (
    "context"
    "fmt"

    "github.com/compozy/compozy/engine/hook"
    "github.com/compozy/compozy/pkg/logger"
    "github.com/compozy/compozy/pkg/tplengine"
)

type logSideEffect struct {
    logger    logger.Logger
    evaluator tplengine.Evaluator
}

func NewLogSideEffect(logger logger.Logger, evaluator tplengine.Evaluator) SideEffect {
    return &logSideEffect{
        logger:    logger,
        evaluator: evaluator,
    }
}

func (h *logSideEffect) Type() hook.ActionType {
    return hook.ActionTypeLog
}

func (h *logSideEffect) Execute(ctx context.Context, hookCtx hook.Context, action hook.Action) error {
    level, _ := action.Params["level"].(string)
    if level == "" {
        level = "info"
    }

    messageTemplate, _ := action.Params["message"].(string)
    if messageTemplate == "" {
        return fmt.Errorf("log side effect requires 'message' parameter")
    }

    // Evaluate template with hook context
    message, err := h.evaluator.EvaluateString(messageTemplate, map[string]any{
        "event":    hookCtx.Event,
        "workflow": map[string]any{
            "id":      hookCtx.WorkflowID,
            "exec_id": hookCtx.WorkflowExecID,
        },
        "task": map[string]any{
            "id":     hookCtx.TaskID,
            "type":   hookCtx.TaskType,
            "state":  hookCtx.TaskState,
        },
        "input":  hookCtx.Input,
        "output": hookCtx.Output,
        "error":  hookCtx.Error,
    })
    if err != nil {
        return fmt.Errorf("failed to evaluate message template: %w", err)
    }

    // Log with appropriate level
    switch level {
    case "debug":
        h.logger.Debug(message, "hook_event", hookCtx.Event)
    case "info":
        h.logger.Info(message, "hook_event", hookCtx.Event)
    case "warn":
        h.logger.Warn(message, "hook_event", hookCtx.Event)
    case "error":
        h.logger.Error(message, "hook_event", hookCtx.Event)
    default:
        h.logger.Info(message, "hook_event", hookCtx.Event)
    }

    return nil
}
```

#### Dispatch Task SideEffect Example

```go
// engine/hook/sideeffect/dispatch_task.go
package sideeffect

import (
    "context"
    "fmt"

    "github.com/compozy/compozy/engine/hook"
    "github.com/compozy/compozy/engine/task"
    "github.com/compozy/compozy/pkg/tplengine"
    "go.temporal.io/sdk/workflow"
)

type dispatchTaskSideEffect struct {
    evaluator tplengine.Evaluator
}

func NewDispatchTaskSideEffect(evaluator tplengine.Evaluator) SideEffect {
    return &dispatchTaskSideEffect{
        evaluator: evaluator,
    }
}

func (d *dispatchTaskSideEffect) Type() hook.ActionType {
    return hook.ActionTypeDispatchTask
}

func (d *dispatchTaskSideEffect) Execute(ctx context.Context, hookCtx hook.Context, action hook.Action) error {
    taskID, ok := action.Params["task_id"].(string)
    if !ok || taskID == "" {
        return fmt.Errorf("dispatch_task side effect requires 'task_id' parameter")
    }

    // Evaluate task ID template
    evaluatedTaskID, err := d.evaluator.EvaluateString(taskID, map[string]any{
        "event":    hookCtx.Event,
        "workflow": map[string]any{
            "id":      hookCtx.WorkflowID,
            "exec_id": hookCtx.WorkflowExecID,
        },
        "task":   hookCtx.TaskState,
        "input":  hookCtx.Input,
        "output": hookCtx.Output,
    })
    if err != nil {
        return fmt.Errorf("failed to evaluate task_id template: %w", err)
    }

    // Prepare input for the dispatched task
    inputData := d.prepareTaskInput(action.Params["input"], hookCtx)

    // IMPORTANT: dispatch_task is always async and non-blocking
    // It queues a new task execution without waiting for completion

    // Option 1: Use Temporal's SignalWithStart to trigger a new workflow
    c := client.GetTemporalClient() // Get Temporal client from context/registry
    workflowID := fmt.Sprintf("%s-dispatched-%s-%d",
        hookCtx.WorkflowID, evaluatedTaskID, time.Now().Unix())

    _, err = c.SignalWithStartWorkflow(ctx, workflowID, "dispatch-task-signal",
        nil, // signal arg
        client.StartWorkflowOptions{
            TaskQueue: "compozy-tasks",
            WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
        },
        "CompozyTaskWorkflow", // Workflow type for individual tasks
        map[string]any{
            "task_id": evaluatedTaskID,
            "input":   inputData,
            "parent_workflow_id": hookCtx.WorkflowID,
        },
    )

    if err != nil {
        // Log the error but don't fail - dispatch_task is fire-and-forget
        logger.Warn("Failed to dispatch task",
            "task_id", evaluatedTaskID,
            "error", err)
    }

    logger.Info("Task dispatched successfully",
        "task_id", evaluatedTaskID,
        "workflow_id", workflowID)

    return nil // Always return nil - dispatch_task doesn't block
}
```

### 4. Integration Layer

#### Task Integration

```go
// Modification to engine/worker/acts_task.go - Execute hooks from workflow
func (e *TaskExecutor) HandleExecution(ctx workflow.Context, taskConfig *task.Config) (task.Response, error) {
    // Build initial hook context
    hookCtx := hook.NewContextBuilder(hook.EventTaskStarted).
        WithWorkflow(workflow.GetInfo(ctx).WorkflowExecution.ID, workflow.GetInfo(ctx).WorkflowExecution.RunID).
        WithTask(&task.State{
            TaskID: taskConfig.ID,
            Type:   taskConfig.Type,
        }).
        Build()

    // Execute task_started hooks asynchronously
    e.hookExecutor.ExecuteHooks(ctx, hook.EventTaskStarted, hookCtx, taskConfig.Hooks)

    // Execute the actual task (existing logic)
    var taskResp task.Response
    var taskErr error

    switch taskConfig.Type {
    case task.TypeBasic:
        taskResp, taskErr = e.executeBasicTask(ctx, taskConfig)
    case task.TypeRouter:
        taskResp, taskErr = e.executeRouterTask(ctx, taskConfig)
    // ... other task types
    }

    // Build completion hook context
    completionHookCtx := hook.NewContextBuilder(hook.EventTaskFinished).
        WithWorkflow(workflow.GetInfo(ctx).WorkflowExecution.ID, workflow.GetInfo(ctx).WorkflowExecution.RunID).
        WithTask(&task.State{
            TaskID: taskConfig.ID,
            Type:   taskConfig.Type,
            Status: determineStatus(taskErr),
            Output: taskResp.Output,
            Error:  taskErr,
        }).
        Build()

    // Execute task_finished hooks asynchronously
    e.hookExecutor.ExecuteHooks(ctx, hook.EventTaskFinished, completionHookCtx, taskConfig.Hooks)

    return taskResp, taskErr
}
```

#### Workflow Integration

```go
// Modification to engine/worker/workflows.go
func (w *CompozyWorkflow) Execute(ctx workflow.Context, input WorkflowInput) (*WorkflowOutput, error) {
    // Initialize workflow
    manager, err := w.InitManager(ctx, input)
    if err != nil {
        return nil, err
    }

    // Execute workflow tasks...
    output, err := w.executeTasks(ctx, manager)

    // Execute workflow_finished hooks with final status
    finalStatus := core.StatusSuccess
    if err != nil {
        finalStatus = core.StatusFailed
    }

    hookCtx := hook.NewContextBuilder(hook.EventWorkflowFinished).
        WithWorkflow(input.WorkflowID, workflow.GetInfo(ctx).WorkflowExecution.ID).
        WithMetadata(map[string]any{
            "status": finalStatus,
            "error":  err,
            "output": output,
        }).
        Build()

    // Execute hooks asynchronously - don't wait for completion
    w.hookExecutor.ExecuteHooks(ctx, hook.EventWorkflowFinished, hookCtx, manager.Config.Hooks)

    return output, err
}
```

### 5. Security Layer

#### Webhook URL Validation

```go
// engine/hook/security/validator.go
package security

import (
    "fmt"
    "net/url"
    "strings"
)

type URLValidator struct {
    allowedHosts []string
    allowedPaths []string
}

func (v *URLValidator) Validate(rawURL string) error {
    u, err := url.Parse(rawURL)
    if err != nil {
        return fmt.Errorf("invalid URL: %w", err)
    }

    // Check scheme
    if u.Scheme != "https" && u.Scheme != "http" {
        return fmt.Errorf("unsupported scheme: %s", u.Scheme)
    }

    // Check host whitelist
    hostAllowed := false
    for _, allowed := range v.allowedHosts {
        if strings.HasSuffix(u.Host, allowed) {
            hostAllowed = true
            break
        }
    }

    if !hostAllowed {
        return fmt.Errorf("host not whitelisted: %s", u.Host)
    }

    return nil
}
```

## Configuration Examples

### Basic Task with Lifecycle Hooks

```yaml
id: process_order
type: basic
action:
    ref: validate_order
hooks:
    task_started:
        - type: log
          params:
              level: info
              message: "Processing order {{ input.order_id }}"
        - type: metric
          params:
              name: order_processing_started
              value: 1
              tags:
                  customer_type: "{{ input.customer_type }}"
    task_finished:
        - type: webhook
          on_error: continue
          params:
              url: "${WEBHOOK_BASE_URL}/orders/complete"
              method: POST
              headers:
                  Content-Type: application/json
                  X-API-Key: "${WEBHOOK_API_KEY}"
              body:
                  order_id: "{{ input.order_id }}"
                  status: "{{ task.status }}"
                  total: "{{ output.total_amount }}"
        - type: dispatch_task
          params:
              task_id: "send_order_notification"
              input:
                  order_id: "{{ input.order_id }}"
                  customer_email: "{{ input.customer_email }}"
                  status: "{{ task.status }}"
```

### Simple Workflow Example

```yaml
id: order_workflow
name: Order Processing Workflow
tasks:
    - id: validate_order
      type: basic
      hooks:
          task_finished:
              - type: log
                params:
                    level: info
                    message: "Order validation {{ task.status }}: {{ input.order_id }}"
    - id: process_payment
      type: basic
      hooks:
          task_started:
              - type: metric
                params:
                    name: payment_processing_started
          task_finished:
              - type: dispatch_task
                on_error: continue
                params:
                    task_id: "audit_payment"
                    input:
                        order_id: "{{ input.order_id }}"
                        amount: "{{ output.amount }}"
                        status: "{{ task.status }}"
hooks:
    workflow_finished:
        - type: webhook
          on_error: continue
          params:
              url: "${WEBHOOK_BASE_URL}/workflow/complete"
              body:
                  workflow_id: "{{ workflow.id }}"
                  status: "{{ workflow.status }}"
```

### Future Enhancement Example

```yaml
# This shows how hooks could be extended in future phases
id: advanced_task
type: collection
hooks:
    task_started:
        - type: log
          params:
              level: info
              message: "Starting batch processing"
    task_finished:
        - type: metric
          params:
              name: batch_processed
              tags:
                  status: "{{ task.status }}"
                  items_count: "{{ output.processed_count }}"
    # Future hooks that could be added:
    # on_iteration_start:
    # on_iteration_complete:
    # on_child_task_complete:
items:
    source: "{{ input.items }}"
subtask:
    type: basic
    action:
        ref: process_single_item
```

### Collection Task with Dynamic Processing

```yaml
id: process_documents
type: collection
items:
    source: "{{ input.documents }}"
hooks:
    on_iteration_complete:
        - type: dispatch_task
          params:
              task_id: "index_document"
              input:
                  doc_id: "{{ item.id }}"
                  status: "{{ metadata.iteration_result.status }}"
        - type: log
          params:
              level: info
              message: "Processed document {{ item.id }} with status {{ metadata.iteration_result.status }}"
subtask:
    type: basic
    action:
        ref: analyze_document
```

## Implementation Phases

### Phase 1: MVP Core (Week 1-2)

- [ ] Create hook domain models with simplified events
- [ ] Implement HookExecutor with all hooks in activities
- [ ] Add log and metric side effects
- [ ] Integrate task_started/task_finished hooks
- [ ] Add workflow_finished hook support
- [ ] Unit tests with Temporal TestWorkflowEnvironment

### Phase 2: Essential SideEffects (Week 3)

- [ ] Implement webhook side effect with Temporal retry
- [ ] Add dispatch_task side effect (async only)
- [ ] Security layer with URL whitelisting
- [ ] Integration tests

### Phase 3: Polish & Performance (Week 4)

- [ ] Add transform side effect with sandboxing
- [ ] Performance optimizations (batching, caching)
- [ ] Monitoring and metrics
- [ ] Documentation and examples
- [ ] End-to-end tests

### Future Phases (Based on User Feedback)

- [ ] Additional hook events (child_complete, iteration_complete, etc.)
- [ ] Hook composition and chaining
- [ ] Advanced routing and conditional hooks
- [ ] External hook registry support

## Testing Strategy

### Unit Tests

- Test each side effect in isolation
- Test hook executor with mocked side effects
- Test context builders and validators
- Test error handling strategies

### Integration Tests

- Test hooks within Temporal activities
- Test async hook execution
- Test error propagation
- Test template evaluation

### End-to-End Tests

- Test complete workflows with hooks
- Test hook failures and recovery
- Test performance impact
- Test monitoring and observability

## Monitoring and Observability

### Metrics

- Hook execution count by event and action type
- Hook execution duration percentiles
- Hook failure rate
- Async hook queue depth

### Logs

- Structured logs for all hook executions
- Error logs with full context
- Debug logs for troubleshooting
- Audit logs for security events

### Tracing

- Distributed tracing for hook execution
- Parent-child span relationships
- Hook execution within task spans
- External call tracing

## Security Considerations

1. **Input Validation**: All hook parameters must be validated
2. **URL Whitelisting**: Only approved endpoints for webhooks
3. **Secret Management**: Use environment variables, never hardcode
4. **Rate Limiting**: Prevent DoS through excessive hook calls
5. **Sandboxing**: Transform hooks run in restricted environment
6. **Audit Trail**: Log all hook executions for compliance

## Performance Optimizations

1. **Batching**: Group multiple async hooks into single activity
2. **Caching**: Cache parsed hook configurations
3. **Connection Pooling**: Reuse HTTP connections for webhooks
4. **Timeout Management**: Strict timeouts for all external calls
5. **Circuit Breaking**: Prevent cascading failures

## Migration Guide

### For Existing Workflows

1. Existing workflows continue to work without hooks
2. Hooks are opt-in per task configuration
3. No breaking changes to current behavior
4. Gradual adoption recommended

### Best Practices

1. Start with logging hooks for observability
2. Add metrics for key business events
3. Use async for non-critical webhooks
4. Keep hook logic simple and focused
5. Test hooks thoroughly before production

## Conclusion

This architecture provides a robust, extensible, and secure hook system for Compozy that enhances the workflow engine's capabilities while maintaining its core principles of determinism and reliability. The phased implementation approach ensures we can deliver value incrementally while maintaining system stability.
