package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/tool"
)

// CreateChildTasksInput follows Temporal best practices by passing minimal data
type CreateChildTasksInput struct {
	ParentStateID  core.ID `json:"parent_state_id"`
	WorkflowExecID core.ID `json:"workflow_exec_id"`
	WorkflowID     string  `json:"workflow_id"`
}

type CreateChildTasks struct {
	taskRepo      task.Repository
	configManager *services.ConfigManager
}

func NewCreateChildTasksUC(taskRepo task.Repository, configManager *services.ConfigManager) *CreateChildTasks {
	return &CreateChildTasks{
		taskRepo:      taskRepo,
		configManager: configManager,
	}
}

func (uc *CreateChildTasks) Execute(ctx context.Context, input *CreateChildTasksInput) error {
	parentState, err := uc.taskRepo.GetState(ctx, input.ParentStateID)
	if err != nil {
		return fmt.Errorf("failed to retrieve parent state: %w", err)
	}

	if err := uc.validateParentState(parentState); err != nil {
		return err
	}

	switch parentState.ExecutionType {
	case task.ExecutionParallel:
		return uc.createParallelChildren(ctx, parentState)
	case task.ExecutionCollection:
		return uc.createCollectionChildren(ctx, parentState)
	default:
		return fmt.Errorf("unsupported execution type for child creation: %s", parentState.ExecutionType)
	}
}

func (uc *CreateChildTasks) createParallelChildren(ctx context.Context, parentState *task.State) error {
	metadata, err := uc.configManager.LoadParallelTaskMetadata(ctx, parentState.TaskExecID)
	if err != nil {
		return err
	}

	if err := uc.validateChildConfigs(metadata.ChildConfigs); err != nil {
		return err
	}

	return uc.createChildStatesInTransaction(ctx, parentState, metadata.ChildConfigs)
}

func (uc *CreateChildTasks) createCollectionChildren(ctx context.Context, parentState *task.State) error {
	metadata, err := uc.configManager.LoadCollectionTaskMetadata(ctx, parentState.TaskExecID)
	if err != nil {
		return err
	}

	if err := uc.validateChildConfigs(metadata.ChildConfigs); err != nil {
		return err
	}

	return uc.createChildStatesInTransaction(ctx, parentState, metadata.ChildConfigs)
}

// validateParentState validates that the parent state can have child tasks
func (uc *CreateChildTasks) validateParentState(parentState *task.State) error {
	if !parentState.CanHaveChildren() {
		return fmt.Errorf("state %s is not a parent task", parentState.TaskExecID)
	}
	return nil
}

func (uc *CreateChildTasks) validateChildConfigs(childConfigs []task.Config) error {
	for i := range childConfigs {
		if childConfigs[i].ID == "" {
			return fmt.Errorf("child config at index %d missing required ID field", i)
		}
	}
	return nil
}

// createChildStatesInTransaction creates all child tasks atomically
func (uc *CreateChildTasks) createChildStatesInTransaction(
	ctx context.Context,
	parentState *task.State,
	childConfigs []task.Config,
) error {
	// Prepare all child states first
	var childStates []*task.State
	for i := range childConfigs {
		childConfig := &childConfigs[i]
		childTaskExecID := core.MustNewID()

		// Save child configuration for later retrieval during execution
		if err := uc.configManager.SaveChildConfig(ctx, childTaskExecID, childConfig); err != nil {
			return fmt.Errorf("failed to save child config %s: %w", childConfig.ID, err)
		}

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
		parentID := parentState.TaskExecID
		childPartialState.ParentStateID = &parentID

		// Create child state (without persisting yet)
		childState := task.CreateBasicState(childStateInput, childPartialState)
		childStates = append(childStates, childState)
	}

	// Create all child states atomically in a single transaction
	err := uc.taskRepo.CreateChildStatesInTransaction(ctx, parentState.TaskExecID, childStates)
	if err != nil {
		return err
	}

	return nil
}

// processChildConfig processes a child task config to create its partial state
func (uc *CreateChildTasks) processChildConfig(childConfig *task.Config) (*task.PartialState, error) {
	// Use the existing processComponent logic but for child config
	baseEnv := childConfig.Env
	executionType := childConfig.GetExecType()
	agentConfig := childConfig.GetAgent()
	toolConfig := childConfig.GetTool()

	switch {
	case childConfig.Type == task.TaskTypeParallel:
		// TODO: Add support for nested parallel tasks
		return nil, fmt.Errorf("nested parallel tasks not yet supported")
	case childConfig.Type == task.TaskTypeCollection:
		// TODO: Add support for nested collection tasks
		return nil, fmt.Errorf("nested collection tasks not yet supported")
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

func (uc *CreateChildTasks) processAgent(
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

func (uc *CreateChildTasks) processTool(
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
