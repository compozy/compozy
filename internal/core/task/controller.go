package task

import (
	"github.com/compozy/compozy/internal/core"
	config "github.com/compozy/compozy/internal/parser/task"
)

type TaskController struct {
	ExecID string
	state  *TaskState
}

func InitTaskController(execID string, cfg *config.TaskConfig, wsState core.State) (*TaskController, error) {
	state, err := InitTaskState(execID, cfg, wsState)
	if err != nil {
		return nil, core.NewTaskError(execID, "exec_state_fail", "failed to create exec state", err)
	}
	return &TaskController{ExecID: execID, state: state}, nil
}

func (c *TaskController) Run() error {
	return nil
}
