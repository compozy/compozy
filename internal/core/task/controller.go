package task

import (
	"context"

	"github.com/compozy/compozy/internal/core"
	config "github.com/compozy/compozy/internal/parser/task"
)

type TaskController struct {
	ExecID string
	state  *TaskState
	Ctx    context.Context
}

func InitTaskController(ctx context.Context, execID string, cfg *config.TaskConfig, wsState core.State) (*TaskController, error) {
	stID, err := core.NewStateID(cfg, execID)
	if err != nil {
		return nil, err
	}

	state, err := InitTaskState(stID, cfg, wsState)
	if err != nil {
		return nil, core.NewError(stID, "exec_state_fail", "failed to create exec state", err)
	}
	return &TaskController{ExecID: execID, state: state, Ctx: ctx}, nil
}

func (c *TaskController) Run() error {
	select {
	case <-c.Ctx.Done():
		return core.NewError(c.state.id, "cancelled", "workflow cancelledl", c.Ctx.Err())
	default:
		return nil
	}
}
