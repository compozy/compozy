package consolidation

import (
	"context"

	"github.com/compozy/compozy/internal/memory"
)

// CompletionObserver receives successful consolidation results after their
// memory transaction and lock release have completed.
type CompletionObserver func(context.Context, memory.ConsolidationResult) error

// SetCompletionObserver attaches the runtime consumer for successful consolidations.
func (r *Runtime) SetCompletionObserver(observer CompletionObserver) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.completionObserver = observer
	r.mu.Unlock()
}

func (r *Runtime) runConsolidation(
	ctx context.Context,
	service Service,
	spawner memory.SessionSpawner,
	workspaceRef string,
) error {
	result, err := service.Run(ctx, spawner, workspaceRef)
	if err != nil {
		return err
	}
	r.notifyCompletion(ctx, result)
	return nil
}

func (r *Runtime) notifyCompletion(ctx context.Context, result memory.ConsolidationResult) {
	r.mu.Lock()
	observer := r.completionObserver
	logger := r.logger
	r.mu.Unlock()
	if observer == nil {
		return
	}

	notifyCtx := context.Background()
	if ctx != nil {
		notifyCtx = context.WithoutCancel(ctx)
	}
	if err := observer(notifyCtx, result); err != nil {
		logger.Warn(
			"daemon: publish memory consolidation completion failed",
			"workspace_id",
			result.WorkspaceID,
			"completed_at",
			result.CompletedAt,
			"error",
			err,
		)
	}
}
