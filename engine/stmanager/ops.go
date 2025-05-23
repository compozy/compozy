package stmanager

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/agent"
	"github.com/compozy/compozy/engine/domain/project"
	"github.com/compozy/compozy/engine/domain/task"
	"github.com/compozy/compozy/engine/domain/tool"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/pkg/nats"
)

// -----------------------------------------------------------------------------
// State Retrieval
// -----------------------------------------------------------------------------

func (m *Manager) GetWorkflowState(corrID common.ID, execID common.ID) (state.State, error) {
	id := state.NewID(nats.ComponentWorkflow, corrID, execID)
	return m.store.GetState(id)
}

func (m *Manager) GetTaskState(corrID common.ID, execID common.ID) (state.State, error) {
	id := state.NewID(nats.ComponentTask, corrID, execID)
	return m.store.GetState(id)
}

func (m *Manager) GetAgentState(corrID common.ID, execID common.ID) (state.State, error) {
	id := state.NewID(nats.ComponentAgent, corrID, execID)
	return m.store.GetState(id)
}

func (m *Manager) GetToolState(corrID common.ID, execID common.ID) (state.State, error) {
	id := state.NewID(nats.ComponentTool, corrID, execID)
	return m.store.GetState(id)
}

func (m *Manager) GetTaskStatesForWorkflow(corrID common.ID, execID common.ID) ([]state.State, error) {
	id := state.NewID(nats.ComponentWorkflow, corrID, execID)
	return m.store.GetTaskStatesForWorkflow(id)
}

func (m *Manager) GetAgentStatesForTask(corrID common.ID, execID common.ID) ([]state.State, error) {
	id := state.NewID(nats.ComponentTask, corrID, execID)
	return m.store.GetAgentStatesForTask(id)
}

func (m *Manager) GetToolStatesForTask(corrID common.ID, execID common.ID) ([]state.State, error) {
	id := state.NewID(nats.ComponentTask, corrID, execID)
	return m.store.GetToolStatesForTask(id)
}

func (m *Manager) GetAllWorkflowStates() ([]state.State, error) {
	return m.store.GetStatesByComponent(nats.ComponentWorkflow)
}

func (m *Manager) GetAllTaskStates() ([]state.State, error) {
	return m.store.GetStatesByComponent(nats.ComponentTask)
}

func (m *Manager) GetAllAgentStates() ([]state.State, error) {
	return m.store.GetStatesByComponent(nats.ComponentAgent)
}

func (m *Manager) GetAllToolStates() ([]state.State, error) {
	return m.store.GetStatesByComponent(nats.ComponentTool)
}

// -----------------------------------------------------------------------------
// Delete
// -----------------------------------------------------------------------------

func (m *Manager) DeleteWorkflowState(corrID common.ID, execID common.ID) error {
	stateID := state.NewID(nats.ComponentWorkflow, corrID, execID)
	if err := m.store.DeleteState(stateID); err != nil {
		return fmt.Errorf("failed to delete workflow state: %w", err)
	}

	taskStates, err := m.store.GetTaskStatesForWorkflow(stateID)
	if err != nil {
		return fmt.Errorf("failed to get task states for workflow: %w", err)
	}

	for _, taskState := range taskStates {
		taskID := taskState.GetID()
		agentStates, err := m.store.GetAgentStatesForTask(taskID)
		if err != nil {
			return fmt.Errorf("failed to get agent states for task: %w", err)
		}

		for _, agState := range agentStates {
			if err := m.store.DeleteState(agState.GetID()); err != nil {
				return fmt.Errorf("failed to delete agent state: %w", err)
			}
		}

		toolStates, err := m.store.GetToolStatesForTask(taskID)
		if err != nil {
			return fmt.Errorf("failed to get tool states for task: %w", err)
		}

		for _, toolState := range toolStates {
			if err := m.store.DeleteState(toolState.GetID()); err != nil {
				return fmt.Errorf("failed to delete tool state: %w", err)
			}
		}

		if err := m.store.DeleteState(taskID); err != nil {
			return fmt.Errorf("failed to delete task state: %w", err)
		}
	}

	return nil
}

