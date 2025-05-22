package task

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
	WorkflowEnv    common.EnvMap
	TaskEnv        common.EnvMap
	TriggerInput   *common.Input
	TaskInput      *common.Input
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
