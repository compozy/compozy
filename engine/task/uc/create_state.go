package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
)

// -----------------------------------------------------------------------------
// CreateTaskState
// -----------------------------------------------------------------------------

type CreateStateInput struct {
	WorkflowState  *workflow.State  `json:"workflow_state"`
	WorkflowConfig *workflow.Config `json:"workflow_config"`
	TaskConfig     *task.Config     `json:"task_config"`
}

type CreateState struct {
	taskRepo    task.Repository
	configStore services.ConfigStore
}

func NewCreateState(taskRepo task.Repository, configStore services.ConfigStore) *CreateState {
	return &CreateState{
		taskRepo:    taskRepo,
		configStore: configStore,
	}
}

func (uc *CreateState) Execute(ctx context.Context, input *CreateStateInput) (*task.State, error) {
	taskExecID := core.MustNewID()
	// NOTE: Persist the task config before state creation so workers never observe a missing config.
	err := uc.configStore.Save(ctx, taskExecID.String(), input.TaskConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to save task config: %w", err)
	}
	state, err := uc.createBasicState(ctx, input, taskExecID)
	if err != nil {
		if deleteErr := uc.configStore.Delete(ctx, taskExecID.String()); deleteErr != nil {
			return nil, fmt.Errorf("state create failed (%w) and rollback failed (%v)", err, deleteErr)
		}
		return nil, err
	}
	// Note: Child config preparation is now handled by task2 infrastructure
	return state, nil
}

func (uc *CreateState) createBasicState(
	ctx context.Context,
	input *CreateStateInput,
	taskExecID core.ID,
) (*task.State, error) {
	envMap := input.TaskConfig.Env
	result, err := uc.processComponent(input, envMap)
	if err != nil {
		return nil, err
	}
	stateInput := task.CreateStateInput{
		WorkflowID:     input.WorkflowConfig.ID,
		WorkflowExecID: input.WorkflowState.WorkflowExecID,
		TaskID:         input.TaskConfig.ID,
		TaskExecID:     taskExecID,
	}
	taskState, err := task.CreateAndPersistState(ctx, uc.taskRepo, &stateInput, result)
	if err != nil {
		return nil, err
	}
	if err := input.TaskConfig.ValidateInput(ctx, taskState.Input); err != nil {
		return nil, fmt.Errorf("failed to validate task params: %w", err)
	}
	return taskState, nil
}

func (uc *CreateState) processComponent(
	input *CreateStateInput,
	baseEnv *core.EnvMap,
) (*task.PartialState, error) {
	executionType := input.TaskConfig.GetExecType()
	agentConfig := input.TaskConfig.GetAgent()
	toolConfig := input.TaskConfig.GetTool()
	switch {
	case input.TaskConfig.Type == task.TaskTypeParallel:
		return uc.processParallelTask(input, baseEnv)
	case input.TaskConfig.Type == task.TaskTypeCollection:
		return uc.processCollectionTask(input, baseEnv)
	case agentConfig != nil:
		return uc.processAgent(agentConfig, executionType, input.TaskConfig.Action)
	case toolConfig != nil:
		return uc.processTool(toolConfig, executionType)
	default:
		var actionID *string
		if input.TaskConfig.Action != "" {
			actionID = &input.TaskConfig.Action
		}
		return &task.PartialState{
			Component:     core.ComponentTask,
			ExecutionType: executionType,
			Input:         input.TaskConfig.With,
			ActionID:      actionID,
			MergedEnv:     baseEnv,
		}, nil
	}
}

func (uc *CreateState) processAgent(
	agentConfig *agent.Config,
	executionType task.ExecutionType,
	actionID string,
) (*task.PartialState, error) {
	agentID := agentConfig.ID
	return &task.PartialState{
		Component:     core.ComponentAgent,
		ExecutionType: executionType,
		AgentID:       &agentID,
		ActionID:      &actionID,
		Input:         agentConfig.With,
		MergedEnv:     agentConfig.Env,
	}, nil
}

func (uc *CreateState) processTool(
	toolConfig *tool.Config,
	executionType task.ExecutionType,
) (*task.PartialState, error) {
	toolID := toolConfig.ID
	return &task.PartialState{
		Component:     core.ComponentTool,
		ExecutionType: executionType,
		ToolID:        &toolID,
		Input:         toolConfig.With,
		MergedEnv:     toolConfig.Env,
	}, nil
}

func (uc *CreateState) processParallelTask(
	input *CreateStateInput,
	baseEnv *core.EnvMap,
) (*task.PartialState, error) {
	parentInput := input.TaskConfig.With
	if parentInput == nil {
		parentInput = &core.Input{}
	}
	return task.CreateParentPartialState(
		parentInput,
		baseEnv,
	), nil
}

func (uc *CreateState) processCollectionTask(
	input *CreateStateInput,
	baseEnv *core.EnvMap,
) (*task.PartialState, error) {
	parentInput := input.TaskConfig.With
	if parentInput == nil {
		parentInput = &core.Input{}
	}
	return task.CreateParentPartialStateWithExecType(
		parentInput,
		baseEnv,
		task.ExecutionCollection,
	), nil
}
