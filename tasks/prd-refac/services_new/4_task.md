---
status: completed
---

<task_context>
<domain>engine/task2/shared</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>template_engine,context_builder,workflow_repository,task_repository</dependencies>
</task_context>

# Task 4.0: Response Handler Foundation

## Overview

Create the foundation for response handlers by implementing the BaseResponseHandler with common logic and establishing the response handler framework. This sets up the infrastructure needed for task-specific response handlers.

## Subtasks

- [x] 4.1 BaseResponseHandler implemented with common response logic
- [x] 4.2 Response handler framework established following task2 patterns
- [x] 4.3 Common logic extracted from TaskResponder.HandleMainTask
- [x] 4.4 Error handling and context management standardized
- [x] 4.5 Parent status update integration working
- [x] 4.6 Output transformation support implemented
- [x] 4.7 Transaction safety patterns with row-level locking
- [x] 4.8 Deferred output transformation system
- [x] 4.9 >70% test coverage for base functionality

## Implementation Details

### Files to Create

1. `engine/task2/shared/base_response_handler.go` - Base handler implementation
2. `engine/task2/shared/base_response_handler_test.go` - Comprehensive tests
3. `engine/task2/shared/parent_status_manager.go` - Parent status logic
4. `engine/task2/shared/parent_status_manager_test.go` - Parent status tests

### Core Logic to Extract

**From TaskResponder.HandleMainTask (lines 66-101):**

```go
// Process task execution result
isSuccess, executionErr := s.processTaskExecutionResult(ctx, input)

// Save state and handle context cancellation
if err := s.saveTaskState(ctx, input.TaskState); err != nil {
    if ctx.Err() != nil {
        return &task.MainTaskResponse{State: input.TaskState}, nil
    }
    return nil, err
}

// Update parent status and handle context cancellation
s.logParentStatusUpdateError(ctx, input.TaskState)

// Process transitions and validate error handling
onSuccess, onError, err := s.processTransitions(ctx, input, isSuccess, executionErr)

// Determine next task
nextTask := s.determineNextTask(input, isSuccess)
```

### BaseResponseHandler Structure

```go
type BaseResponseHandler struct {
    templateEngine   *tplengine.TemplateEngine
    contextBuilder   *shared.ContextBuilder
    parentStatusMgr  ParentStatusManager
    workflowRepo     workflow.Repository
    taskRepo         task.Repository
}

func (h *BaseResponseHandler) ProcessMainTaskResponse(ctx context.Context, input *ResponseInput) (*ResponseOutput, error) {
    // 1. Process task execution result
    // 2. Save task state with cancellation handling
    // 3. Update parent status if needed
    // 4. Process transitions (success/error)
    // 5. Determine next task
    // 6. Return standardized response
}
```

### Key Methods to Implement

**BaseResponseHandler:**

- `ProcessMainTaskResponse()` - Main processing logic with transaction safety
- `processTaskExecutionResult()` - Result evaluation
- `saveTaskState()` - State persistence with cancellation and locking
- `processTransitions()` - Transition normalization
- `determineNextTask()` - Next task selection
- `applyOutputTransformation()` - Output processing
- `shouldDeferOutputTransformation()` - Determines if transformation should be deferred
- `applyDeferredOutputTransformation()` - Transaction-safe deferred transformation

**ParentStatusManager:**

- `UpdateParentStatus()` - Parent status coordination
- `extractParallelStrategy()` - Strategy extraction
- `updateParentStatusIfNeeded()` - Conditional updates

### Transaction Safety Implementation

**Row-Level Locking Pattern:**

```go
func (h *BaseResponseHandler) saveTaskStateWithLocking(ctx context.Context, state *task.State) error {
    return h.taskRepo.WithTx(ctx, func(tx pgx.Tx) error {
        // Get latest state with row-level lock to prevent concurrent modifications
        latestState, err := h.taskRepo.GetStateForUpdate(ctx, tx, state.TaskExecID)
        if err != nil {
            return fmt.Errorf("failed to lock state for update: %w", err)
        }

        // Apply changes to latest state
        latestState.Status = state.Status
        latestState.Output = state.Output
        latestState.Error = state.Error

        // Save with transaction safety
        return h.taskRepo.UpsertState(ctx, latestState)
    })
}
```

**Deferred Output Transformation System:**

```go
func (h *BaseResponseHandler) shouldDeferOutputTransformation(taskConfig *task.Config) bool {
    return taskConfig.Type == task.TaskTypeCollection || taskConfig.Type == task.TaskTypeParallel
}

func (h *BaseResponseHandler) applyDeferredOutputTransformation(ctx context.Context, input *ResponseInput) error {
    // Transaction-safe transformation AFTER children complete
    return h.taskRepo.WithTx(ctx, func(tx pgx.Tx) error {
        // Get latest state with lock for atomic transformation
        latestState, err := h.taskRepo.GetStateForUpdate(ctx, tx, input.TaskState.TaskExecID)
        if err != nil {
            return fmt.Errorf("failed to lock state for deferred transformation: %w", err)
        }

        // Apply output transformation using template engine
        transformedOutput, err := h.outputTransformer.Transform(ctx, latestState, input.WorkflowConfig)
        if err != nil {
            return fmt.Errorf("deferred output transformation failed: %w", err)
        }

        latestState.Output = transformedOutput
        return h.taskRepo.UpsertState(ctx, latestState)
    })
}
```

## Dependencies

- Task 1: Interfaces and shared types
- Existing task2 normalizers (success/error transitions, output transformer)
- Existing workflow and task repositories
- PostgreSQL transaction support (pgx.Tx)
- Row-level locking capabilities

