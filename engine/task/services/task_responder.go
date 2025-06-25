package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2"
	task2core "github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/jackc/pgx/v5"
)

// -----------------------------------------------------------------------------
// TaskResponder - Service for handling task responses
// -----------------------------------------------------------------------------

// ParallelConfigData represents the parallel task configuration stored in task state
type ParallelConfigData struct {
	Strategy task.ParallelStrategy `json:"strategy"`
	// Add other parallel config fields as needed
}

type TaskResponder struct {
	workflowRepo                workflow.Repository
	taskRepo                    task.Repository
	parentStatusUpdater         *ParentStatusUpdater
	successTransitionNormalizer *task2core.SuccessTransitionNormalizer
	errorTransitionNormalizer   *task2core.ErrorTransitionNormalizer
	outputTransformer           *task2core.OutputTransformer
}

func NewTaskResponder(workflowRepo workflow.Repository, taskRepo task.Repository) *TaskResponder {
	// Create template engine for task2 normalizers
	engine := tplengine.NewEngine(tplengine.FormatJSON)
	templateEngineAdapter := task2.NewTemplateEngineAdapter(engine)

	return &TaskResponder{
		workflowRepo:                workflowRepo,
		taskRepo:                    taskRepo,
		parentStatusUpdater:         NewParentStatusUpdater(taskRepo),
		successTransitionNormalizer: task2core.NewSuccessTransitionNormalizer(templateEngineAdapter),
		errorTransitionNormalizer:   task2core.NewErrorTransitionNormalizer(templateEngineAdapter),
		outputTransformer:           task2core.NewOutputTransformer(templateEngineAdapter),
	}
}

// -----------------------------------------------------------------------------
// Main Task Response Handling
// -----------------------------------------------------------------------------

type MainTaskResponseInput struct {
	WorkflowConfig   *workflow.Config `json:"workflow_config"`
	TaskState        *task.State      `json:"task_state"`
	TaskConfig       *task.Config     `json:"task_config"`
	ExecutionError   error            `json:"execution_error"`
	NextTaskOverride *task.Config     `json:"next_task_override,omitempty"`
}

func (s *TaskResponder) HandleMainTask(
	ctx context.Context,
	input *MainTaskResponseInput,
) (*task.MainTaskResponse, error) {
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
	if ctx.Err() != nil {
		return &task.MainTaskResponse{State: input.TaskState}, nil
	}

	// Process transitions and validate error handling
	onSuccess, onError, err := s.processTransitions(ctx, input, isSuccess, executionErr)
	if err != nil {
		return nil, err
	}

	// Determine next task
	nextTask := s.determineNextTask(input, isSuccess)

	return &task.MainTaskResponse{
		OnSuccess: onSuccess,
		OnError:   onError,
		State:     input.TaskState,
		NextTask:  nextTask,
	}, nil
}

// processTaskExecutionResult handles output transformation and determines success status
func (s *TaskResponder) processTaskExecutionResult(
	ctx context.Context,
	input *MainTaskResponseInput,
) (bool, error) {
	state := input.TaskState
	executionErr := input.ExecutionError

	// Determine if task is successful so far
	isSuccess := executionErr == nil && state.Status != core.StatusFailed

	// Apply output transformation if needed
	// Skip for collection/parallel tasks as they need children data first
	if isSuccess && !s.shouldDeferOutputTransformation(input.TaskConfig) {
		state.UpdateStatus(core.StatusSuccess)
		if input.TaskConfig.GetOutputs() != nil && state.Output != nil {
			if err := s.applyOutputTransformation(ctx, input); err != nil {
				executionErr = err
				isSuccess = false
			}
		}
	}

	// Handle final state
	if !isSuccess {
		state.UpdateStatus(core.StatusFailed)
		s.setErrorState(state, executionErr)
	}

	return isSuccess, executionErr
}

// saveTaskState persists the task state to the repository
func (s *TaskResponder) saveTaskState(ctx context.Context, state *task.State) error {
	if err := s.taskRepo.UpsertState(ctx, state); err != nil {
		return fmt.Errorf("failed to update task state: %w", err)
	}
	return nil
}

