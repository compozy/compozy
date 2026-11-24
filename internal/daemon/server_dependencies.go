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
	state.deps.TelemetryStatus = state.ensureTelemetry().Status
	if installer, ok := state.sessions.(goalCommandHandlerInstaller); ok {
		if handler, handlerOK := loopAPI.(session.GoalCommandHandler); handlerOK {
			installer.SetGoalCommandHandler(handler)
		}
	}
	if state.remoteOperator != nil {
		// Rebuild the remote operator router once late-booted dependencies
		// (worktrees, automation) have been wired into state.deps.
		state.remoteOperator.swap(remoteOperatorRoutes(&state.deps))
	}
	return ctx.Err()
}
