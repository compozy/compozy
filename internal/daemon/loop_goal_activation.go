package daemon

import (
	"context"
	"log/slog"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	taskpkg "github.com/compozy/agh/internal/task"
)

type loopGoalRunActivator struct {
	state *bootState
}

var _ looppkg.GoalRunActivator = loopGoalRunActivator{}

func (a loopGoalRunActivator) ActivateGoalRun(ctx context.Context, run taskpkg.Run) {
	if a.state == nil || a.state.tasks == nil {
		a.logUnavailable(run, "task runtime is unavailable")
		return
	}
	dispatcher := a.state.tasks.activation.Load()
	if dispatcher == nil {
		a.logUnavailable(run, "activation dispatcher is unavailable")
		return
	}
	dispatcher.OnTaskRunEnqueued(ctx, activationDispatchPayload(run))
}

func (a loopGoalRunActivator) logUnavailable(run taskpkg.Run, reason string) {
	logger := slog.Default()
	if a.state != nil && a.state.logger != nil {
		logger = a.state.logger
	}
	logger.Error(
		"daemon: committed Goal successor could not enter activation dispatcher",
		"reason", strings.TrimSpace(reason),
		daemonLogRunIDKey, strings.TrimSpace(run.ID),
	)
}
