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

func (m *Manager) LoadTaskState(stateID state.ID, checkStatus bool) (*task.State, error) {
	state, err := m.GetTaskState(stateID.CorrID, stateID.ExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task state: %w", err)
	}
	if checkStatus && state.GetStatus() != nats.StatusRunning {
		return nil, fmt.Errorf("task is not in running state")
	}
	taskState, ok := state.(*task.State)
	if !ok {
		return nil, fmt.Errorf("failed to cast task state")
	}
	return taskState, nil
}

func (m *Manager) LoadAgentState(stateID state.ID, checkStatus bool) (*agent.State, error) {
	state, err := m.GetAgentState(stateID.CorrID, stateID.ExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent state: %w", err)
	}
	if checkStatus && state.GetStatus() != nats.StatusRunning {
		return nil, fmt.Errorf("agent is not in running state")
	}
	agentState, ok := state.(*agent.State)
	if !ok {
		return nil, fmt.Errorf("failed to cast agent state")
	}
	return agentState, nil
}

func (m *Manager) LoadToolState(stateID state.ID, checkStatus bool) (*tool.State, error) {
	state, err := m.GetToolState(stateID.CorrID, stateID.ExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool state: %w", err)
	}
	if checkStatus && state.GetStatus() != nats.StatusRunning {
		return nil, fmt.Errorf("tool is not in running state")
	}
	toolState, ok := state.(*tool.State)
	if !ok {
		return nil, fmt.Errorf("failed to cast tool state")
	}
	return toolState, nil
}

func (m *Manager) LoadWorkflowState(stateID state.ID, checkStatus bool) (*workflow.State, error) {
	state, err := m.GetWorkflowState(stateID.CorrID, stateID.ExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow state: %w", err)
	}
	if checkStatus && state.GetStatus() != nats.StatusRunning {
		return nil, fmt.Errorf("workflow is not in running state")
	}
	wfState, ok := state.(*workflow.State)
	if !ok {
		return nil, fmt.Errorf("failed to cast workflow state: %w", err)
	}
	return wfState, nil
}

// -----------------------------------------------------------------------------
// Load State Maps
// -----------------------------------------------------------------------------

func (m *Manager) LoadTaskStateMap(stateID state.ID, checkStatus bool) (map[string]any, error) {
	taskState, err := m.LoadTaskState(stateID, checkStatus)
	if err != nil {
		return nil, err
	}
	jsonMap := state.NewJSONMap(taskState)
	return jsonMap.AsMap(), nil
}

func (m *Manager) LoadAgentStateMap(stateID state.ID, checkStatus bool) (map[string]any, error) {
	agentState, err := m.LoadAgentState(stateID, checkStatus)
	if err != nil {
		return nil, err
	}
	jsonMap := state.NewJSONMap(agentState)
	return jsonMap.AsMap(), nil
}

func (m *Manager) LoadToolStateMap(stateID state.ID, checkStatus bool) (map[string]any, error) {
	toolState, err := m.LoadToolState(stateID, checkStatus)
	if err != nil {
		return nil, err
	}
	jsonMap := state.NewJSONMap(toolState)
	return jsonMap.AsMap(), nil
}

func (m *Manager) LoadWorkflowStateMap(stateID state.ID) (map[string]any, map[string]any, error) {
	workflowState, err := m.LoadWorkflowState(stateID, false)
	if err != nil {
		return nil, nil, err
	}

	corrID := workflowState.StateID.CorrID
	jsonMap := state.NewJSONMap(workflowState)
	workflowMap := jsonMap.AsMap()
	tasksMap := make(map[string]any)
	agentsMap := make(map[string]any)
	toolsMap := make(map[string]any)

	taskStates, err := m.GetTaskStateForWorkflow(corrID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get task states: %w", err)
	}
	for _, taskState := range taskStates {
		taskID := taskState.GetComponentID()
		taskJSONMap := state.NewJSONMap(taskState)
		tasksMap[taskID] = taskJSONMap.AsMap()
	}

	agentStates, err := m.GetAgentStateForWorkflow(corrID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get agent states: %w", err)
	}
	for _, agentState := range agentStates {
		agentID := agentState.GetComponentID()
		agentJSONMap := state.NewJSONMap(agentState)
		agentsMap[agentID] = agentJSONMap.AsMap()
	}

	toolStates, err := m.GetToolStateForWorkflow(corrID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get tool states: %w", err)
	}
	for _, toolState := range toolStates {
		toolID := toolState.GetComponentID()
		toolJSONMap := state.NewJSONMap(toolState)
		toolsMap[toolID] = toolJSONMap.AsMap()
	}

	workflowMap["tasks"] = tasksMap
	workflowMap["agents"] = agentsMap
	workflowMap["tools"] = toolsMap
	return jsonMap.AsMap(), workflowMap, nil
}

func removeEnvFromMap(m map[string]any) {
	delete(m, "env")
	for _, value := range m {
		if nestedMap, ok := value.(map[string]any); ok {
			removeEnvFromMap(nestedMap)
		}
	}
}

func (m *Manager) LoadWorkflowStateMapSafe(stateID state.ID) (map[string]any, map[string]any, error) {
	workflowMap, fullMap, err := m.LoadWorkflowStateMap(stateID)
	if err != nil {
		return nil, nil, err
	}
	removeEnvFromMap(workflowMap)
	removeEnvFromMap(fullMap)
	return workflowMap, fullMap, nil
}
