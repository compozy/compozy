package daemon

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/session"
)

func (d *Daemon) prepareServerDependencies(ctx context.Context, state *bootState) error {
	if state == nil {
		return errors.New("daemon: server dependency state is required")
	}
	loopAPI, err := newDaemonLoopAPIService(state, d.homePaths, d.now)
	if err != nil {
		return err
	}
	state.deps.Loops = loopAPI
	if installer, ok := state.sessions.(goalCommandHandlerInstaller); ok {
		if handler, handlerOK := loopAPI.(session.GoalCommandHandler); handlerOK {
			installer.SetGoalCommandHandler(handler)
		}
	}
	return ctx.Err()
}
