package agent

import (
	"github.com/compozy/compozy/engine/common"
)

// -----------------------------------------------------------------------------
// Execution
// -----------------------------------------------------------------------------

type Execution struct {
	CorrID         common.ID     `json:"correlation_id"`
	WorkflowExecID common.ID     `json:"workflow_execution_id"`
	TaskExecID     common.ID     `json:"task_execution_id"`
	AgentExecID    common.ID     `json:"agent_execution_id"`
	TaskEnv        common.EnvMap `json:"task_env"`
	AgentEnv       common.EnvMap `json:"agent_env"`
	TriggerInput   *common.Input `json:"trigger_input"`
	TaskInput      *common.Input `json:"task_input"`
	AgentInput     *common.Input `json:"agent_input"`
}

func NewExecution(
	corrID common.ID,
	taskExecID, workflowExecID common.ID,
	taskEnv, agentEnv common.EnvMap,
	tgInput, taskInput, agentInput *common.Input,
) (*Execution, error) {
	execID, err := common.NewID()
	if err != nil {
		return nil, err
	}
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
	}, nil
}
