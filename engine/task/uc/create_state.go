package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/normalizer"
)

// -----------------------------------------------------------------------------
// CreateTaskState
// -----------------------------------------------------------------------------

type CreateStateInput struct {
	WorkflowID     string  `json:"workflow_id"`
	WorkflowExecID core.ID `json:"workflow_exec_id"`
	TaskID         string  `json:"task_id"`
	WorkflowState  *workflow.State
	WorkflowConfig *workflow.Config
	TaskConfig     *task.Config
}

type CreateStateOutput struct {
	State *task.State
}

type CreateState struct {
	taskRepo   task.Repository
	normalizer *normalizer.ConfigNormalizer
}

func NewCreateState(taskRepo task.Repository) *CreateState {
	return &CreateState{
		taskRepo:   taskRepo,
		normalizer: normalizer.NewConfigNormalizer(),
	}
}

func (uc *CreateState) Execute(ctx context.Context, input *CreateStateInput) (*CreateStateOutput, error) {
	taskConfigs := normalizer.BuildTaskConfigsMap(input.WorkflowConfig.Tasks)
	baseEnv, err := uc.normalizer.NormalizeTask(
		input.WorkflowState,
		input.WorkflowConfig,
		input.TaskConfig,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize task %s for workflow %s: %w",
			input.TaskConfig.ID, input.WorkflowConfig.ID, err)
	}
	result, err := uc.processComponent(input, taskConfigs, baseEnv)
	if err != nil {
		return nil, err
	}
	taskExecID := core.MustNewID()
	stateInput := task.StateInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
		TaskID:         input.TaskID,
		TaskExecID:     taskExecID,
	}
	taskState, err := task.CreateAndPersistState(ctx, uc.taskRepo, &stateInput, result)
	if err != nil {
		return nil, err
	}
	if err := input.TaskConfig.ValidateInput(ctx, taskState.Input); err != nil {
		return nil, fmt.Errorf("failed to validate task params: %w", err)
	}
	return &CreateStateOutput{
		State: taskState,
	}, nil
}

func (uc *CreateState) processComponent(
	input *CreateStateInput,
	taskConfigs map[string]*task.Config,
	baseEnv core.EnvMap,
) (*task.PartialState, error) {
	taskType := input.TaskConfig.Type
	if taskType == "" {
		taskType = task.TaskTypeBasic
	}
	var executionType task.ExecutionType
	switch taskType {
	case task.TaskTypeParallel:
		executionType = task.ExecutionParallel
	default:
		executionType = task.ExecutionBasic
	}
	agentConfig := input.TaskConfig.GetAgent()
	toolConfig := input.TaskConfig.GetTool()
	switch {
	case agentConfig != nil:
		return uc.processAgent(input, agentConfig, taskConfigs, executionType)
	case toolConfig != nil:
		return uc.processTool(input, toolConfig, taskConfigs, executionType)
	default:
		return &task.PartialState{
			Component:     core.ComponentTask,
			ExecutionType: executionType,
			Input:         input.TaskConfig.With,
			ActionID:      &input.TaskConfig.Action,
			MergedEnv:     baseEnv,
		}, nil
	}
}

func (uc *CreateState) processAgent(
	input *CreateStateInput,
	agentConfig *agent.Config,
	taskConfigs map[string]*task.Config,
	executionType task.ExecutionType,
) (*task.PartialState, error) {
	mergedEnv, err := uc.normalizer.NormalizeAgentComponent(
		input.WorkflowState,
		input.WorkflowConfig,
		input.TaskConfig,
		agentConfig,
		taskConfigs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to process agent component for task %s: %w",
			input.TaskConfig.ID, err)
	}
	agentID := agentConfig.ID
	return &task.PartialState{
		Component:     core.ComponentAgent,
		ExecutionType: executionType,
		AgentID:       &agentID,
		ActionID:      &input.TaskConfig.Action,
		Input:         agentConfig.With,
		MergedEnv:     mergedEnv,
	}, nil
}

func (uc *CreateState) processTool(
	input *CreateStateInput,
	toolConfig *tool.Config,
	taskConfigs map[string]*task.Config,
	executionType task.ExecutionType,
) (*task.PartialState, error) {
	mergedEnv, err := uc.normalizer.NormalizeToolComponent(
		input.WorkflowState,
		input.WorkflowConfig,
		input.TaskConfig,
		toolConfig,
		taskConfigs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to process tool component for task %s: %w",
			input.TaskConfig.ID, err)
	}
	toolID := toolConfig.ID
	return &task.PartialState{
		Component:     core.ComponentTool,
		ExecutionType: executionType,
		ToolID:        &toolID,
		Input:         toolConfig.With,
		MergedEnv:     mergedEnv,
	}, nil
}
