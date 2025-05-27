package core

import (
	"maps"
	"time"
)

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
	AsMap() map[ID]any
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
	component ComponentType,
	workflowID string,
	workflowExecID ID,
	parentInput, input *Input,
	output *Output,
	env *EnvMap,
	err *Error,
) *BaseExecution {
	return &BaseExecution{
		Component:      component,
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

func (b *BaseExecution) AsMap() map[ID]any {
	return map[ID]any{
		"exec_id":      b.GetID(),
		"component_id": b.GetComponentID(),
		"status":       b.GetStatus(),
		"parent":       b.GetParentInput(),
		"input":        b.GetInput(),
		"output":       b.GetOutput(),
		"error":        b.GetError(),
		"start_time":   b.StartTime,
		"end_time":     b.EndTime,
		"duration":     b.Duration,
	}
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
