package tool

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
	ExecID         common.ID
	TaskEnv        common.EnvMap
	ToolEnv        common.EnvMap
	TriggerInput   *common.Input
	TaskInput      *common.Input
	ToolInput      *common.Input
}

func NewExecution(
	corrID common.ID,
	taskExecID, workflowExecID common.ID,
	taskEnv, toolEnv common.EnvMap,
	tgInput, taskInput, toolInput *common.Input,
) (*Execution, error) {
	execID, err := common.NewID()
	if err != nil {
		return nil, err
	}
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
	}, nil
}
