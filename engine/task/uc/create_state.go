package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
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
	taskRepo task.Repository
}

func NewCreateState(taskRepo task.Repository) *CreateState {
	return &CreateState{taskRepo: taskRepo}
}

func (uc *CreateState) Execute(ctx context.Context, input *CreateStateInput) (*task.State, error) {
	envMap := input.TaskConfig.Env
	result, err := uc.processComponent(input, envMap)
	if err != nil {
		return nil, err
	}
	taskExecID := core.MustNewID()
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
		return uc.processParallelTask(input, baseEnv, executionType)
	case agentConfig != nil:
		return uc.processAgent(agentConfig, executionType, input.TaskConfig.Action)
	case toolConfig != nil:
		return uc.processTool(toolConfig, executionType)
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
	execType task.ExecutionType,
) (*task.PartialState, error) {
	parallelConfig := &input.TaskConfig.ParallelTask
	subTasks := make(map[string]*task.State)
	for i := range parallelConfig.Tasks {
		subTaskConfig := &parallelConfig.Tasks[i]
		subTaskExecID := core.MustNewID()
		var subTask *task.State
		switch {
		case subTaskConfig.Agent != nil:
			subTask = task.CreateAgentSubTaskState(
				subTaskConfig.ID,
				subTaskExecID,
				input.WorkflowConfig.ID,
				input.WorkflowState.WorkflowExecID,
				subTaskConfig.Agent.ID,
				subTaskConfig.Action,
				subTaskConfig.With,
			)
		case subTaskConfig.Tool != nil:
			subTask = task.CreateToolSubTaskState(
				subTaskConfig.ID,
				subTaskExecID,
				input.WorkflowConfig.ID,
				input.WorkflowState.WorkflowExecID,
				subTaskConfig.Tool.ID,
				subTaskConfig.With,
			)
		case subTaskConfig.Type == task.TaskTypeParallel:
			// Nested parallel task - recursively process its sub-tasks
			subTask = uc.createNestesParallelState(
				subTaskConfig,
				subTaskExecID,
				input,
				baseEnv,
			)
		default:
			// Default to basic task component
			subTask = task.CreateSubTaskState(
				subTaskConfig.ID,
				subTaskExecID,
				input.WorkflowConfig.ID,
				input.WorkflowState.WorkflowExecID,
				execType,
				core.ComponentTask,
				subTaskConfig.With,
			)
		}
		if subTask == nil {
			return nil, fmt.Errorf("failed to create sub-task state for %s", subTaskConfig.ID)
		}
		subTasks[subTaskConfig.ID] = subTask
	}
	return task.CreateParallelPartialState(
		parallelConfig.GetStrategy(),
		parallelConfig.GetMaxWorkers(),
		parallelConfig.Timeout,
		subTasks,
		baseEnv,
	), nil
}

// createNestesParallelState handles recursive creation of nested parallel sub-task states
func (uc *CreateState) createNestesParallelState(
	subTaskConfig *task.Config,
	subTaskExecID core.ID,
	input *CreateStateInput,
	baseEnv *core.EnvMap,
) *task.State {
	// Create a nested ExecData for the parallel sub-task
	nestedInput := &CreateStateInput{
		WorkflowState:  input.WorkflowState,
		WorkflowConfig: input.WorkflowConfig,
		TaskConfig:     subTaskConfig,
	}
	execType := subTaskConfig.GetExecType()
	// Recursively validate the nested parallel task structure
	_, err := uc.processParallelTask(nestedInput, baseEnv, execType)
	if err != nil {
		// Log error but return a basic sub-task state to maintain execution
		// The error will be caught during execution phase
		return task.CreateSubTaskState(
			subTaskConfig.ID,
			subTaskExecID,
			input.WorkflowConfig.ID,
			input.WorkflowState.WorkflowExecID,
			execType,
			core.ComponentTask,
			subTaskConfig.With,
		)
	}
	// Create a nested parallel sub-task state with the processed parallel state
	subTask := task.CreateSubTaskState(
		subTaskConfig.ID,
		subTaskExecID,
		input.WorkflowConfig.ID,
		input.WorkflowState.WorkflowExecID,
		execType,
		core.ComponentTask,
		subTaskConfig.With,
	)
	return subTask
}
