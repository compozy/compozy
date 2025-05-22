package tool

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
	ToolExecID     common.ID     `json:"tool_execution_id"`
	TaskEnv        common.EnvMap `json:"task_env"`
	ToolEnv        common.EnvMap `json:"tool_env"`
	TriggerInput   *common.Input `json:"trigger_input"`
	TaskInput      *common.Input `json:"task_input"`
	ToolInput      *common.Input `json:"tool_input"`
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
		ToolExecID:     execID,
		TaskEnv:        taskEnv,
		ToolEnv:        toolEnv,
		TriggerInput:   tgInput,
		TaskInput:      taskInput,
		ToolInput:      toolInput,
	}, nil
}
