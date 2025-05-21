package workflow

import (
	"github.com/compozy/compozy/engine/common"
)

// -----------------------------------------------------------------------------
// Execution
// -----------------------------------------------------------------------------

type Execution struct {
	CorrID         common.CorrID
	WorkflowExecID common.ExecID
	TriggerInput   *common.Input
	ProjectEnv     common.EnvMap
	WorkflowEnv    common.EnvMap
}

func NewExecution(tgInput *common.Input, pjEnv common.EnvMap) *Execution {
	corrID := common.NewCorrID()
	execID := common.NewExecID()
	return &Execution{
		CorrID:         corrID,
		WorkflowExecID: execID,
		TriggerInput:   tgInput,
		ProjectEnv:     pjEnv,
		WorkflowEnv:    make(common.EnvMap),
	}
}

func (e *Execution) ToProtoBufMap() (map[string]any, error) {
	execMap := map[string]any{
		"corr_id":          e.CorrID.String(),
		"workflow_exec_id": e.WorkflowExecID.String(),
	}
	if e.TriggerInput != nil {
		if err := common.AssignProto(execMap, "trigger_input", e.TriggerInput); err != nil {
			return nil, err
		}
	}
	if e.ProjectEnv != nil {
		if err := common.AssignProto(execMap, "project_env", &e.ProjectEnv); err != nil {
			return nil, err
		}
	}
	if e.WorkflowEnv != nil {
		if err := common.AssignProto(execMap, "workflow_env", &e.WorkflowEnv); err != nil {
			return nil, err
		}
	}
	return execMap, nil
}
