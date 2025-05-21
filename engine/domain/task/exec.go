package task

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
	WorkflowEnv    common.EnvMap
	TaskEnv        common.EnvMap
	TriggerInput   *common.Input
	TaskInput      *common.Input
}

func NewExecution(
	corrID common.CorrID,
	workflowExecID common.ExecID,
	workflowEnv, taskEnv common.EnvMap,
	tgInput, taskInput *common.Input,
) *Execution {
	execID := common.NewExecID()
	return &Execution{
		CorrID:         corrID,
		WorkflowExecID: workflowExecID,
		TaskExecID:     execID,
		WorkflowEnv:    workflowEnv,
		TaskEnv:        taskEnv,
		TriggerInput:   tgInput,
		TaskInput:      taskInput,
	}
}

func (e *Execution) ToProtoBufMap() (map[string]any, error) {
	execMap := map[string]any{
		"corr_id":          e.CorrID.String(),
		"workflow_exec_id": e.WorkflowExecID.String(),
		"task_exec_id":     e.TaskExecID.String(),
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
	if e.WorkflowEnv != nil {
		if err := common.AssignProto(execMap, "workflow_env", &e.WorkflowEnv); err != nil {
			return nil, err
		}
	}
	if e.TaskEnv != nil {
		if err := common.AssignProto(execMap, "task_env", &e.TaskEnv); err != nil {
			return nil, err
		}
	}
	return execMap, nil
}
