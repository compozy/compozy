package task

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
	WorkflowEnv    common.EnvMap `json:"workflow_env"`
	TaskEnv        common.EnvMap `json:"task_env"`
	TriggerInput   *common.Input `json:"trigger_input"`
	TaskInput      *common.Input `json:"task_input"`
}

func NewExecution(
	corrID common.ID,
	workflowExecID common.ID,
	workflowEnv, taskEnv common.EnvMap,
	tgInput, taskInput *common.Input,
) (*Execution, error) {
	execID, err := common.NewID()
	if err != nil {
		return nil, err
	}
	return &Execution{
		CorrID:         corrID,
		WorkflowExecID: workflowExecID,
		TaskExecID:     execID,
		WorkflowEnv:    workflowEnv,
		TaskEnv:        taskEnv,
		TriggerInput:   tgInput,
		TaskInput:      taskInput,
	}, nil
}
