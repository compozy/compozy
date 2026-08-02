package loop

import (
	"context"
	"time"
)

const loopPostCommitTimeout = 30 * time.Second

type loopPostCommitContextKey struct{}

func loopPostCommitContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx.Value(loopPostCommitContextKey{}) != nil {
		return context.WithCancel(ctx)
	}
	bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), loopPostCommitTimeout)
	return context.WithValue(bounded, loopPostCommitContextKey{}, struct{}{}), cancel
}
