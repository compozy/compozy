package core

import (
	"maps"
	"time"
)

type JSONMap struct {
	ComponentID string  `json:"component_id"`
	ExecID      ID      `json:"exec_id"`
	Status      string  `json:"status"`
	Parent      *Input  `json:"parent,omitempty"`
	Input       *Input  `json:"input,omitempty"`
	Output      *Output `json:"output,omitempty"`
	Error       *Error  `json:"error,omitempty"`
}

func JSONMapFromExecution(execution Execution) JSONMap {
	return JSONMap{
		ComponentID: execution.GetComponentID(),
		ExecID:      execution.GetID(),
		Status:      string(execution.GetStatus()),
		Parent:      execution.GetParentInput(),
		Input:       execution.GetInput(),
		Output:      execution.GetOutput(),
		Error:       execution.GetError(),
	}
}

func (jm *JSONMap) AsMap() map[ID]any {
	return map[ID]any{
		"component_id": jm.ComponentID,
		"exec_id":      jm.ExecID,
		"status":       jm.Status,
		"parent":       jm.Parent,
		"input":        jm.Input,
		"output":       jm.Output,
		"error":        jm.Error,
	}
}

type ExecutionMap struct {
	ExecID      ID             `json:"exec_id"`
	ComponentID string         `json:"component_id"`
	Status      string         `json:"status"`
	Input       *Input         `json:"input"`
	Output      *Output        `json:"output"`
	Error       *Error         `json:"error"`
	Tasks       map[ID]JSONMap `json:"tasks"`
	Agents      map[ID]JSONMap `json:"agents"`
	Tools       map[ID]JSONMap `json:"tools"`
}

func NewExecutionMap(execution Execution, tasksMap, agentsMap, toolsMap map[ID]JSONMap) *ExecutionMap {
	return &ExecutionMap{
		ExecID:      execution.GetID(),
		ComponentID: execution.GetComponentID(),
		Status:      string(execution.GetStatus()),
		Input:       execution.GetInput(),
		Output:      execution.GetOutput(),
		Error:       execution.GetError(),
		Tasks:       tasksMap,
		Agents:      agentsMap,
		Tools:       toolsMap,
	}
}

// -----------------------------------------------------------------------------
// Base Execution
// -----------------------------------------------------------------------------

type Execution interface {
	StoreKey() []byte
	IsRunning() bool
	GetID() ID
	GetWorkflowID() string
	GetWorkflowExecID() ID
	GetComponent() ComponentType
	GetComponentID() string
	GetStatus() StatusType
	GetEnv() *EnvMap
	GetParentInput() *Input
	GetInput() *Input
	GetOutput() *Output
	GetError() *Error
	SetDuration()
	CalcDuration() time.Duration
}

type BaseExecution struct {
	Component      ComponentType `json:"component"`
	WorkflowID     string        `json:"workflow_id"`
	WorkflowExecID ID            `json:"workflow_exec_id"`
	Status         StatusType    `json:"status"`
	ParentInput    *Input        `json:"parent_input,omitempty"`
	Input          *Input        `json:"input,omitempty"`
	Output         *Output       `json:"output,omitempty"`
	Env            *EnvMap       `json:"env,omitempty"`
	Error          *Error        `json:"error,omitempty"`
	StartTime      time.Time     `json:"start_time"`
	EndTime        time.Time     `json:"end_time"`
	Duration       time.Duration `json:"duration"`
}

func NewBaseExecution(
	workflowID string,
	workflowExecID ID,
	parentInput, input *Input,
	output *Output,
	env *EnvMap,
	err *Error,
) *BaseExecution {
	return &BaseExecution{
		WorkflowID:     workflowID,
		WorkflowExecID: workflowExecID,
		Status:         StatusPending,
		ParentInput:    parentInput,
		Input:          input,
		Output:         output,
		Env:            env,
		Error:          err,
		StartTime:      time.Now(),
	}
}

func (b *BaseExecution) StoreKey() []byte {
	return nil
}

func (b *BaseExecution) GetComponent() ComponentType {
	return b.Component
}

func (b *BaseExecution) GetID() ID {
	// The ID is the equivalent ExecID off each entity
	return MustNewID()
}

func (b *BaseExecution) GetComponentID() string {
	return ""
}

func (b *BaseExecution) GetWorkflowID() string {
	return b.WorkflowID
}

func (b *BaseExecution) GetWorkflowExecID() ID {
	return b.WorkflowExecID
}

func (b *BaseExecution) GetStatus() StatusType {
	return b.Status
}

func (b *BaseExecution) GetEnv() *EnvMap {
	return b.Env
}

func (b *BaseExecution) GetParentInput() *Input {
	return b.ParentInput
}

func (b *BaseExecution) GetInput() *Input {
	return b.Input
}

func (b *BaseExecution) GetOutput() *Output {
	return b.Output
}

func (b *BaseExecution) GetError() *Error {
	return b.Error
}

func (b *BaseExecution) IsRunning() bool {
	return b.Status == "running"
}

func (b *BaseExecution) SetDuration() {
	b.EndTime = time.Now()
	b.Duration = b.CalcDuration()
}

func (b *BaseExecution) CalcDuration() time.Duration {
	return b.EndTime.Sub(b.StartTime)
}

// -----------------------------------------------------------------------------
// Result from Event
// -----------------------------------------------------------------------------

func SetExecutionError(execution *BaseExecution, err ErrorPayload) {
	if err == nil {
		return
	}
	execution.Output = nil
	execution.Error = &Error{
		Message: err.GetMessage(),
	}
	if err.GetCode() != "" {
		execution.Error.Code = err.GetCode()
	}
	if err.GetDetails() != nil {
		execution.Error.Details = err.GetDetails().AsMap()
	}
}

func SetExecutionResult(execution *BaseExecution, payload EventDetailsSuccess) {
	output := payload.GetResult()
	if output == nil {
		return
	}
	execution.Error = nil
	res := &Output{}
	maps.Copy((*res), output.AsMap())
	execution.Output = res
}
