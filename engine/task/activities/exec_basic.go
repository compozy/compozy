package activities

import (
	"context"
	"fmt"

	"maps"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/normalizer"
)

const ExecuteBasicLabel = "ExecuteBasicTask"

type ExecuteBasicInput = DispatchOutput

type ExecuteBasicData struct {
	WorkflowConfig *workflow.Config
	TaskConfig     *task.Config
	AgentConfig    *agent.Config
	ActionConfig   *agent.ActionConfig
}

type ExecuteBasic struct {
	workflows    []*workflow.Config
	workflowRepo workflow.Repository
	taskRepo     task.Repository
	normalizer   *normalizer.ConfigNormalizer
	llmService   llm.Service
}

// NewExecuteBasic creates a new ExecuteBasic activity
func NewExecuteBasic(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	llmService llm.Service,
) *ExecuteBasic {
	return &ExecuteBasic{
		workflows:    workflows,
		workflowRepo: workflowRepo,
		taskRepo:     taskRepo,
		normalizer:   normalizer.NewConfigNormalizer(),
		llmService:   llmService,
	}
}

func (a *ExecuteBasic) Run(ctx context.Context, input *ExecuteBasicInput) (*task.Response, error) {
	state := input.State
	// Load execution data
	execData, err := a.loadData(state, input)
	if err != nil {
		return nil, err
	}
	taskType := execData.TaskConfig.Type
	if taskType != task.TaskTypeBasic {
		return nil, fmt.Errorf("unsupported task type: %s", taskType)
	}

	// TODO: We will deal just with agent component for now
	if state.Component != core.ComponentAgent {
		return nil, fmt.Errorf("unsupported component type: %s", state.Component)
	}
	if state.AgentID == nil {
		return nil, fmt.Errorf("agent ID is required for agent execution")
	}
	if state.ActionID == nil {
		return nil, fmt.Errorf("action ID is required for agent execution")
	}

	// Create LLM and execute agent
	agentConfig := execData.AgentConfig
	actionConfig := execData.ActionConfig
	providerConfig := agentConfig.Config
	model, err := a.llmService.CreateLLM(&providerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM: %w", err)
	}
	instructions := agentConfig.Instructions
	prompt := a.llmService.BuildPrompt(
		&llm.PromptConfig{
			BasePrompt: actionConfig.Prompt,
		},
		state.Input,
	)
	result, err := a.llmService.GenerateContent(ctx, model, instructions, prompt)
	if err != nil {
		return a.responseOnError(ctx, execData, state, err)
	}
	return a.responseOnSuccess(ctx, execData, state, result)
}

func (a *ExecuteBasic) loadData(state *task.State, input *ExecuteBasicInput) (*ExecuteBasicData, error) {
	// Find workflow config
	workflowID := state.WorkflowID
	workflowConfig, err := workflow.FindConfig(a.workflows, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow config: %w", err)
	}
	agentConfig := input.Config.Agent
	actionConfig, err := a.findActionConfig(agentConfig, *state.ActionID)
	if err != nil {
		return nil, fmt.Errorf("failed to find action config: %w", err)
	}
	return &ExecuteBasicData{
		WorkflowConfig: workflowConfig,
		TaskConfig:     input.Config,
		AgentConfig:    agentConfig,
		ActionConfig:   actionConfig,
	}, nil
}

func (a *ExecuteBasic) determineNextTask(
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
	success bool,
) *task.Config {
	var nextTaskID string
	if success && taskConfig.OnSuccess != nil && taskConfig.OnSuccess.Next != nil {
		nextTaskID = *taskConfig.OnSuccess.Next
	} else if !success && taskConfig.OnError != nil && taskConfig.OnError.Next != nil {
		nextTaskID = *taskConfig.OnError.Next
	}
	if nextTaskID == "" {
		return nil
	}
	// Find the next task config
	nextTask, err := task.FindConfig(workflowConfig.Tasks, nextTaskID)
	if err != nil {
		return nil
	}
	return nextTask
}

func (a *ExecuteBasic) findActionConfig(agentConfig *agent.Config, actionID string) (*agent.ActionConfig, error) {
	for _, action := range agentConfig.Actions {
		if action.ID == actionID {
			return action, nil
		}
	}
	return nil, fmt.Errorf("action not found: %s", actionID)
}

