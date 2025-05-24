package stmanager

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/agent"
	"github.com/compozy/compozy/engine/domain/project"
	"github.com/compozy/compozy/engine/domain/task"
	"github.com/compozy/compozy/engine/domain/tool"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/pkg/pb"
)

// -----------------------------------------------------------------------------
// Create
// -----------------------------------------------------------------------------

func (m *Manager) CreateAgentState(metadata *pb.AgentMetadata, config *agent.Config) (*agent.State, error) {
	stateID := agent.GetWorkflowStateID(metadata)
	wState, err := m.LoadWorkflowState(stateID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow state: %w", err)
	}
	taskStateID := agent.GetTaskStateID(metadata)
	taskState, err := m.LoadTaskState(taskStateID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to load task state: %w", err)
	}
	ctx, err := agent.NewContext(
		metadata,
		*taskState.GetEnv(),
		config.GetEnv(),
		wState.GetTrigger(),
		taskState.GetInput(),
		config.With,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent context: %w", err)
	}
	state, err := agent.NewState(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent state: %w", err)
	}
	if err := config.ValidateParams(state.Input); err != nil {
		return nil, fmt.Errorf("failed to validate input: %w", err)
	}
	if err := m.store.UpsertState(state); err != nil {
		return nil, fmt.Errorf("failed to save agent state: %w", err)
	}
	return state, nil
}

func (m *Manager) CreateTaskState(metadata *pb.TaskMetadata, config *task.Config) (*task.State, error) {
	stateID := task.GetWorkflowStateID(metadata)
	wState, err := m.LoadWorkflowState(stateID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow state: %w", err)
	}
	ctx, err := task.NewContext(
		metadata,
		*wState.GetEnv(),
		config.GetEnv(),
		wState.GetTrigger(),
		config.With,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create task context: %w", err)
	}
	state, err := task.NewState(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create state: %w", err)
	}
	if err := config.ValidateParams(state.Input); err != nil {
		return nil, fmt.Errorf("failed to validate input: %w", err)
	}
	if err := m.store.UpsertState(state); err != nil {
		return nil, fmt.Errorf("failed to save state: %w", err)
	}
	return state, nil
}

func (m *Manager) CreateToolState(metadata *pb.ToolMetadata, config *tool.Config) (*tool.State, error) {
	stateID := tool.GetWorkflowStateID(metadata)
	wState, err := m.LoadWorkflowState(stateID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow state: %w", err)
	}
	taskStateID := tool.GetTaskStateID(metadata)
	taskState, err := m.LoadTaskState(taskStateID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to load task state: %w", err)
	}
	ctx, err := tool.NewContext(
		metadata,
		*taskState.GetEnv(),
		config.GetEnv(),
		wState.GetTrigger(),
		taskState.GetInput(),
		config.With,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool context: %w", err)
	}
	state, err := tool.NewState(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool state: %w", err)
	}
	if err := config.ValidateParams(config.With); err != nil {
		return nil, fmt.Errorf("failed to validate params: %w", err)
	}
	if err := m.store.UpsertState(state); err != nil {
		return nil, fmt.Errorf("failed to save tool state: %w", err)
	}
	return state, nil
}

func (m *Manager) CreateWorkflowState(
	metadata *pb.WorkflowMetadata,
	pConfig *project.Config,
	config *workflow.Config,
	triggerInput *common.Input,
) (*workflow.State, error) {
	ctx, err := workflow.NewContext(
		metadata,
		triggerInput,
		pConfig.GetEnv(),
		config.GetEnv(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution: %w", err)
	}
	state, err := workflow.NewState(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create state: %w", err)
	}
	if err := config.ValidateParams(state.Trigger); err != nil {
		return nil, fmt.Errorf("failed to validate params: %w", err)
	}
	if err := m.store.UpsertState(state); err != nil {
		return nil, fmt.Errorf("failed to save state: %w", err)
	}
	return state, nil
}
