package workflow

import (
	"github.com/compozy/compozy/engine/common"
)

// -----------------------------------------------------------------------------
// Execution
// -----------------------------------------------------------------------------

type Execution struct {
	CorrID         common.ID
	WorkflowExecID common.ID
	TriggerInput   *common.Input
	ProjectEnv     common.EnvMap
	WorkflowEnv    common.EnvMap
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
