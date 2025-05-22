package agent

import (
	"github.com/compozy/compozy/engine/common"
)

// -----------------------------------------------------------------------------
// Execution
// -----------------------------------------------------------------------------

type Execution struct {
	CorrID         common.ID
	WorkflowExecID common.ID
	TaskExecID     common.ID
	AgentExecID    common.ID
	TaskEnv        common.EnvMap
	AgentEnv       common.EnvMap
	TriggerInput   *common.Input
	TaskInput      *common.Input
	AgentInput     *common.Input
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
