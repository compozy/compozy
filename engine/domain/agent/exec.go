package agent

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
	AgentExecID    common.ExecID
	TaskEnv        common.EnvMap
	AgentEnv       common.EnvMap
	TriggerInput   *common.Input
	TaskInput      *common.Input
	AgentInput     *common.Input
}

func NewExecution(
	corrID common.CorrID,
	taskExecID, workflowExecID common.ExecID,
	taskEnv, agentEnv common.EnvMap,
	tgInput, taskInput, agentInput *common.Input,
) *Execution {
	execID := common.NewExecID()
	return &Execution{
		CorrID:         corrID,
		WorkflowExecID: workflowExecID,
		TaskExecID:     taskExecID,
		AgentExecID:    execID,
		TaskEnv:        taskEnv,
		AgentEnv:       agentEnv,
		TriggerInput:   tgInput,
		TaskInput:      taskInput,
		AgentInput:     agentInput,
	}
}

func (e *Execution) ToProtoBufMap() (map[string]any, error) {
	execMap := map[string]any{
		"corr_id":          e.CorrID.String(),
		"workflow_exec_id": e.WorkflowExecID.String(),
		"task_exec_id":     e.TaskExecID.String(),
		"agent_exec_id":    e.AgentExecID.String(),
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
	if e.AgentInput != nil {
		if err := common.AssignProto(execMap, "agent_input", e.AgentInput); err != nil {
			return nil, err
		}
	}
	if e.TaskEnv != nil {
		if err := common.AssignProto(execMap, "task_env", &e.TaskEnv); err != nil {
			return nil, err
		}
	}
	if e.AgentEnv != nil {
		if err := common.AssignProto(execMap, "agent_env", &e.AgentEnv); err != nil {
			return nil, err
		}
	}
	return execMap, nil
}
