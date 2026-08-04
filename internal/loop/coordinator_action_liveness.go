package loop

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/task"
)

// ActionRunSilenceWindow returns the pinned silence-attention policy for one action cell.
func (r *CoordinatorRunner) ActionRunSilenceWindow(ctx context.Context, taskRun task.Run) (time.Duration, error) {
	actionCtx, err := r.resolveActionRun(ctx, taskRun)
	if err != nil {
		return 0, err
	}
	resolved, err := ResolveNodeLifecycleConfig(actionCtx.node, nil, actionCtx.effective.Lifecycle)
	if err != nil {
		return 0, err
	}
	return resolved.LivenessSilenceWindow, nil
}

// ActionRunDeathStreakLimit returns the pinned confirmed-death continuation cap for one action cell.
func (r *CoordinatorRunner) ActionRunDeathStreakLimit(ctx context.Context, taskRun task.Run) (int, error) {
	actionCtx, err := r.resolveActionRun(ctx, taskRun)
	if err != nil {
		return 0, err
	}
	resolved, err := ResolveNodeLifecycleConfig(actionCtx.node, nil, actionCtx.effective.Lifecycle)
	if err != nil {
		return 0, err
	}
	return resolved.ResumeDeathStreakLimit, nil
}
