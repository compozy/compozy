package workflow

import (
	"context"

	"github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/parser/common"
	config "github.com/compozy/compozy/internal/parser/workflow"
)

type WorkflowController struct {
	ExecID string
	state  *WorkflowState
	store  *core.Store
	Ctx    context.Context
}

func InitWorkflowController(ctx context.Context, execID string, cfg *config.WorkflowConfig, input common.Input) (*WorkflowController, error) {
	// Validate WorkflowConfig
	if err := cfg.Validate(); err != nil {
		return nil, core.NewError(nil, "invalid_config", "invalid workflow config", err)
	}

	// Init Store and create StateID
	store := core.NewStore(execID)
	stID, err := core.NewStateID(cfg, execID)
	if err != nil {
		return nil, err
	}

	// Validate configuration params
	if err := cfg.ValidateParams(input); err != nil {
		return nil, core.NewError(stID, "invalid_params", "invalid input params", err)
	}

	// Init WorkflowState and add it to the Store
	state, err := InitWorkflowState(stID, input, cfg)
	if err != nil {
		return nil, core.NewError(stID, "exec_state_fail", "failed to create exec state", err)
	}
	store.SetWorkflow(state)

	return &WorkflowController{ExecID: execID, state: state, store: store}, nil
}

func (c *WorkflowController) Run() error {
	select {
	case <-c.Ctx.Done():
		return core.NewError(c.state.id, "cancelled", "workflow cancelledl", c.Ctx.Err())
	default:
		return nil
	}
}
