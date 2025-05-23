package stmanager

import (
	"fmt"

	"github.com/compozy/compozy/engine/domain/agent"
	"github.com/compozy/compozy/engine/domain/task"
	"github.com/compozy/compozy/engine/domain/tool"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/pkg/nats"
)

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
