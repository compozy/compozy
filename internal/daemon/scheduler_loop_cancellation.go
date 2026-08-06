package daemon

import (
	"context"

	taskpkg "github.com/compozy/compozy/internal/task"
)

func (s schedulerTaskSource) ReconcilePendingCancellations(
	ctx context.Context,
	limit int,
	actor taskpkg.ActorContext,
) (int, error) {
	if s.cancellationReconciler == nil {
		return 0, nil
	}
	return s.cancellationReconciler.ReconcilePendingCancellations(ctx, limit, actor)
}
