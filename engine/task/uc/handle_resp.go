package uc

import (
	"context"
	"errors"
	"fmt"

	"maps"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/normalizer"
)

// -----------------------------------------------------------------------------
// HandleResponse
// -----------------------------------------------------------------------------

type HandleResponseInput struct {
	WorkflowConfig   *workflow.Config `json:"workflow_config"`
	TaskState        *task.State      `json:"task_state"`
	TaskConfig       *task.Config     `json:"task_config"`
	ExecutionError   error            `json:"execution_error"`
	NextTaskOverride *task.Config     `json:"next_task_override,omitempty"`
}

type HandleResponse struct {
	workflowRepo workflow.Repository
	taskRepo     task.Repository
	normalizer   *normalizer.ConfigNormalizer
}

func NewHandleResponse(workflowRepo workflow.Repository, taskRepo task.Repository) *HandleResponse {
	return &HandleResponse{
		workflowRepo: workflowRepo,
		taskRepo:     taskRepo,
		normalizer:   normalizer.NewConfigNormalizer(),
	}
}

func (uc *HandleResponse) Execute(ctx context.Context, input *HandleResponseInput) (*task.Response, error) {
	if input == nil {
		return nil, fmt.Errorf("input is nil")
	}
	if input.TaskState == nil {
		return nil, fmt.Errorf("task state is nil")
	}
	if input.TaskConfig == nil {
		return nil, fmt.Errorf("task config is nil")
	}
	if input.WorkflowConfig == nil {
		return nil, fmt.Errorf("workflow config is nil")
	}
	
	// Check if there's an execution error OR if the task state indicates failure
	hasExecutionError := input.ExecutionError != nil
	hasTaskFailure := input.TaskState.Status == core.StatusFailed
	if hasExecutionError || hasTaskFailure {
		return uc.handleErrorFlow(ctx, input)
	}
	return uc.handleSuccessFlow(ctx, input)
}

func (uc *HandleResponse) handleSuccessFlow(
	ctx context.Context,
	input *HandleResponseInput,
) (*task.Response, error) {
	state := input.TaskState
	state.UpdateStatus(core.StatusSuccess)
	if input.TaskConfig.GetOutputs() != nil && state.Output != nil {
		workflowState, err := uc.workflowRepo.GetState(ctx, input.TaskState.WorkflowExecID)
		if err != nil {
			return nil, fmt.Errorf("failed to get workflow state for output transformation: %w", err)
		}
		output, err := uc.normalizer.NormalizeTaskOutput(
			state.Output,
			input.TaskConfig.GetOutputs(),
			workflowState,
			input.WorkflowConfig,
			input.TaskConfig,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to apply output transformation: %w", err)
		}
		state.Output = output
	}
	if err := uc.taskRepo.UpsertState(ctx, state); err != nil {
		if ctx.Err() != nil {
			return &task.Response{State: state}, nil
		}
		return nil, fmt.Errorf("failed to update task state: %w", err)
	}
	if ctx.Err() != nil {
		return &task.Response{State: state}, nil
	}
	onSuccess, onError, err := uc.normalizeTransitions(ctx, input)
	if err != nil {
		if ctx.Err() != nil {
			return &task.Response{State: state}, nil
		}
		return nil, fmt.Errorf("failed to normalize transitions: %w", err)
	}
	var nextTask *task.Config
	if input.NextTaskOverride != nil {
		nextTask = input.NextTaskOverride
	} else {
		nextTask = input.WorkflowConfig.DetermineNextTask(input.TaskConfig, true)
	}
	return &task.Response{
		OnSuccess: onSuccess,
		OnError:   onError,
		State:     state,
		NextTask:  nextTask,
	}, nil
}

func (uc *HandleResponse) handleErrorFlow(
	ctx context.Context,
	input *HandleResponseInput,
) (*task.Response, error) {
	state := input.TaskState
	executionErr := input.ExecutionError
	state.UpdateStatus(core.StatusFailed)

	// Handle case where task failed but there's no execution error (e.g., parallel task with failed subtasks)
	if executionErr == nil {
		var errorMessage string
		if state.IsParallel() {
			errorMessage = "parallel task execution failed due to subtask failures"
		} else {
			errorMessage = "task execution failed"
		}
		state.Error = core.NewError(errors.New(errorMessage), "execution_error", nil)
	} else {
		state.Error = core.NewError(executionErr, "execution_error", nil)
	}

	if updateErr := uc.taskRepo.UpsertState(ctx, state); updateErr != nil {
		if ctx.Err() != nil {
			return &task.Response{State: state}, nil
		}
		return nil, fmt.Errorf("failed to update task state after error: %w", updateErr)
	}
	if ctx.Err() != nil {
		return &task.Response{State: state}, nil
	}
	if input.TaskConfig.OnError == nil || input.TaskConfig.OnError.Next == nil {
		// For cases where execution error is nil, we shouldn't propagate it
		if executionErr != nil {
			return nil, fmt.Errorf("task failed with no error transition defined: %w", executionErr)
		}
		return nil, fmt.Errorf("task failed with no error transition defined")
	}
	onSuccess, onError, err := uc.normalizeTransitions(ctx, input)
	if err != nil {
		if ctx.Err() != nil {
			return &task.Response{State: state}, nil
		}
		return nil, fmt.Errorf("failed to normalize transitions: %w", err)
	}
	nextTask := input.WorkflowConfig.DetermineNextTask(input.TaskConfig, false)
	return &task.Response{
		OnSuccess: onSuccess,
		OnError:   onError,
		State:     state,
		NextTask:  nextTask,
	}, nil
}

func (uc *HandleResponse) normalizeTransitions(
	ctx context.Context,
	input *HandleResponseInput,
) (*core.SuccessTransition, *core.ErrorTransition, error) {
	workflowExecID := input.TaskState.WorkflowExecID
	workflowConfig := input.WorkflowConfig
	taskConfig := input.TaskConfig
	tasks := workflowConfig.Tasks
	allTaskConfigs := normalizer.BuildTaskConfigsMap(tasks)
	workflowState, err := uc.workflowRepo.GetState(ctx, workflowExecID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get workflow state: %w", err)
	}
	err = uc.normalizer.NormalizeTaskEnvironment(workflowConfig, taskConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to normalize base environment: %w", err)
	}
	normalizedOnSuccess, err := uc.normalizeSuccessTransition(
		taskConfig.OnSuccess,
		workflowState,
		workflowConfig,
		allTaskConfigs,
		taskConfig.Env,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to normalize success transition: %w", err)
	}
	normalizedOnError, err := uc.normalizeErrorTransition(
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

func (uc *HandleResponse) normalizeSuccessTransition(
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
	if err := uc.normalizer.NormalizeSuccessTransition(
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

func (uc *HandleResponse) normalizeErrorTransition(
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
	if err := uc.normalizer.NormalizeErrorTransition(
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
