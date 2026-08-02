package task

import (
	"context"
	"time"
)

const taskPostCommitTimeout = 30 * time.Second

type taskPostCommitContextKey struct{}

func taskPostCommitContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if isCompletedSettlementPublicationContext(ctx) || ctx.Value(taskPostCommitContextKey{}) != nil {
		return context.WithCancel(ctx)
	}
	bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), taskPostCommitTimeout)
	return context.WithValue(bounded, taskPostCommitContextKey{}, struct{}{}), cancel
}

func taskRunObservationHookContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return taskPostCommitContext(ctx)
}