// processTransitions normalizes transitions and validates error handling requirements
func (s *TaskResponder) processTransitions(
	ctx context.Context,
	input *MainTaskResponseInput,
	isSuccess bool,
	executionErr error,
) (*core.SuccessTransition, *core.ErrorTransition, error) {
	// Normalize transitions
	onSuccess, onError, err := s.normalizeTransitions(ctx, input)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, nil // Will be handled by caller
		}
		return nil, nil, fmt.Errorf("failed to normalize transitions: %w", err)
	}

	// Check for error transition requirement
	if !isSuccess && (onError == nil || onError.Next == nil) {
		if executionErr != nil {
			return nil, nil, fmt.Errorf("task failed with no error transition defined: %w", executionErr)
		}
		return nil, nil, errors.New("task failed with no error transition defined")
	}

	return onSuccess, onError, nil
}

// determineNextTask selects the next task based on override or workflow configuration
func (s *TaskResponder) determineNextTask(input *MainTaskResponseInput, isSuccess bool) *task.Config {
	if input.NextTaskOverride != nil {
		return input.NextTaskOverride
	}
	return input.WorkflowConfig.DetermineNextTask(input.TaskConfig, isSuccess)
}

// -----------------------------------------------------------------------------
// Subtask Response Handling
// -----------------------------------------------------------------------------

type SubtaskResponseInput struct {
	WorkflowConfig *workflow.Config `json:"workflow_config"`
	TaskState      *task.State      `json:"task_state"`
	TaskConfig     *task.Config     `json:"task_config"`
	ExecutionError error            `json:"execution_error"`
}

func (s *TaskResponder) HandleSubtask(ctx context.Context, input *SubtaskResponseInput) (*task.SubtaskResponse, error) {
	state := input.TaskState
	executionErr := input.ExecutionError

	// Determine if subtask is successful
	isSuccess := executionErr == nil && state.Status != core.StatusFailed

	// Apply output transformation if needed
	if isSuccess {
		state.UpdateStatus(core.StatusSuccess)
		state.Error = nil // Clear any residual error from previous attempts
		if input.TaskConfig.GetOutputs() != nil && state.Output != nil {
			if err := s.applySubtaskOutputTransformation(ctx, input); err != nil {
				executionErr = err
				isSuccess = false
			}
		}
	}

	// Handle final state
	if !isSuccess {
		state.UpdateStatus(core.StatusFailed)
		s.setErrorState(state, executionErr)
	}

	// Save state
	if err := s.taskRepo.UpsertState(ctx, state); err != nil {
		if ctx.Err() != nil {
			return &task.SubtaskResponse{
				TaskID: input.TaskConfig.ID,
				Output: state.Output,
				Status: state.Status,
				Error:  state.Error,
				State:  state,
			}, nil
		}
		return nil, fmt.Errorf("failed to update subtask state: %w", err)
	}

	// Parent status updates are handled by GetParallelResponse/GetCollectionResponse activities
	// after all child tasks complete, avoiding race conditions in concurrent subtask execution

	return &task.SubtaskResponse{
		TaskID: input.TaskConfig.ID,
		Output: state.Output,
		Status: state.Status,
		Error:  state.Error,
		State:  state,
	}, nil
}

// -----------------------------------------------------------------------------
// Collection Response Handling
// -----------------------------------------------------------------------------

type CollectionResponseInput struct {
	WorkflowConfig   *workflow.Config `json:"workflow_config"`
	TaskState        *task.State      `json:"task_state"`
	TaskConfig       *task.Config     `json:"task_config"`
	ExecutionError   error            `json:"execution_error"`
	NextTaskOverride *task.Config     `json:"next_task_override,omitempty"`
	ItemCount        int              `json:"item_count"`
	SkippedCount     int              `json:"skipped_count"`
}

