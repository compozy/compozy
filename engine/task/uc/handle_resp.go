package uc

import (
	"context"
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
	State          *task.State
	WorkflowConfig *workflow.Config
	TaskConfig     *task.Config
	ExecutionError error
}

type HandleResponseOutput struct {
	Response *task.Response
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

func (uc *HandleResponse) Execute(ctx context.Context, input *HandleResponseInput) (*HandleResponseOutput, error) {
	if input.ExecutionError != nil {
		return uc.handleErrorFlow(ctx, input)
	}
	return uc.handleSuccessFlow(ctx, input)
}

func (uc *HandleResponse) handleSuccessFlow(
	ctx context.Context,
	input *HandleResponseInput,
) (*HandleResponseOutput, error) {
	state := input.State
	state.UpdateStatus(core.StatusSuccess)
	if err := uc.taskRepo.UpsertState(ctx, state); err != nil {
		if ctx.Err() != nil {
			return &HandleResponseOutput{
				Response: &task.Response{State: state},
			}, nil
		}
		return nil, fmt.Errorf("failed to update task state: %w", err)
	}
	if ctx.Err() != nil {
		return &HandleResponseOutput{
			Response: &task.Response{State: state},
		}, nil
	}
	onSuccess, onError, err := uc.normalizeTransitions(ctx, input)
	if err != nil {
		if ctx.Err() != nil {
			return &HandleResponseOutput{
				Response: &task.Response{State: state},
			}, nil
		}
		return nil, fmt.Errorf("failed to normalize transitions: %w", err)
	}
	nextTask := input.WorkflowConfig.DetermineNextTask(input.TaskConfig, true)
	return &HandleResponseOutput{
		Response: &task.Response{
			OnSuccess: onSuccess,
			OnError:   onError,
			State:     state,
			NextTask:  nextTask,
		},
	}, nil
}

func (uc *HandleResponse) handleErrorFlow(
	ctx context.Context,
	input *HandleResponseInput,
) (*HandleResponseOutput, error) {
	state := input.State
	executionErr := input.ExecutionError
	state.UpdateStatus(core.StatusFailed)
	state.Error = core.NewError(executionErr, "execution_error", nil)
	if updateErr := uc.taskRepo.UpsertState(ctx, state); updateErr != nil {
		if ctx.Err() != nil {
			return &HandleResponseOutput{
				Response: &task.Response{State: state},
			}, nil
		}
		return nil, fmt.Errorf("failed to update task state after error: %w", updateErr)
	}
	if ctx.Err() != nil {
		return &HandleResponseOutput{
			Response: &task.Response{State: state},
		}, nil
	}
	if input.TaskConfig.OnError == nil || input.TaskConfig.OnError.Next == nil {
		return nil, fmt.Errorf("task failed with no error transition defined: %w", executionErr)
	}
	onSuccess, onError, err := uc.normalizeTransitions(ctx, input)
	if err != nil {
		if ctx.Err() != nil {
			return &HandleResponseOutput{
				Response: &task.Response{State: state},
			}, nil
		}
		return nil, fmt.Errorf("failed to normalize transitions: %w", err)
	}
	nextTask := input.WorkflowConfig.DetermineNextTask(input.TaskConfig, false)
	return &HandleResponseOutput{
		Response: &task.Response{
			OnSuccess: onSuccess,
			OnError:   onError,
			State:     state,
			NextTask:  nextTask,
		},
	}, nil
}

func (uc *HandleResponse) normalizeTransitions(
	ctx context.Context,
	input *HandleResponseInput,
) (*core.SuccessTransition, *core.ErrorTransition, error) {
	workflowExecID := input.State.WorkflowExecID
	workflowConfig := input.WorkflowConfig
	taskConfig := input.TaskConfig
	tasks := workflowConfig.Tasks
	allTaskConfigs := normalizer.BuildTaskConfigsMap(tasks)
	workflowState, err := uc.workflowRepo.GetState(ctx, workflowExecID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get workflow state: %w", err)
	}
	baseEnv, err := uc.normalizer.NormalizeTask(workflowState, workflowConfig, taskConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to normalize base environment: %w", err)
	}
	normalizedOnSuccess, err := uc.normalizeSuccessTransition(
		taskConfig.OnSuccess,
		workflowState,
		workflowConfig,
		allTaskConfigs,
		baseEnv,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to normalize success transition: %w", err)
	}
	normalizedOnError, err := uc.normalizeErrorTransition(
		taskConfig.OnError,
		workflowState,
		workflowConfig,
		allTaskConfigs,
		baseEnv,
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
	baseEnv core.EnvMap,
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
	baseEnv core.EnvMap,
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
