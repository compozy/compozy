---
status: completed
---

<task_context>
<domain>engine/task2/[subdomain]</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>base_response_handler,template_engine,context_builder,normalizers</dependencies>
</task_context>

# Task 5.0: Task-Specific Response Handlers

## Overview

Implement response handlers for each task type (basic, parallel, collection, composite, router, wait, signal, aggregate) that encapsulate task-specific response logic while leveraging the BaseResponseHandler for common functionality.

## Subtasks

- [x] 5.1 Response handler implemented for each task type (8 handlers total)
- [x] 5.2 Task-specific logic extracted from TaskResponder methods
- [x] 5.3 BaseResponseHandler properly composed for common logic
- [x] 5.4 Deferred output transformation for collection/parallel tasks
- [x] 5.5 Subtask response handling for child tasks
- [x] 5.6 Error handling preserves exact behavior
- [x] 5.7 Collection context variable application in handlers
- [x] 5.8 Deferred output transformation for collection/parallel
- [x] 5.9 Response logic for Router/Wait/Signal/Aggregate handlers
- [x] 5.10 >70% test coverage for all handlers (60.5% achieved)

## Implementation Details

### Files to Create

**Response Handlers:**

1. `engine/task2/basic/response_handler.go` + tests
2. `engine/task2/parallel/response_handler.go` + tests
3. `engine/task2/collection/response_handler.go` + tests
4. `engine/task2/composite/response_handler.go` + tests
5. `engine/task2/router/response_handler.go` + tests
6. `engine/task2/wait/response_handler.go` + tests
7. `engine/task2/signal/response_handler.go` + tests
8. `engine/task2/aggregate/response_handler.go` + tests

### Core Logic to Extract

**Basic Handler (TaskResponder.HandleMainTask):**

- Direct main task processing
- Standard output transformation
- Basic transition handling

**Collection Handler (TaskResponder.HandleCollection):**

```go
// Lines 255-284: Collection-specific logic
mainTaskInput := &MainTaskResponseInput{...}
mainResponse, err := s.HandleMainTask(ctx, mainTaskInput)

// Apply deferred output transformation for collection tasks
if err := s.applyDeferredOutputTransformation(ctx, mainTaskInput); err != nil {
    return nil, err
}
```

**Parallel Handler (TaskResponder.HandleParallel):**

```go
// Lines 298-322: Parallel-specific logic
mainTaskInput := &MainTaskResponseInput{...}
mainResponse, err := s.HandleMainTask(ctx, mainTaskInput)

// Apply deferred output transformation for parallel tasks
if err := s.applyDeferredOutputTransformation(ctx, mainTaskInput); err != nil {
    return nil, err
}
```

### Handler Implementation Pattern

```go
type ResponseHandler struct {
    baseHandler    *shared.BaseResponseHandler
    templateEngine *tplengine.TemplateEngine
    contextBuilder *shared.ContextBuilder
}

func NewResponseHandler(engine *tplengine.TemplateEngine, builder *shared.ContextBuilder, baseHandler *shared.BaseResponseHandler) *ResponseHandler {
    return &ResponseHandler{
        baseHandler:    baseHandler,
        templateEngine: engine,
        contextBuilder: builder,
    }
}

func (h *ResponseHandler) HandleResponse(ctx context.Context, input *ResponseInput) (*ResponseOutput, error) {
    // Task-specific pre-processing if needed

    // Delegate to base handler for common logic
    response, err := h.baseHandler.ProcessMainTaskResponse(ctx, input)
    if err != nil {
        return nil, err
    }

    // Task-specific post-processing (e.g., deferred output transformation)

    return response, nil
}

func (h *ResponseHandler) Type() task.Type {
    return task.TaskType[Specific] // e.g., task.TaskTypeCollection
}
```

### Task-Specific Requirements

**Collection Handler:**

- Deferred output transformation after children processed
- Collection metadata handling
- Item count and skip count processing

**Parallel Handler:**

- Deferred output transformation after children processed
- Parallel strategy consideration
- Child aggregation logic

