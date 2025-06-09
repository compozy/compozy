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

// Constants for metadata keys
const (
	ParallelConfigKey = "_parallel_config"
	ChildConfigsKey   = "child_configs"
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
		return uc.processParallelTask(input, baseEnv)
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
	// Store parallel configuration in the parent task's input for child task creation
	parallelConfig := input.TaskConfig.ParallelTask
	// Create enriched input that includes parallel metadata
	parentInput := input.TaskConfig.With
	if parentInput == nil {
		parentInput = &core.Input{}
	}
	// Store parallel configuration and child task configs as metadata
	(*parentInput)[ParallelConfigKey] = map[string]any{
		"strategy":      parallelConfig.GetStrategy(),
		"max_workers":   parallelConfig.GetMaxWorkers(),
		"timeout":       parallelConfig.Timeout,
		ChildConfigsKey: parallelConfig.Tasks, // Store child task configurations
	}
	return task.CreateParentPartialState(
		parentInput,
		baseEnv,
	), nil
}

// CreateChildTasksInput follows Temporal best practices by passing minimal data
type CreateChildTasksInput struct {
	ParentStateID  core.ID `json:"parent_state_id"`
	WorkflowExecID core.ID `json:"workflow_exec_id"`
	WorkflowID     string  `json:"workflow_id"`
}

// CreateChildTasks creates child tasks for a parallel parent using lightweight input
func (uc *CreateState) CreateChildTasks(ctx context.Context, input *CreateChildTasksInput) error {
	// Retrieve parent state from database (Temporal best practice)
	parentState, err := uc.taskRepo.GetState(ctx, input.ParentStateID)
	if err != nil {
		return fmt.Errorf("failed to retrieve parent state: %w", err)
	}

	// Validate parent is a parallel task
	if !parentState.IsParentTask() {
		return fmt.Errorf("state %s is not a parent task", input.ParentStateID)
	}

	// Extract parallel configuration from parent's input metadata
	parallelMetaRaw, exists := (*parentState.Input)[ParallelConfigKey]
	if !exists {
		return fmt.Errorf("parent state missing parallel configuration metadata")
	}

	parallelMeta, ok := parallelMetaRaw.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid parallel configuration metadata format")
	}

	// Extract child task configurations
	childConfigsRaw, ok := parallelMeta[ChildConfigsKey]
	if !ok {
		return fmt.Errorf("parent state missing child configurations")
	}

	// Convert to task.Config slice (this handles the interface{} conversion)
	childConfigs, ok := childConfigsRaw.([]task.Config)
	if !ok {
		return fmt.Errorf("invalid child configurations format: expected []task.Config, got %T", childConfigsRaw)
	}

	// Create child tasks atomically using transaction
	return uc.createChildTasksInTransaction(ctx, parentState, childConfigs)
}

// createChildTasksInTransaction creates all child tasks atomically
func (uc *CreateState) createChildTasksInTransaction(
	ctx context.Context,
	parentState *task.State,
	childConfigs []task.Config,
) error {
	// Prepare all child states first
	var childStates []*task.State
	for i := range childConfigs {
		childConfig := &childConfigs[i]
		childTaskExecID := core.MustNewID()
		// Create child partial state by recursively processing the child config
		childPartialState, err := uc.processChildConfig(childConfig)
		if err != nil {
			return fmt.Errorf("failed to process child config %s: %w", childConfig.ID, err)
		}
		// Create child state input with parent reference
		childStateInput := &task.CreateStateInput{
			WorkflowID:     parentState.WorkflowID,
			WorkflowExecID: parentState.WorkflowExecID,
			TaskID:         childConfig.ID,
			TaskExecID:     childTaskExecID,
		}
		// Set parent relationship in partial state
		childPartialState.ParentStateID = &parentState.TaskExecID
		// Create child state (without persisting yet)
		childState := task.CreateBasicState(childStateInput, childPartialState)
		childStates = append(childStates, childState)
	}

	// Create all child states atomically in a single transaction
	return uc.taskRepo.CreateChildStatesInTransaction(ctx, parentState.TaskExecID, childStates)
}

// processChildConfig processes a child task config to create its partial state
func (uc *CreateState) processChildConfig(childConfig *task.Config) (*task.PartialState, error) {
	// Use the existing processComponent logic but for child config
	baseEnv := childConfig.Env
	executionType := childConfig.GetExecType()
	agentConfig := childConfig.GetAgent()
	toolConfig := childConfig.GetTool()

	switch {
	case childConfig.Type == task.TaskTypeParallel:
		// Nested parallel task - not yet supported in new architecture
		return nil, fmt.Errorf("nested parallel tasks not yet supported")
	case agentConfig != nil:
		return uc.processAgent(agentConfig, executionType, childConfig.Action)
	case toolConfig != nil:
		return uc.processTool(toolConfig, executionType)
	default:
		var actionID *string
		if childConfig.Action != "" {
			actionID = &childConfig.Action
		}
		return &task.PartialState{
			Component:     core.ComponentTask,
			ExecutionType: executionType,
			Input:         childConfig.With,
			ActionID:      actionID,
			MergedEnv:     baseEnv,
		}, nil
	}
}
