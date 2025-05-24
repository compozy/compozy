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

func (m *Manager) GetTaskStateForWorkflow(corrID common.ID) ([]state.State, error) {
	taskStates, err := m.store.GetStatesByComponent(nats.ComponentTask)
	if err != nil {
		return nil, err
	}
	var filteredStates []state.State
	for _, state := range taskStates {
		if state.GetCorrelationID() == corrID {
			filteredStates = append(filteredStates, state)
		}
	}
	return filteredStates, nil
}

func (m *Manager) GetAgentStateForWorkflow(corrID common.ID) ([]state.State, error) {
	agentStates, err := m.store.GetStatesByComponent(nats.ComponentAgent)
	if err != nil {
		return nil, err
	}
	var filteredStates []state.State
	for _, state := range agentStates {
		if state.GetCorrelationID() == corrID {
			filteredStates = append(filteredStates, state)
		}
	}
	return filteredStates, nil
}

func (m *Manager) GetToolStateForWorkflow(corrID common.ID) ([]state.State, error) {
	toolStates, err := m.store.GetStatesByComponent(nats.ComponentTool)
	if err != nil {
		return nil, err
	}
	var filteredStates []state.State
	for _, state := range toolStates {
		if state.GetCorrelationID() == corrID {
			filteredStates = append(filteredStates, state)
		}
	}
	return filteredStates, nil
}

// -----------------------------------------------------------------------------
// Delete
// -----------------------------------------------------------------------------

func (m *Manager) DeleteWorkflowState(corrID common.ID, execID common.ID) error {
	stateID := state.NewID(nats.ComponentWorkflow, corrID, execID)
	if err := m.store.DeleteState(stateID); err != nil {
		return fmt.Errorf("failed to delete workflow state: %w", err)
	}

	// Get all task states for this workflow
	taskStates, err := m.GetTaskStateForWorkflow(corrID)
	if err != nil {
		return fmt.Errorf("failed to get task states for workflow: %w", err)
	}

	// Delete all task states
	for _, taskState := range taskStates {
		if err := m.store.DeleteState(taskState.GetID()); err != nil {
			return fmt.Errorf("failed to delete task state: %w", err)
		}
	}

	// Get all agent states for this workflow
	agentStates, err := m.GetAgentStateForWorkflow(corrID)
	if err != nil {
		return fmt.Errorf("failed to get agent states for workflow: %w", err)
	}

	// Delete all agent states
	for _, agentState := range agentStates {
		if err := m.store.DeleteState(agentState.GetID()); err != nil {
			return fmt.Errorf("failed to delete agent state: %w", err)
		}
	}

	// Get all tool states for this workflow
	toolStates, err := m.GetToolStateForWorkflow(corrID)
	if err != nil {
		return fmt.Errorf("failed to get tool states for workflow: %w", err)
	}

	// Delete all tool states
	for _, toolState := range toolStates {
		if err := m.store.DeleteState(toolState.GetID()); err != nil {
			return fmt.Errorf("failed to delete tool state: %w", err)
		}
	}

	return nil
}
