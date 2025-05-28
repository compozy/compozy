package task

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
	TaskExecID     core.ID
}

func NewStoreKey(workflowExecID core.ID, taskExecID core.ID) StoreKey {
	return StoreKey{
		WorkflowExecID: workflowExecID,
		TaskExecID:     taskExecID,
	}
}

func (s *StoreKey) String() string {
	return fmt.Sprintf("workflow:%s:task:%s", s.WorkflowExecID, s.TaskExecID)
}

func (s *StoreKey) Bytes() []byte {
	return []byte(s.String())
}

// -----------------------------------------------------------------------------
// RequestData
// -----------------------------------------------------------------------------

type RequestData struct {
	*pb.TaskMetadata `json:"metadata"`
	ParentInput      *core.Input  `json:"parent_input"`
	TaskInput        *core.Input  `json:"task_input"`
	WorkflowEnv      *core.EnvMap `json:"workflow_env"`
	TaskEnv          *core.EnvMap `json:"task_env"`
}

func NewRequestData(
	metadata *pb.TaskMetadata,
	parentInput, taskInput *core.Input,
	workflowEnv, taskEnv *core.EnvMap,
) (*RequestData, error) {
	return &RequestData{
		TaskMetadata: metadata,
		ParentInput:  parentInput,
		TaskInput:    taskInput,
		WorkflowEnv:  workflowEnv,
		TaskEnv:      taskEnv,
	}, nil
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
		TaskExecID:     core.ID(r.TaskExecId),
	}
}

// -----------------------------------------------------------------------------
// Execution
// -----------------------------------------------------------------------------

type Execution struct {
	*core.BaseExecution
	TaskID      string       `json:"task_id"`
	TaskExecID  core.ID      `json:"task_exec_id"`
	RequestData *RequestData `json:"request_data,omitempty"`
}

func NewExecution(data *RequestData) (*Execution, error) {
	return NewExecutionWithContext(data, nil)
}

func NewExecutionWithContext(data *RequestData, mainExecMap *core.MainExecutionMap) (*Execution, error) {
	env, err := data.WorkflowEnv.Merge(*data.TaskEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to merge env: %w", err)
	}
	baseExec := core.NewBaseExecution(
		core.ComponentTask,
		data.WorkflowId,
		core.ID(data.WorkflowExecId),
		data.ParentInput,
		data.TaskInput,
		nil,
		&env,
		nil,
	)
	exec := &Execution{
		BaseExecution: baseExec,
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
	return e.TaskExecID
}

func (e *Execution) GetComponentID() string {
	return e.TaskID
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
		Input:          e.GetInput(),
		Output:         e.GetOutput(),
		Error:          e.GetError(),
		StartTime:      e.GetStartTime(),
		EndTime:        e.GetEndTime(),
		Duration:       e.GetDuration(),
	}
	return &execMap
}
