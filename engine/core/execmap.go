package core

import (
	"time"
)

// -----------------------------------------------------------------------------
// ChildrenExecutionMap
// -----------------------------------------------------------------------------

type ExecutionMap struct {
	Status         StatusType    `json:"status"`
	Component      ComponentType `json:"component"`
	WorkflowID     string        `json:"workflow_id"`
	WorkflowExecID ID            `json:"workflow_exec_id"`
	TaskID         string        `json:"task_id"`
	TaskExecID     ID            `json:"task_exec_id"`
	AgentID        *string       `json:"agent_id,omitempty"`
	AgentExecID    *ID           `json:"agent_exec_id,omitempty"`
	ToolID         *string       `json:"tool_id,omitempty"`
	ToolExecID     *ID           `json:"tool_exec_id,omitempty"`
	Input          *Input        `json:"input"`
	Output         *Output       `json:"output"`
	Error          *Error        `json:"error"`
	StartTime      time.Time     `json:"start_time"`
	EndTime        time.Time     `json:"end_time"`
	Duration       time.Duration `json:"duration"`
}

func (e *ExecutionMap) AsMap() map[ID]any {
	item := map[ID]any{
		"status":           e.Status,
		"component":        e.Component,
		"workflow_id":      e.WorkflowID,
		"workflow_exec_id": e.WorkflowExecID,
		"input":            e.Input,
		"output":           e.Output,
		"error":            e.Error,
		"start_time":       e.StartTime,
		"end_time":         e.EndTime,
		"duration":         e.Duration,
	}
	if e.TaskID != "" {
		item["task_id"] = e.TaskID
	}
	if e.TaskExecID != "" {
		item["task_exec_id"] = e.TaskExecID
	}
	if e.AgentID != nil {
		item["agent_id"] = e.AgentID
	}
	if e.AgentExecID != nil {
		item["agent_exec_id"] = e.AgentExecID
	}
	if e.ToolID != nil {
		item["tool_id"] = e.ToolID
	}
	if e.ToolExecID != nil {
		item["tool_exec_id"] = e.ToolExecID
	}
	return item
}

// -----------------------------------------------------------------------------
// ChildrenExecutions
// -----------------------------------------------------------------------------

type ChildrenExecutionMap struct {
	Tasks  []*ExecutionMap `json:"tasks"`
	Agents []*ExecutionMap `json:"agents"`
	Tools  []*ExecutionMap `json:"tools"`
}

// -----------------------------------------------------------------------------
// MainExecutionMap
// -----------------------------------------------------------------------------

type MainExecutionMap struct {
	ChildrenExecutionMap
	Status         StatusType    `json:"status"`
	Component      ComponentType `json:"component"`
	WorkflowID     string        `json:"workflow_id"`
	WorkflowExecID ID            `json:"workflow_exec_id"`
	Input          *Input        `json:"input"`
	Output         *Output       `json:"output"`
	Error          *Error        `json:"error"`
	StartTime      time.Time     `json:"start_time"`
	EndTime        time.Time     `json:"end_time"`
	Duration       time.Duration `json:"duration"`
}

func (e *MainExecutionMap) AsMap() map[ID]any {
	return map[ID]any{
		"status":           e.Status,
		"component":        e.Component,
		"workflow_id":      e.WorkflowID,
		"workflow_exec_id": e.WorkflowExecID,
		"input":            e.Input,
		"output":           e.Output,
		"error":            e.Error,
		"start_time":       e.StartTime,
		"end_time":         e.EndTime,
		"duration":         e.Duration,
		"tasks":            e.Tasks,
		"agents":           e.Agents,
		"tools":            e.Tools,
	}
}

func (e *MainExecutionMap) WithTasks(tasks []*ExecutionMap) {
	e.Tasks = tasks
}

func (e *MainExecutionMap) WithAgents(agents []*ExecutionMap) {
	e.Agents = agents
}

func (e *MainExecutionMap) WithTools(tools []*ExecutionMap) {
	e.Tools = tools
}
