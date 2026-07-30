package daemon

import (
	"context"
	"log/slog"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type loopCoordinatorRunActivator struct {
	state *bootState
}

var _ looppkg.CoordinatorRunActivator = loopCoordinatorRunActivator{}

func (a loopCoordinatorRunActivator) ActivateCoordinatorRun(ctx context.Context, run taskpkg.Run) {
	if a.state == nil || a.state.tasks == nil || a.state.tasks.coordinatorBackstop == nil {
		a.logFailure(run, "coordinator backstop is unavailable", nil)
		return
	}
	actor, err := taskpkg.DeriveDaemonActorContext("loop-control-activation", "loop.control")
	if err != nil {
		a.logFailure(run, "derive coordinator activation actor", err)
		return
	}
	if _, err := a.state.tasks.coordinatorBackstop.RunLoopCoordinatorBackstop(
		ctx,
		time.Now().UTC(),
		actor,
	); err != nil {
		a.logFailure(run, "start coordinator backstop", err)
	}
}

func (a loopCoordinatorRunActivator) logFailure(run taskpkg.Run, reason string, err error) {
	logger := slog.Default()
	if a.state != nil && a.state.logger != nil {
		logger = a.state.logger
	}
	args := []any{
		"reason", strings.TrimSpace(reason),
		daemonLogRunIDKey, strings.TrimSpace(run.ID),
	}
	if err != nil {
		args = append(args, "error", err)
	}
	logger.Error("daemon: committed Loop coordinator wake could not start", args...)
}