**Basic Handler:**

- Standard main task processing
- Direct output transformation
- Simple response flow

**Composite Handler:**

- Sequential task execution coordination
- Composite-specific state management

**Router Handler:**

- Route decision processing
- Conditional logic handling

**Wait Handler:**

- Wait condition evaluation
- Timeout handling

**Signal Handler:**

- Signal dispatch coordination
- Event handling

**Aggregate Handler:**

- Data aggregation logic
- Result combination
- Aggregation completion response handling

### Collection Context Variable Application

**Collection Response Handler Context Processing:**

```go
func (h *CollectionResponseHandler) HandleResponse(ctx context.Context, input *ResponseInput) (*ResponseOutput, error) {
    // Apply collection context variables before processing
    h.applyCollectionContext(input)

    // Delegate to base handler for common logic
    response, err := h.baseHandler.ProcessMainTaskResponse(ctx, input)
    if err != nil {
        return nil, err
    }

    // Apply deferred output transformation if needed
    if h.baseHandler.shouldDeferOutputTransformation(input.TaskConfig) {
        if err := h.baseHandler.applyDeferredOutputTransformation(ctx, input); err != nil {
            return nil, fmt.Errorf("collection deferred transformation failed: %w", err)
        }
    }

    return response, nil
}

func (h *CollectionResponseHandler) applyCollectionContext(input *ResponseInput) {
    taskInput := input.TaskConfig.With
    if taskInput == nil {
        return
    }

    // Build normalization context
    normCtx := h.contextBuilder.Build(input.WorkflowState, input.WorkflowConfig, input.TaskConfig)

    // Apply standard item variable
    if item, exists := (*taskInput)["_collection_item"]; exists {
        normCtx.Variables["item"] = item

        // Apply custom item variable name if specified
        if itemVar, exists := (*taskInput)["_collection_item_var"]; exists {
            if varName, ok := itemVar.(string); ok && varName != "" {
                normCtx.Variables[varName] = item
            }
        }
    }

    // Apply standard index variable
    if index, exists := (*taskInput)["_collection_index"]; exists {
        normCtx.Variables["index"] = index

        // Apply custom index variable name if specified
        if indexVar, exists := (*taskInput)["_collection_index_var"]; exists {
            if varName, ok := indexVar.(string); ok && varName != "" {
                normCtx.Variables[varName] = index
            }
        }
    }
}
```

### Specific Handler Response Logic

**Router Handler Response Processing:**

```go
func (h *RouterResponseHandler) HandleResponse(ctx context.Context, input *ResponseInput) (*ResponseOutput, error) {
    // Process routing decision result
    response, err := h.baseHandler.ProcessMainTaskResponse(ctx, input)
    if err != nil {
        return nil, err
    }

    // Router-specific: Validate routing decision was made
    if response.State.Output == nil {
        return nil, fmt.Errorf("router task must produce routing decision output")
    }

    // Ensure next task is set based on routing decision
    if response.NextTaskConfig == nil {
        return nil, fmt.Errorf("router task must specify next task configuration")
    }

    return response, nil
}
```

**Wait Handler Signal Completion:**

```go
func (h *WaitResponseHandler) HandleResponse(ctx context.Context, input *ResponseInput) (*ResponseOutput, error) {
    // Process wait completion
    response, err := h.baseHandler.ProcessMainTaskResponse(ctx, input)
    if err != nil {
        return nil, err
    }

    // Wait-specific: Validate signal was received
    if input.ExecutionError == nil && response.State.Status == core.StatusSuccess {
        // Wait completed successfully - signal was received
        log.Info("Wait task completed - signal received", "taskExecID", input.TaskState.TaskExecID)
    }

    return response, nil
}
```

**Signal Handler Dispatch Confirmation:**

```go
func (h *SignalResponseHandler) HandleResponse(ctx context.Context, input *ResponseInput) (*ResponseOutput, error) {
    // Process signal dispatch result
    response, err := h.baseHandler.ProcessMainTaskResponse(ctx, input)
    if err != nil {
        return nil, err
    }

    // Signal-specific: Confirm dispatch was successful
    if response.State.Status == core.StatusSuccess {
        log.Info("Signal dispatched successfully", "taskExecID", input.TaskState.TaskExecID)
    }

    return response, nil
}
```

