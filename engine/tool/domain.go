package tool

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
	ToolExecID     core.ID
}

func NewStoreKey(workflowExecID core.ID, toolExecID core.ID) StoreKey {
	return StoreKey{
		WorkflowExecID: workflowExecID,
		ToolExecID:     toolExecID,
	}
}

func (s *StoreKey) String() string {
	return fmt.Sprintf("workflow:%s:tool:%s", s.WorkflowExecID, s.ToolExecID)
}

func (s *StoreKey) Bytes() []byte {
	return []byte(s.String())
}

// -----------------------------------------------------------------------------
// RequestData
// -----------------------------------------------------------------------------

type RequestData struct {
	*pb.ToolMetadata `json:"metadata"`
	ParentInput      *core.Input  `json:"parent_input"`
	TaskInput        *core.Input  `json:"task_input"`
	ToolInput        *core.Input  `json:"tool_input"`
	TaskEnv          *core.EnvMap `json:"task_env"`
	ToolEnv          *core.EnvMap `json:"tool_env"`
}

func NewRequestData(
	metadata *pb.ToolMetadata,
	parentInput, taskInput, toolInput *core.Input,
	taskEnv, toolEnv *core.EnvMap,
) (*RequestData, error) {
	return &RequestData{
		ToolMetadata: metadata,
		ParentInput:  parentInput,
		TaskInput:    taskInput,
		ToolInput:    toolInput,
		TaskEnv:      taskEnv,
		ToolEnv:      toolEnv,
	}, nil
}

func (r *RequestData) GetToolExecID() core.ID {
	return core.ID(r.ToolExecId)
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
		ToolExecID:     core.ID(r.ToolExecId),
	}
}

// -----------------------------------------------------------------------------
// Execution
// -----------------------------------------------------------------------------

type Execution struct {
	*core.BaseExecution
	TaskID      string       `json:"task_id"`
	TaskExecID  core.ID      `json:"task_exec_id"`
	ToolID      string       `json:"tool_id"`
	ToolExecID  core.ID      `json:"tool_exec_id"`
	RequestData *RequestData `json:"request_data,omitempty"`
}

func NewExecution(data *RequestData) (*Execution, error) {
	return NewExecutionWithContext(data, nil)
}

func NewExecutionWithContext(data *RequestData, mainExecMap *core.MainExecutionMap) (*Execution, error) {
	env, err := data.TaskEnv.Merge(*data.ToolEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to merge env: %w", err)
	}
	input, err := data.ToolInput.Merge(*data.TaskInput)
	if err != nil {
		return nil, fmt.Errorf("failed to merge input: %w", err)
	}
	baseExec := core.NewBaseExecution(
		core.ComponentTool,
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
		ToolID:        data.ToolId,
		ToolExecID:    core.ID(data.ToolExecId),
		TaskID:        data.TaskId,
		TaskExecID:    core.ID(data.TaskExecId),
		RequestData:   data,
	}
	normalizer := tplengine.NewNormalizer()
	if err := normalizer.ParseExecutionWithContext(exec, mainExecMap); err != nil {
		return nil, fmt.Errorf("failed to parse execution: %w", err)
	}
	return exec, nil
}

func (e *Execution) StoreKey() []byte {
	storeKey := e.RequestData.ToStoreKey()
	return storeKey.Bytes()
}

func (e *Execution) GetID() core.ID {
	return e.ToolExecID
}

func (e *Execution) GetComponentID() string {
	return e.ToolID
}

func (e *Execution) AsMainExecMap() *core.MainExecutionMap {
	return nil
}

func (e *Execution) AsExecMap() *core.ExecutionMap {
	execMap := core.ExecutionMap{
		Status:         e.Status,
		Component:      e.Component,
		WorkflowID:     e.WorkflowID,
		WorkflowExecID: e.WorkflowExecID,
		TaskID:         e.TaskID,
		TaskExecID:     e.TaskExecID,
		ToolID:         &e.ToolID,
		ToolExecID:     &e.ToolExecID,
		Input:          e.GetInput(),
		Output:         e.GetOutput(),
		Error:          e.GetError(),
		StartTime:      e.GetStartTime(),
		EndTime:        e.GetEndTime(),
		Duration:       e.GetDuration(),
	}
	return &execMap
}