// -----------------------------------------------------------------------------
// Load
// -----------------------------------------------------------------------------

func (m *Manager) LoadWorkflowState(stateID state.ID) (*workflow.State, error) {
	state, err := m.GetWorkflowState(stateID.CorrID, stateID.ExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow state: %w", err)
	}
	if state.GetStatus() != nats.StatusRunning {
		return nil, fmt.Errorf("workflow is not in running state")
	}
	wfState, ok := state.(*workflow.State)
	if !ok {
		return nil, fmt.Errorf("failed to cast workflow state: %w", err)
	}
	return wfState, nil
}

func (m *Manager) LoadTaskState(stateID state.ID) (*task.State, error) {
	state, err := m.GetTaskState(stateID.CorrID, stateID.ExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task state: %w", err)
	}
	if state.GetStatus() != nats.StatusRunning {
		return nil, fmt.Errorf("task is not in running state")
	}
	taskState, ok := state.(*task.State)
	if !ok {
		return nil, fmt.Errorf("failed to cast task state")
	}
	return taskState, nil
}

func (m *Manager) LoadAgentState(stateID state.ID) (*agent.State, error) {
	state, err := m.GetAgentState(stateID.CorrID, stateID.ExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent state: %w", err)
	}
	if state.GetStatus() != nats.StatusRunning {
		return nil, fmt.Errorf("agent is not in running state")
	}
	agentState, ok := state.(*agent.State)
	if !ok {
		return nil, fmt.Errorf("failed to cast agent state")
	}
	return agentState, nil
}

func (m *Manager) LoadToolState(stateID state.ID) (*tool.State, error) {
	state, err := m.GetToolState(stateID.CorrID, stateID.ExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool state: %w", err)
	}
	if state.GetStatus() != nats.StatusRunning {
		return nil, fmt.Errorf("tool is not in running state")
	}
	toolState, ok := state.(*tool.State)
	if !ok {
		return nil, fmt.Errorf("failed to cast tool state")
	}
	return toolState, nil
}

// -----------------------------------------------------------------------------
// Create
// -----------------------------------------------------------------------------

func (m *Manager) CreateTaskState(metadata *task.Metadata, config *task.Config) (*task.State, error) {
	wState, err := m.LoadWorkflowState(metadata.WorkflowStateID)
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
	if err := m.store.UpsertState(state); err != nil {
		return nil, fmt.Errorf("failed to save state: %w", err)
	}
	return state, nil
}

func (m *Manager) CreateAgentState(metadata *agent.Metadata, config *agent.Config) (*agent.State, error) {
	wState, err := m.LoadWorkflowState(metadata.WorkflowStateID)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow state: %w", err)
	}
	taskState, err := m.LoadTaskState(metadata.TaskStateID)
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
	if err := m.store.UpsertState(state); err != nil {
		return nil, fmt.Errorf("failed to save agent state: %w", err)
	}
	return state, nil
}

func (m *Manager) CreateToolState(metadata *tool.Metadata, config *tool.Config) (*tool.State, error) {
	wState, err := m.LoadWorkflowState(metadata.WorkflowStateID)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow state: %w", err)
	}
	taskState, err := m.LoadTaskState(metadata.TaskStateID)
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
	if err := m.store.UpsertState(state); err != nil {
		return nil, fmt.Errorf("failed to save tool state: %w", err)
	}
	return state, nil
}

func (m *Manager) CreateWorkflowState(
	metadata *workflow.Metadata,
	triggerInput *common.Input,
	pConfig *project.Config,
	wConfig *workflow.Config,
) (*workflow.State, error) {
	ctx, err := workflow.NewContext(
		metadata,
		triggerInput,
		pConfig.GetEnv(),
		wConfig.GetEnv(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution: %w", err)
	}
	state, err := workflow.NewState(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create state: %w", err)
	}
	if err := m.store.UpsertState(state); err != nil {
		return nil, fmt.Errorf("failed to save state: %w", err)
	}
	return state, nil
}