func (s *TaskResponder) HandleCollection(
	ctx context.Context,
	input *CollectionResponseInput,
) (*task.CollectionResponse, error) {
	// Handle the main task response logic first
	mainTaskInput := &MainTaskResponseInput{
		WorkflowConfig:   input.WorkflowConfig,
		TaskState:        input.TaskState,
		TaskConfig:       input.TaskConfig,
		ExecutionError:   input.ExecutionError,
		NextTaskOverride: input.NextTaskOverride,
	}

	mainResponse, err := s.HandleMainTask(ctx, mainTaskInput)
	if err != nil {
		return nil, err
	}

	// Apply deferred output transformation for collection tasks after children are processed
	if err := s.applyDeferredOutputTransformation(ctx, mainTaskInput); err != nil {
		return nil, err
	}

	// Convert to collection response with additional fields
	return &task.CollectionResponse{
		MainTaskResponse: mainResponse,
		ItemCount:        input.ItemCount,
		SkippedCount:     input.SkippedCount,
	}, nil
}

// -----------------------------------------------------------------------------
// Parallel Response Handling
// -----------------------------------------------------------------------------

type ParallelResponseInput struct {
	WorkflowConfig   *workflow.Config `json:"workflow_config"`
	TaskState        *task.State      `json:"task_state"`
	TaskConfig       *task.Config     `json:"task_config"`
	ExecutionError   error            `json:"execution_error"`
	NextTaskOverride *task.Config     `json:"next_task_override,omitempty"`
}

func (s *TaskResponder) HandleParallel(
	ctx context.Context,
	input *ParallelResponseInput,
) (*task.MainTaskResponse, error) {
	// Handle the main task response logic first
	mainTaskInput := &MainTaskResponseInput{
		WorkflowConfig:   input.WorkflowConfig,
		TaskState:        input.TaskState,
		TaskConfig:       input.TaskConfig,
		ExecutionError:   input.ExecutionError,
		NextTaskOverride: input.NextTaskOverride,
	}

	mainResponse, err := s.HandleMainTask(ctx, mainTaskInput)
	if err != nil {
		return nil, err
	}

	// Apply deferred output transformation for parallel tasks after children are processed
	if err := s.applyDeferredOutputTransformation(ctx, mainTaskInput); err != nil {
		return nil, err
	}

	return mainResponse, nil
}

// -----------------------------------------------------------------------------
// Helper Methods
// -----------------------------------------------------------------------------

func (s *TaskResponder) shouldDeferOutputTransformation(taskConfig *task.Config) bool {
	return taskConfig.Type == task.TaskTypeCollection || taskConfig.Type == task.TaskTypeParallel
}

// applyDeferredOutputTransformation applies output transformation for parent tasks after children are processed
func (s *TaskResponder) applyDeferredOutputTransformation(
	ctx context.Context,
	input *MainTaskResponseInput,
) error {
	// Only apply if no execution error and task is not failed
	if input.ExecutionError != nil || input.TaskState.Status == core.StatusFailed {
		return nil
	}

	// Use a transaction to prevent race conditions and ensure atomicity
	return s.taskRepo.WithTx(ctx, func(tx pgx.Tx) error {
		// Get the latest state with lock to prevent concurrent modifications
		latestState, err := s.taskRepo.GetStateForUpdate(ctx, tx, input.TaskState.TaskExecID)
		if err != nil {
			return fmt.Errorf("failed to get latest state for update: %w", err)
		}

		// Use the fresh state for the transformation
		// Create a new input for the transformation function to avoid side effects
		transformInput := &MainTaskResponseInput{
			WorkflowConfig: input.WorkflowConfig,
			TaskState:      latestState,
			TaskConfig:     input.TaskConfig,
		}

		// Ensure the task is marked as successful for the transformation context
		transformInput.TaskState.UpdateStatus(core.StatusSuccess)

		// Apply output transformation if needed
		var transformErr error
		if transformInput.TaskConfig.GetOutputs() != nil && transformInput.TaskState.Output != nil {
			transformErr = s.applyOutputTransformation(ctx, transformInput)
		}

		if transformErr != nil {
			// On transformation failure, mark task as failed and set error
			transformInput.TaskState.UpdateStatus(core.StatusFailed)
			s.setErrorState(transformInput.TaskState, transformErr)
		}

		// Save the updated state (either with transformed output or with failure status)
		if err := s.taskRepo.UpsertStateWithTx(ctx, tx, transformInput.TaskState); err != nil {
			if transformErr != nil {
				return fmt.Errorf(
					"failed to save state after transformation error: %w (original error: %w)",
					err,
					transformErr,
				)
			}
			return fmt.Errorf("failed to save transformed state: %w", err)
		}

		return transformErr // Return the original transformation error if it occurred
	})
}