func (a *ExecuteBasic) normalizeTransitions(
	ctx context.Context,
	execData *ExecuteBasicData,
	state *task.State,
) (*core.SuccessTransition, *core.ErrorTransition, error) {
	workflowExecID := state.WorkflowExecID
	workflowConfig := execData.WorkflowConfig
	taskConfig := execData.TaskConfig
	tasks := workflowConfig.Tasks
	allTaskConfigs := normalizer.BuildTaskConfigsMap(tasks)
	workflowState, err := a.workflowRepo.GetState(ctx, workflowExecID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get workflow state: %w", err)
	}
	baseEnv, err := a.normalizer.NormalizeTask(workflowState, workflowConfig, taskConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to normalize base environment: %w", err)
	}
	normalizedOnSuccess, err := a.normalizeSuccessTransition(
		taskConfig.OnSuccess,
		workflowState,
		workflowConfig,
		allTaskConfigs,
		baseEnv,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to normalize success transition: %w", err)
	}
	normalizedOnError, err := a.normalizeErrorTransition(
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

func (a *ExecuteBasic) normalizeSuccessTransition(
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
	if err := a.normalizer.NormalizeSuccessTransition(
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

func (a *ExecuteBasic) normalizeErrorTransition(
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
	if err := a.normalizer.NormalizeErrorTransition(
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

func (a *ExecuteBasic) responseOnSuccess(
	ctx context.Context,
	execData *ExecuteBasicData,
	state *task.State,
	result core.Output,
) (*task.Response, error) {
	// Always update status first
	state.UpdateStatus(core.StatusSuccess)
	state.Output = &result

	// Try to update task state, but don't fail if context is canceled
	if err := a.taskRepo.UpsertState(ctx, state); err != nil {
		// If context is canceled, return basic response
		if ctx.Err() != nil {
			return &task.Response{
				State: state,
			}, nil
		}
		return nil, fmt.Errorf("failed to update task state: %w", err)
	}

	// Skip normalization if context is canceled
	if ctx.Err() != nil {
		return &task.Response{
			State: state,
		}, nil
	}

	workflowConfig := execData.WorkflowConfig
	taskConfig := execData.TaskConfig
	onSuccess, onError, err := a.normalizeTransitions(ctx, execData, state)
	if err != nil {
		// If normalization fails due to cancellation, return basic response
		if ctx.Err() != nil {
			return &task.Response{
				State: state,
			}, nil
		}
		return nil, fmt.Errorf("failed to normalize transitions: %w", err)
	}
	nextTask := a.determineNextTask(workflowConfig, taskConfig, true)
	return &task.Response{
		OnSuccess: onSuccess,
		OnError:   onError,
		State:     state,
		NextTask:  nextTask,
	}, nil
}

func (a *ExecuteBasic) responseOnError(
	ctx context.Context,
	execData *ExecuteBasicData,
	state *task.State,
	executionErr error,
) (*task.Response, error) {
	// Always update status first
	state.UpdateStatus(core.StatusFailed)
	state.Error = core.NewError(executionErr, "agent_execution_error", nil)

	// Try to update task state, but don't fail if context is canceled
	if updateErr := a.taskRepo.UpsertState(ctx, state); updateErr != nil {
		// If context is canceled, log but don't fail the response
		if ctx.Err() != nil {
			// Still return a valid response even if we couldn't update the database
			return &task.Response{
				State: state,
			}, nil
		}
		return nil, fmt.Errorf("failed to update task state after error: %w", updateErr)
	}

	// Skip normalization if context is canceled
	if ctx.Err() != nil {
		return &task.Response{
			State: state,
		}, nil
	}

	workflowConfig := execData.WorkflowConfig
	taskConfig := execData.TaskConfig

	// Check if there's an error transition defined
	if taskConfig.OnError == nil || taskConfig.OnError.Next == nil {
		// No error transition defined, fail the workflow
		return nil, fmt.Errorf("task failed with no error transition defined: %w", executionErr)
	}

	onSuccess, onError, err := a.normalizeTransitions(ctx, execData, state)
	if err != nil {
		// If normalization fails due to cancellation, return basic response
		if ctx.Err() != nil {
			return &task.Response{
				State: state,
			}, nil
		}
		return nil, fmt.Errorf("failed to normalize transitions: %w", err)
	}
	nextTask := a.determineNextTask(workflowConfig, taskConfig, false)
	return &task.Response{
		OnSuccess: onSuccess,
		OnError:   onError,
		State:     state,
		NextTask:  nextTask,
	}, nil
}
