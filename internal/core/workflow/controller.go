package workflow

import (
	"github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/parser/common"
	config "github.com/compozy/compozy/internal/parser/workflow"
)

type WorkflowController struct {
	ExecID string
	state  *WorkflowState
	store  *core.Store
}

func NewWorkflowController(execID string, cfg *config.WorkflowConfig, input common.Input) (*WorkflowController, error) {
	state, err := InitWorkflowState(execID, input, cfg)
	if err != nil {
		return nil, core.NewWorkflowError(execID, "exec_state_fail", "failed to create exec state", err)
	}
	store := core.NewStore(execID)
	store.SetWorkflow(state)
	return &WorkflowController{ExecID: execID, state: state, store: store}, nil
}

func (c *WorkflowController) Run() error {
	return nil
}
