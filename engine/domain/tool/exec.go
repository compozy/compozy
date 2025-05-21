package tool

import (
	"github.com/compozy/compozy/engine/common"
)

// -----------------------------------------------------------------------------
// Execution
// -----------------------------------------------------------------------------

type Execution struct {
	CorrID         common.CorrID
	WorkflowExecID common.ExecID
	TaskExecID     common.ExecID
	ExecID         common.ExecID
	TaskEnv        common.EnvMap
	ToolEnv        common.EnvMap
	TriggerInput   *common.Input
	TaskInput      *common.Input
	ToolInput      *common.Input
}

func NewExecution(
	corrID common.CorrID,
	taskExecID, workflowExecID common.ExecID,
	taskEnv, toolEnv common.EnvMap,
	tgInput, taskInput, toolInput *common.Input,
) *Execution {
	execID := common.NewExecID()
	return &Execution{
		CorrID:         corrID,
		WorkflowExecID: workflowExecID,
		TaskExecID:     taskExecID,
		ExecID:         execID,
		TaskEnv:        taskEnv,
		ToolEnv:        toolEnv,
		TriggerInput:   tgInput,
		TaskInput:      taskInput,
		ToolInput:      toolInput,
	}
}

func (e *Execution) ToProtoBufMap() (map[string]any, error) {
	execMap := map[string]any{
		"corr_id":          e.CorrID.String(),
		"workflow_exec_id": e.WorkflowExecID.String(),
		"task_exec_id":     e.TaskExecID.String(),
		"exec_id":          e.ExecID.String(),
	}
	if e.TriggerInput != nil {
		if err := common.AssignProto(execMap, "trigger_input", e.TriggerInput); err != nil {
			return nil, err
		}
	}
	if e.TaskInput != nil {
		if err := common.AssignProto(execMap, "task_input", e.TaskInput); err != nil {
			return nil, err
		}
	}
	if e.ToolInput != nil {
		if err := common.AssignProto(execMap, "tool_input", e.ToolInput); err != nil {
			return nil, err
		}
	}
	if e.TaskEnv != nil {
		if err := common.AssignProto(execMap, "task_env", &e.TaskEnv); err != nil {
			return nil, err
		}
	}
	if e.ToolEnv != nil {
		if err := common.AssignProto(execMap, "tool_env", &e.ToolEnv); err != nil {
			return nil, err
		}
	}
	return execMap, nil
}
