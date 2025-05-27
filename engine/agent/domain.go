package agent

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/pb"
	"github.com/compozy/compozy/pkg/tplengine"
)

// -----------------------------------------------------------------------------
// StoreKey
// -----------------------------------------------------------------------------

type StoreKey struct {
	WorkflowExecID core.ID
	AgentExecID    core.ID
}

func NewStoreKey(workflowExecID core.ID, agentExecID core.ID) StoreKey {
	return StoreKey{
		WorkflowExecID: workflowExecID,
		AgentExecID:    agentExecID,
	}
}

func (s *StoreKey) String() string {
	return fmt.Sprintf("workflow:%s:agent:%s", s.WorkflowExecID, s.AgentExecID)
}

func (s *StoreKey) Bytes() []byte {
	return []byte(s.String())
}

// -----------------------------------------------------------------------------
// RequestData
// -----------------------------------------------------------------------------

type RequestData struct {
	*pb.AgentMetadata `json:"metadata"`
	ParentInput       *core.Input  `json:"parent_input"`
	TaskInput         *core.Input  `json:"task_input"`
	AgentInput        *core.Input  `json:"agent_input"`
	TaskEnv           *core.EnvMap `json:"task_env"`
	AgentEnv          *core.EnvMap `json:"agent_env"`
}

func NewRequestData(
	metadata *pb.AgentMetadata,
	parentInput, taskInput, agentInput *core.Input,
	taskEnv, agentEnv *core.EnvMap,
) (*RequestData, error) {
	return &RequestData{
		AgentMetadata: metadata,
		ParentInput:   parentInput,
		TaskInput:     taskInput,
		AgentInput:    agentInput,
		TaskEnv:       taskEnv,
		AgentEnv:      agentEnv,
	}, nil
}

func (r *RequestData) GetAgentExecID() core.ID {
	return core.ID(r.AgentExecId)
}

func (r *RequestData) GetTaskExecID() core.ID {
	return core.ID(r.TaskExecId)
}

func (r *RequestData) GetWorkflowExecID() core.ID {
	return core.ID(r.WorkflowExecId)
}

func (r *RequestData) ToStoreKey() StoreKey {
	return StoreKey{
		WorkflowExecID: core.ID(r.WorkflowExecId),
		AgentExecID:    core.ID(r.AgentExecId),
	}
}

// -----------------------------------------------------------------------------
// Execution
// -----------------------------------------------------------------------------

type Execution struct {
	*core.BaseExecution
	TaskID      string       `json:"task_id"`
	TaskExecID  core.ID      `json:"task_exec_id"`
	AgentID     string       `json:"agent_id"`
	AgentExecID core.ID      `json:"agent_exec_id"`
	RequestData *RequestData `json:"request_data,omitempty"`
}

func NewExecution(data *RequestData) (*Execution, error) {
	env, err := data.TaskEnv.Merge(*data.AgentEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to merge env: %w", err)
	}
	input, err := data.AgentInput.Merge(*data.TaskInput)
	if err != nil {
		return nil, fmt.Errorf("failed to merge input: %w", err)
	}
	baseExec := core.NewBaseExecution(
		core.ComponentAgent,
		data.WorkflowId,
		core.ID(data.WorkflowExecId),
		data.ParentInput,
		&input,
		nil,
		&env,
		nil,
	)
	exec := &Execution{
		BaseExecution: baseExec,
		AgentID:       data.AgentId,
		AgentExecID:   core.ID(data.AgentExecId),
		TaskID:        data.TaskId,
		TaskExecID:    core.ID(data.TaskExecId),
		RequestData:   data,
	}
	normalizer := tplengine.NewNormalizer()
	if err := normalizer.ParseExecution(exec); err != nil {
		return nil, fmt.Errorf("failed to parse execution: %w", err)
	}
	return exec, nil
}

func (e *Execution) StoreKey() []byte {
	storeKey := e.RequestData.ToStoreKey()
	return storeKey.Bytes()
}

func (e *Execution) GetID() core.ID {
	return e.AgentExecID
}

func (e *Execution) GetComponentID() string {
	return e.AgentID
}

func (e *Execution) AsMap() map[core.ID]any {
	return map[core.ID]any{
		"status":           e.GetStatus(),
		"component":        e.GetComponent(),
		"agent_exec_id":    e.GetID(),
		"agent_id":         e.GetComponentID(),
		"workflow_id":      e.GetWorkflowID(),
		"workflow_exec_id": e.GetWorkflowExecID(),
		"task_id":          e.TaskID,
		"task_exec_id":     e.TaskExecID,
		"input":            e.GetInput(),
		"output":           e.GetOutput(),
		"error":            e.GetError(),
		"start_time":       e.StartTime,
		"end_time":         e.EndTime,
		"duration":         e.Duration,
	}
}