func (s *TaskResponder) applyOutputTransformationCommon(
	ctx context.Context,
	state *task.State,
	taskConfig *task.Config,
	workflowConfig *workflow.Config,
) error {
	workflowState, err := s.workflowRepo.GetState(ctx, state.WorkflowExecID)
	if err != nil {
		return fmt.Errorf("failed to get workflow state for output transformation: %w", err)
	}
	// Build task configs map for context
	taskConfigs := task2.BuildTaskConfigsMap(workflowConfig.Tasks)

	// Build children index for parent-child relationships
	childrenIndexBuilder := shared.NewChildrenIndexBuilder()
	childrenIndex := childrenIndexBuilder.BuildChildrenIndex(workflowState)

	// Create normalization context
	normCtx := &shared.NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     taskConfig,
		TaskConfigs:    taskConfigs,
		CurrentInput:   taskConfig.With,
		MergedEnv:      taskConfig.Env,
		ChildrenIndex:  childrenIndex,
	}

	output, err := s.outputTransformer.TransformOutput(
		state.Output,
		taskConfig.GetOutputs(),
		normCtx,
		taskConfig,
	)
	if err != nil {
		return fmt.Errorf("failed to apply output transformation: %w", err)
	}
	state.Output = output
	return nil
}

func (s *TaskResponder) applyOutputTransformation(ctx context.Context, input *MainTaskResponseInput) error {
	return s.applyOutputTransformationCommon(ctx, input.TaskState, input.TaskConfig, input.WorkflowConfig)
}

func (s *TaskResponder) applySubtaskOutputTransformation(ctx context.Context, input *SubtaskResponseInput) error {
	return s.applyOutputTransformationCommon(ctx, input.TaskState, input.TaskConfig, input.WorkflowConfig)
}

func (s *TaskResponder) setErrorState(state *task.State, executionErr error) {
	if executionErr == nil {
		var errorMessage string
		if state.IsParallelExecution() {
			errorMessage = "parent task execution failed due to child task failures"
		} else {
			errorMessage = "task execution failed"
		}
		state.Error = core.NewError(errors.New(errorMessage), "execution_error", nil)
	} else {
		state.Error = core.NewError(executionErr, "execution_error", nil)
	}
}

func (s *TaskResponder) normalizeTransitions(
	ctx context.Context,
	input *MainTaskResponseInput,
) (*core.SuccessTransition, *core.ErrorTransition, error) {
	workflowState, err := s.workflowRepo.GetState(ctx, input.TaskState.WorkflowExecID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get workflow state: %w", err)
	}

	// Create normalization context for task2
	normCtx := &shared.NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: input.WorkflowConfig,
		TaskConfig:     input.TaskConfig,
		CurrentInput:   input.TaskState.Input,
	}

	// Normalize success transition
	var normalizedOnSuccess *core.SuccessTransition
	if input.TaskConfig.OnSuccess != nil {
		// Create a copy to avoid mutating the original
		successCopy := &core.SuccessTransition{
			Next: input.TaskConfig.OnSuccess.Next,
			With: input.TaskConfig.OnSuccess.With,
		}
		if input.TaskConfig.OnSuccess.With != nil {
			withCopy := make(core.Input)
			maps.Copy(withCopy, *input.TaskConfig.OnSuccess.With)
			successCopy.With = &withCopy
		}

		err = s.successTransitionNormalizer.Normalize(successCopy, normCtx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to normalize success transition: %w", err)
		}
		normalizedOnSuccess = successCopy
	}

	// Normalize error transition
	var normalizedOnError *core.ErrorTransition
	if input.TaskConfig.OnError != nil {
		// Create a copy to avoid mutating the original
		errorCopy := &core.ErrorTransition{
			Next: input.TaskConfig.OnError.Next,
			With: input.TaskConfig.OnError.With,
		}
		if input.TaskConfig.OnError.With != nil {
			withCopy := make(core.Input)
			maps.Copy(withCopy, *input.TaskConfig.OnError.With)
			errorCopy.With = &withCopy
		}

		err = s.errorTransitionNormalizer.Normalize(errorCopy, normCtx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to normalize error transition: %w", err)
		}
		normalizedOnError = errorCopy
	}

	return normalizedOnSuccess, normalizedOnError, nil
}

