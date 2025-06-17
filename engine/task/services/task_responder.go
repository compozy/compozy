package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/normalizer"
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
	workflowRepo        workflow.Repository
	taskRepo            task.Repository
	normalizer          *normalizer.ConfigNormalizer
	parentStatusUpdater *ParentStatusUpdater
}

func NewTaskResponder(workflowRepo workflow.Repository, taskRepo task.Repository) *TaskResponder {
	return &TaskResponder{
		workflowRepo:        workflowRepo,
		taskRepo:            taskRepo,
		normalizer:          normalizer.NewConfigNormalizer(),
		parentStatusUpdater: NewParentStatusUpdater(taskRepo),
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
	if isSuccess {
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

	// Convert to collection response with additional fields
	return &task.CollectionResponse{
		MainTaskResponse: mainResponse,
		ItemCount:        input.ItemCount,
		SkippedCount:     input.SkippedCount,
	}, nil
}

// -----------------------------------------------------------------------------
// Helper Methods
// -----------------------------------------------------------------------------

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
	output, err := s.normalizer.NormalizeTaskOutput(
		state.Output,
		taskConfig.GetOutputs(),
		workflowState,
		workflowConfig,
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
	workflowExecID := input.TaskState.WorkflowExecID
	workflowConfig := input.WorkflowConfig
	taskConfig := input.TaskConfig
	tasks := workflowConfig.Tasks
	allTaskConfigs := normalizer.BuildTaskConfigsMap(tasks)

	workflowState, err := s.workflowRepo.GetState(ctx, workflowExecID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get workflow state: %w", err)
	}

	err = s.normalizer.NormalizeTaskEnvironment(workflowConfig, taskConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to normalize base environment: %w", err)
	}

	normalizedOnSuccess, err := s.normalizeSuccessTransition(
		taskConfig.OnSuccess,
		workflowState,
		workflowConfig,
		allTaskConfigs,
		taskConfig.Env,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to normalize success transition: %w", err)
	}

	normalizedOnError, err := s.normalizeErrorTransition(
		taskConfig.OnError,
		workflowState,
		workflowConfig,
		allTaskConfigs,
		taskConfig.Env,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to normalize error transition: %w", err)
	}

	return normalizedOnSuccess, normalizedOnError, nil
}

func (s *TaskResponder) normalizeSuccessTransition(
	transition *core.SuccessTransition,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	allTaskConfigs map[string]*task.Config,
	baseEnv *core.EnvMap,
) (*core.SuccessTransition, error) {
	if transition == nil {
		return nil, nil
	}
	normalizedTransition := &core.SuccessTransition{
		Next: transition.Next,
		With: transition.With,
	}
	if transition.With != nil {
		withCopy := make(core.Input)
		maps.Copy(withCopy, *transition.With)
		normalizedTransition.With = &withCopy
	}
	if err := s.normalizer.NormalizeSuccessTransition(
		normalizedTransition,
		workflowState,
		workflowConfig,
		allTaskConfigs,
		baseEnv,
	); err != nil {
		return nil, err
	}
	return normalizedTransition, nil
}

func (s *TaskResponder) normalizeErrorTransition(
	transition *core.ErrorTransition,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	allTaskConfigs map[string]*task.Config,
	baseEnv *core.EnvMap,
) (*core.ErrorTransition, error) {
	if transition == nil {
		return nil, nil
	}
	normalizedTransition := &core.ErrorTransition{
		Next: transition.Next,
		With: transition.With,
	}
	if transition.With != nil {
		withCopy := make(core.Input)
		maps.Copy(withCopy, *transition.With)
		normalizedTransition.With = &withCopy
	}
	if err := s.normalizer.NormalizeErrorTransition(
		normalizedTransition,
		workflowState,
		workflowConfig,
		allTaskConfigs,
		baseEnv,
	); err != nil {
		return nil, err
	}
	return normalizedTransition, nil
}

func (s *TaskResponder) logParentStatusUpdateError(ctx context.Context, state *task.State) {
	if err := s.updateParentStatusIfNeeded(ctx, state); err != nil {
		logger.Debug("failed to update parent status", "error", err)
	}
}

func (s *TaskResponder) updateParentStatusIfNeeded(ctx context.Context, childState *task.State) error {
	// Only proceed if this is a child task (has a parent)
	if childState.ParentStateID == nil {
		return nil
	}

	parentStateID := *childState.ParentStateID

	// Get the parent task to determine the parallel strategy
	parentState, err := s.taskRepo.GetState(ctx, parentStateID)
	if err != nil {
		return fmt.Errorf("failed to get parent state %s: %w", parentStateID, err)
	}

	// Only update parent status for parallel tasks
	if parentState.ExecutionType != task.ExecutionParallel {
		return nil
	}

	// Extract strategy from parallel configuration
	strategy := s.extractParallelStrategy(parentState, parentStateID)

	// Use the shared service to update parent status
	_, err = s.parentStatusUpdater.UpdateParentStatus(ctx, &UpdateParentStatusInput{
		ParentStateID: parentStateID,
		Strategy:      strategy,
		Recursive:     true,
		ChildState:    childState,
	})

	return err
}

// extractParallelStrategy extracts the parallel strategy from parent state input
// using a typed struct to avoid brittle nested type assertions
func (s *TaskResponder) extractParallelStrategy(parentState *task.State, parentStateID core.ID) task.ParallelStrategy {
	// Default strategy
	defaultStrategy := task.StrategyWaitAll

	// Check if input exists
	if parentState.Input == nil {
		return defaultStrategy
	}

	// Get the parallel config field
	parallelConfigRaw, exists := (*parentState.Input)["_parallel_config"]
	if !exists {
		return defaultStrategy
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
			logger.Error("Failed to marshal parallel config for extraction",
				"parent_state_id", parentStateID,
				"error", err,
			)
			return defaultStrategy
		}
	}

	var configData ParallelConfigData
	if err := json.Unmarshal(jsonBytes, &configData); err != nil {
		logger.Error("Failed to unmarshal parallel config into typed struct",
			"parent_state_id", parentStateID,
			"config_type", fmt.Sprintf("%T", parallelConfigRaw),
			"error", err,
		)
		return defaultStrategy
	}

	// Validate the extracted strategy
	if !task.ValidateStrategy(string(configData.Strategy)) {
		if configData.Strategy != "" {
			logger.Error("Invalid parallel strategy found, using default wait_all",
				"invalid_strategy", configData.Strategy,
				"parent_state_id", parentStateID,
			)
		}
		return defaultStrategy
	}

	return configData.Strategy
}
