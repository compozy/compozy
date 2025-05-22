package workflow

import (
	"github.com/compozy/compozy/engine/common"
)

// -----------------------------------------------------------------------------
// Execution
// -----------------------------------------------------------------------------

type Execution struct {
	CorrID         common.ID     `json:"correlation_id"`
	WorkflowExecID common.ID     `json:"workflow_execution_id"`
	TriggerInput   *common.Input `json:"trigger_input"`
	ProjectEnv     common.EnvMap `json:"project_env"`
	WorkflowEnv    common.EnvMap `json:"workflow_env"`
}

func NewExecution(tgInput *common.Input, pjEnv common.EnvMap) (*Execution, error) {
	corrID, err := common.NewID()
	if err != nil {
		return nil, err
	}
	execID, err := common.NewID()
	if err != nil {
		return nil, err
	}
	return &Execution{
		CorrID:         corrID,
		WorkflowExecID: execID,
		TriggerInput:   tgInput,
		ProjectEnv:     pjEnv,
		WorkflowEnv:    make(common.EnvMap),
	}, nil
}
