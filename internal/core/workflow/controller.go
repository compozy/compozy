package workflow

import (
	"context"

	"github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/parser/common"
	config "github.com/compozy/compozy/internal/parser/workflow"
)

type Controller struct {
	ExecID string
	state  *State
	store  *core.Store
	Ctx    context.Context
}

func InitWorkflowController(
	_ context.Context,
	execID string,
	cfg *config.Config,
	input common.Input,
) (*Controller, error) {
	// Init Store and create StateID
	store := core.NewStore(execID)
	stID, err := core.NewStateID(cfg, execID)
	if err != nil {
		return nil, err
	}

	// Validate configuration params
	if err := cfg.ValidateParams(input); err != nil {
		errResponse := core.NewError(stID, "invalid_params", "invalid input params", err)
		return nil, &errResponse
	}

	// Init WorkflowState and add it to the Store
	state, err := InitWorkflowState(stID, input, cfg)
	if err != nil {
		errResponse := core.NewError(stID, "exec_state_fail", "failed to create exec state", err)
		return nil, &errResponse
	}
	store.SetWorkflow(state)

	return &Controller{ExecID: execID, state: state, store: store}, nil
}

func (c *Controller) Run() error {
	select {
	case <-c.Ctx.Done():
		errResponse := core.NewError(c.state.id, "canceled", "workflow cancelledl", c.Ctx.Err())
		return &errResponse
	default:
		return nil
	}
}