func (s *TaskResponder) logParentStatusUpdateError(ctx context.Context, state *task.State) {
	log := logger.FromContext(ctx)
	if err := s.updateParentStatusIfNeeded(ctx, state); err != nil {
		log.Debug("Failed to update parent status", "error", err)
	}
}

func (s *TaskResponder) updateParentStatusIfNeeded(ctx context.Context, childState *task.State) error {
	// Only proceed if this is a child task (has a parent)
	if childState.ParentStateID == nil {
		return nil
	}

	parentStateID := *childState.ParentStateID

	// Use transaction to prevent race conditions when multiple children update parent simultaneously
	return s.taskRepo.WithTx(ctx, func(tx pgx.Tx) error {
		// Get the parent task with row lock to prevent concurrent modifications
		parentState, err := s.taskRepo.GetStateForUpdate(ctx, tx, parentStateID)
		if err != nil {
			return fmt.Errorf("failed to get parent state %s for update: %w", parentStateID, err)
		}

		// Only update parent status for parallel tasks
		if parentState.ExecutionType != task.ExecutionParallel {
			return nil
		}

		// Extract strategy from parallel configuration
		strategy, err := s.extractParallelStrategy(ctx, parentState, parentStateID)
		if err != nil {
			return fmt.Errorf("failed to extract parallel strategy: %w", err)
		}

		// Use the shared service to update parent status within transaction
		_, err = s.parentStatusUpdater.UpdateParentStatus(ctx, &UpdateParentStatusInput{
			ParentStateID: parentStateID,
			Strategy:      strategy,
			Recursive:     true,
			ChildState:    childState,
		})

		return err
	})
}

// extractParallelStrategy extracts the parallel strategy from parent state input
// using a typed struct to avoid brittle nested type assertions
func (s *TaskResponder) extractParallelStrategy(
	ctx context.Context,
	parentState *task.State,
	parentStateID core.ID,
) (task.ParallelStrategy, error) {
	log := logger.FromContext(ctx)
	// Default strategy
	defaultStrategy := task.StrategyWaitAll

	// Check if input exists
	if parentState.Input == nil {
		return defaultStrategy, nil
	}

	// Get the parallel config field
	parallelConfigRaw, exists := (*parentState.Input)["_parallel_config"]
	if !exists {
		return defaultStrategy, nil
	}

	// Try to unmarshal into our typed struct
	// Handle both map and JSON string formats
	var jsonBytes []byte
	switch v := parallelConfigRaw.(type) {
	case string:
		jsonBytes = []byte(v) // already JSON
	default:
		var err error
		jsonBytes, err = json.Marshal(v)
		if err != nil {
			log.Error("Failed to marshal parallel config for extraction",
				"parent_state_id", parentStateID,
				"error", err,
			)
			return "", fmt.Errorf("failed to marshal parallel config: %w", err)
		}
	}

	var configData ParallelConfigData
	if err := json.Unmarshal(jsonBytes, &configData); err != nil {
		log.Error("Failed to unmarshal parallel config into typed struct",
			"parent_state_id", parentStateID,
			"config_type", fmt.Sprintf("%T", parallelConfigRaw),
			"error", err,
		)
		return "", fmt.Errorf("failed to unmarshal parallel config: %w", err)
	}

	// Validate the extracted strategy
	if !task.ValidateStrategy(string(configData.Strategy)) {
		if configData.Strategy != "" {
			log.Debug("Invalid parallel strategy found, using default wait_all",
				"invalid_strategy", configData.Strategy,
				"parent_state_id", parentStateID,
			)
		}
		return defaultStrategy, nil
	}

	return configData.Strategy, nil
}
