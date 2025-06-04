package activities

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

const DispatchLabel = "DispatchTask"

type DispatchInput struct {
	WorkflowID     string  `json:"workflow_id"`
	WorkflowExecID core.ID `json:"workflow_exec_id"`
	TaskID         string  `json:"task_id"`
}

type DispatchOutput struct {
	State  *task.State
	Config *task.Config
}

type DispatchData struct {
	WorkflowState  *workflow.State
	WorkflowConfig *workflow.Config
	TaskConfig     *task.Config
}

type Dispatch struct {
	workflows    []*workflow.Config
	workflowRepo workflow.Repository
	taskRepo     task.Repository
	normalizer   *normalizer.ConfigNormalizer
	taskConfigs  map[string]*task.Config
}

// NewDispatch creates a new Dispatch activity
func NewDispatch(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
) *Dispatch {
	return &Dispatch{
		workflows:    workflows,
		workflowRepo: workflowRepo,
		taskRepo:     taskRepo,
		normalizer:   normalizer.NewConfigNormalizer(),
	}
}

// Run executes a task
func (a *Dispatch) Run(ctx context.Context, input *DispatchInput) (*DispatchOutput, error) {
	// Load execution data
	execData, err := a.loadData(ctx, input)
	if err != nil {
		return nil, err
	}

	state, err := a.createState(ctx, execData, input)
	if err != nil {
		return nil, err
	}
	return &DispatchOutput{
		State:  state,
		Config: execData.TaskConfig,
	}, nil
}

func (a *Dispatch) createState(
	ctx context.Context,
	execData *DispatchData,
	input *DispatchInput,
) (*task.State, error) {
	a.taskConfigs = normalizer.BuildTaskConfigsMap(execData.WorkflowConfig.Tasks)
	baseEnv, err := a.normalizer.NormalizeTask(
		execData.WorkflowState,
		execData.WorkflowConfig,
		execData.TaskConfig,
	)
	if err != nil {
		errMsg := fmt.Sprintf(
			"failed to normalize task %s for workflow %s",
			execData.TaskConfig.ID,
			execData.WorkflowConfig.ID,
		)
		return nil, fmt.Errorf("%s: %w", errMsg, err)
	}

	// Process component and get result
	result, err := a.processComponent(execData, baseEnv)
	if err != nil {
		return nil, err
	}

	// Create and persist task state
	taskExecID := core.MustNewID()
	stateInput := task.StateInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
		TaskID:         input.TaskID,
		TaskExecID:     taskExecID,
	}
	taskState, err := task.CreateAndPersistState(ctx, a.taskRepo, &stateInput, result)
	if err != nil {
		return nil, err
	}
	if err := execData.TaskConfig.ValidateParams(ctx, taskState.Input); err != nil {
		return nil, fmt.Errorf("failed to validate task params: %w", err)
	}
	return taskState, nil
}

// loadData loads all necessary configurations
func (a *Dispatch) loadData(ctx context.Context, input *DispatchInput) (*DispatchData, error) {
	workflowState, err := a.workflowRepo.GetState(ctx, input.WorkflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow state: %w", err)
	}

	workflowConfig, err := workflow.FindConfig(a.workflows, input.WorkflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow config: %w", err)
	}

	taskID := input.TaskID
	if input.TaskID == "" {
		taskID = workflowConfig.Tasks[0].ID
	}
	input.TaskID = taskID
	taskConfig, err := task.FindConfig(workflowConfig.Tasks, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to find task config: %w", err)
	}

	return &DispatchData{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     taskConfig,
	}, nil
}

// processComponent processes the appropriate component (agent, tool, or task)
func (a *Dispatch) processComponent(
	execData *DispatchData,
	baseEnv core.EnvMap,
) (*task.PartialState, error) {
	agentConfig := execData.TaskConfig.GetAgent()
	toolConfig := execData.TaskConfig.GetTool()

	switch {
	case agentConfig != nil:
		return a.processAgent(execData, agentConfig)
	case toolConfig != nil:
		return a.processTool(execData, toolConfig)
	default:
		return &task.PartialState{
			Component: core.ComponentTask,
			Input:     execData.TaskConfig.With,
			ActionID:  &execData.TaskConfig.Action,
			MergedEnv: baseEnv,
		}, nil
	}
}

// processAgent processes an agent component
func (a *Dispatch) processAgent(
	execData *DispatchData,
	agentConfig *agent.Config,
) (*task.PartialState, error) {
	// Normalize agent configuration and get merged environment
	mergedEnv, err := a.normalizer.NormalizeAgentComponent(
		execData.WorkflowState,
		execData.WorkflowConfig,
		execData.TaskConfig,
		agentConfig,
		a.taskConfigs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to process agent component for task %s: %w", execData.TaskConfig.ID, err)
	}

	agentID := agentConfig.ID
	return &task.PartialState{
		Component: core.ComponentAgent,
		AgentID:   &agentID,
		ActionID:  &execData.TaskConfig.Action,
		Input:     agentConfig.With,
		MergedEnv: mergedEnv,
	}, nil
}

// processTool processes a tool component
func (a *Dispatch) processTool(
	execData *DispatchData,
	toolConfig *tool.Config,
) (*task.PartialState, error) {
	// Normalize tool configuration and get merged environment
	mergedEnv, err := a.normalizer.NormalizeToolComponent(
		execData.WorkflowState,
		execData.WorkflowConfig,
		execData.TaskConfig,
		toolConfig,
		a.taskConfigs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to process tool component for task %s: %w", execData.TaskConfig.ID, err)
	}

	toolID := toolConfig.ID
	return &task.PartialState{
		Component: core.ComponentTool,
		ToolID:    &toolID,
		Input:     toolConfig.With,
		MergedEnv: mergedEnv,
	}, nil
}