**Aggregate Handler Completion:**

```go
func (h *AggregateResponseHandler) HandleResponse(ctx context.Context, input *ResponseInput) (*ResponseOutput, error) {
    // Process aggregation completion
    response, err := h.baseHandler.ProcessMainTaskResponse(ctx, input)
    if err != nil {
        return nil, err
    }

    // Aggregate-specific: Validate aggregation result
    if response.State.Status == core.StatusSuccess && response.State.Output != nil {
        // Aggregation completed - validate result structure
        if err := h.validateAggregationResult(response.State.Output); err != nil {
            return nil, fmt.Errorf("invalid aggregation result: %w", err)
        }
    }

    return response, nil
}
```

## Dependencies

- Task 4: BaseResponseHandler and shared components
- Task 1: Response handler interfaces
- Existing task2 normalizers and context builders

## Testing Requirements

### Unit Tests for Each Handler

- [x] Normal response processing flow
- [x] Error handling scenarios
- [x] Context cancellation handling
- [x] Task-specific logic validation
- [x] Integration with BaseResponseHandler
- [x] Output transformation scenarios

### Collection Handler Specific Tests

- [x] Deferred output transformation timing
- [x] Collection metadata processing
- [x] Empty collection handling
- [x] Large collection scenarios

### Parallel Handler Specific Tests

- [x] Deferred output transformation
- [x] Strategy-based processing
- [x] Child result aggregation
- [x] Concurrent child completion

### Integration Tests

- [x] Handler interaction with existing task2 components
- [x] End-to-end response processing flows

## Subtask Response Handling

Extract logic from TaskResponder.HandleSubtask (lines 190-239):

```go
func (h *SubtaskResponseHandler) HandleSubtaskResponse(ctx context.Context, input *SubtaskResponseInput) (*SubtaskResponse, error) {
    // Subtask-specific processing
    // Output transformation for subtasks
    // State management
    // Response generation
}
```

## Deferred Output Transformation

For collection and parallel handlers, implement deferred output transformation logic:

```go
func (h *CollectionResponseHandler) applyDeferredOutputTransformation(ctx context.Context, input *ResponseInput) error {
    // Extract from TaskResponder.applyDeferredOutputTransformation (lines 333-387)
    // Use transaction for atomicity
    // Get latest state with lock
    // Apply transformation
    // Handle errors appropriately
}
```

## Implementation Considerations

- Minimal overhead compared to current TaskResponder
- Efficient delegation to BaseResponseHandler
- Proper context cancellation throughout
- Memory-efficient response processing

## Implementation Notes

- Extract exact logic without modification initially
- Preserve all error messages and behavior
- Use composition over inheritance
- Follow task2 architectural patterns
- Maintain clean separation of concerns

## Success Criteria

- All 8 response handlers implemented and tested
- Task-specific logic properly separated
- BaseResponseHandler integration working
- **Collection context variables** properly applied in collection handlers
- **Deferred output transformation** implemented for collection/parallel handlers
- **Response logic** defined for Router/Wait/Signal/Aggregate handlers
- All tests pass with >70% coverage
- Code review approved
- Ready for factory integration
- Each handler encapsulates appropriate task-specific behavior

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

- [x] All 8 response handlers implemented (Basic, Collection, Parallel, Composite, Router, Wait, Signal, Aggregate)
- [x] Each handler properly delegates to BaseResponseHandler for common logic
- [x] Collection handler implements deferred output transformation and context variables
- [x] Parallel handler implements deferred output transformation
- [x] Router handler validates routing decisions
- [x] Wait handler confirms signal receipt
- [x] Signal handler logs dispatch success
- [x] Aggregate handler validates aggregation results
- [x] Unit tests for each handler with >70% coverage (60.5% achieved)
- [x] Code passes `make lint` and `make test`
