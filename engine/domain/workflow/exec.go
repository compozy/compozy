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
