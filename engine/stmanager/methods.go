package stmanager

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
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
	stID := state.NewID(nats.ComponentWorkflow, corrID, execID)
	if err := m.store.DeleteState(stID); err != nil {
		return fmt.Errorf("failed to delete workflow state: %w", err)
	}

	taskStates, err := m.store.GetTaskStatesForWorkflow(stID)
	if err != nil {
		return fmt.Errorf("failed to get task states for workflow: %w", err)
	}

	for _, taskState := range taskStates {
		tID := taskState.GetID()
		agentStates, err := m.store.GetAgentStatesForTask(tID)
		if err != nil {
			return fmt.Errorf("failed to get agent states for task: %w", err)
		}

		for _, agState := range agentStates {
			if err := m.store.DeleteState(agState.GetID()); err != nil {
				return fmt.Errorf("failed to delete agent state: %w", err)
			}
		}

		toolStates, err := m.store.GetToolStatesForTask(tID)
		if err != nil {
			return fmt.Errorf("failed to get tool states for task: %w", err)
		}

		for _, toolState := range toolStates {
			if err := m.store.DeleteState(toolState.GetID()); err != nil {
				return fmt.Errorf("failed to delete tool state: %w", err)
			}
		}

		if err := m.store.DeleteState(tID); err != nil {
			return fmt.Errorf("failed to delete task state: %w", err)
		}
	}

	return nil
}