## Testing Requirements

### BaseResponseHandler Tests

- [ ] Successful task processing flow
- [ ] Task failure handling
- [ ] Context cancellation during state save
- [ ] Context cancellation during parent update
- [ ] Transition processing (success/error scenarios)
- [ ] Output transformation application
- [ ] Next task determination logic

### ParentStatusManager Tests

- [ ] Parent status updates for parallel tasks
- [ ] Strategy extraction from parallel config
- [ ] Conditional update logic (parent type checking)
- [ ] Error handling and logging
- [ ] Concurrent access scenarios

### Transaction Safety Tests

- [ ] Row-level locking behavior validation
- [ ] Concurrent modification prevention
- [ ] Transaction rollback scenarios
- [ ] Deferred transformation timing
- [ ] Context cancellation during transactions

**Transaction Safety Test Implementation:**

```go
func TestBaseResponseHandler_TransactionSafety(t *testing.T) {
    t.Run("Should use row-level locking for state updates", func(t *testing.T) {
        // Arrange
        mockTaskRepo := new(MockTaskRepository)
        mockTaskRepo.On("WithTx", mock.Anything, mock.AnythingOfType("func(pgx.Tx) error")).Return(nil)
        mockTaskRepo.On("GetStateForUpdate", mock.Anything, mock.Anything, mock.Anything).Return(&task.State{}, nil)
        mockTaskRepo.On("UpsertState", mock.Anything, mock.Anything).Return(nil)

        handler := NewBaseResponseHandler(mockTaskRepo, nil, nil)
        state := &task.State{TaskExecID: "test-id"}

        // Act
        err := handler.saveTaskStateWithLocking(context.Background(), state)

        // Assert
        assert.NoError(t, err)
        mockTaskRepo.AssertExpectations(t)
        mockTaskRepo.AssertCalled(t, "GetStateForUpdate", mock.Anything, mock.Anything, "test-id")
    })

    t.Run("Should handle concurrent modification prevention", func(t *testing.T) {
        // Arrange
        mockTaskRepo := new(MockTaskRepository)
        expectedErr := errors.New("row locked by another transaction")
        mockTaskRepo.On("WithTx", mock.Anything, mock.AnythingOfType("func(pgx.Tx) error")).Return(expectedErr)

        handler := NewBaseResponseHandler(mockTaskRepo, nil, nil)
        state := &task.State{TaskExecID: "test-id"}

        // Act
        err := handler.saveTaskStateWithLocking(context.Background(), state)

        // Assert
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "row locked")
        mockTaskRepo.AssertExpectations(t)
    })

    t.Run("Should apply deferred output transformation safely", func(t *testing.T) {
        // Arrange
        mockTaskRepo := new(MockTaskRepository)
        mockOutputTransformer := new(MockOutputTransformer)

        state := &task.State{TaskExecID: "test-id", Status: core.StatusSuccess}
        transformedOutput := map[string]interface{}{"result": "transformed"}

        mockTaskRepo.On("WithTx", mock.Anything, mock.AnythingOfType("func(pgx.Tx) error")).Return(nil)
        mockTaskRepo.On("GetStateForUpdate", mock.Anything, mock.Anything, "test-id").Return(state, nil)
        mockTaskRepo.On("UpsertState", mock.Anything, mock.Anything).Return(nil)
        mockOutputTransformer.On("Transform", mock.Anything, state, mock.Anything).Return(transformedOutput, nil)

        handler := NewBaseResponseHandler(mockTaskRepo, mockOutputTransformer, nil)
        input := &ResponseInput{TaskState: state}

        // Act
        err := handler.applyDeferredOutputTransformation(context.Background(), input)

        // Assert
        assert.NoError(t, err)
        mockTaskRepo.AssertExpectations(t)
        mockOutputTransformer.AssertExpectations(t)
    })
}
```

### Integration Tests

- [ ] Integration with existing task2 normalizers
- [ ] Workflow and task repository interaction
- [ ] Error propagation through the stack

## Error Handling Strategy

- Preserve exact error semantics from TaskResponder
- Context cancellation handling at appropriate points
- Proper error wrapping with meaningful context
- Parent status update failures should not fail main flow

## Implementation Notes

- Extract logic without modification initially
- Maintain exact behavior compatibility
- Use dependency injection for repositories and normalizers
- Follow task2 architectural patterns
- Support for deferred output transformation (collection/parallel tasks)

## Parent Status Management

Extract complex parent status logic from TaskResponder:

- Transaction handling for concurrent updates
- Strategy-based status calculation
- Parallel task-specific logic
- Error handling and logging

## Implementation Considerations

- Maintain current behavior characteristics
- Efficient state saving and parent updates
- Minimal overhead for non-parent tasks
- Proper context cancellation support

## Success Criteria

- BaseResponseHandler successfully extracts common logic
- ParentStatusManager handles all parent status scenarios
- All tests pass with >70% coverage
- Code review approved
- Ready for task-specific handler implementation
- Framework established supports all task types

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

- [x] BaseResponseHandler implements ProcessMainTaskResponse with all common logic
- [x] Transaction safety patterns implemented with row-level locking
- [x] Deferred output transformation system working for collection/parallel tasks
- [x] ParentStatusManager handles all parent update scenarios
- [x] Context cancellation handling properly implemented
- [x] Error handling preserves exact semantics from TaskResponder
- [x] Integration with template engine and context builder verified
- [x] All test scenarios pass including concurrent access tests
- [x] Test coverage >70% for all components
- [x] Code passes `make lint` and `make test`
