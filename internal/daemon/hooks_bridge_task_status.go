package daemon

import (
	"context"

	hookspkg "github.com/compozy/compozy/internal/hooks"
)

func (n *hooksNotifier) DispatchTaskStatusChanged(
	ctx context.Context,
	payload hookspkg.TaskStatusChangedPayload,
) (hookspkg.TaskStatusChangedPayload, error) {
	return dispatchTaskStatusChangedWithWatchObservers(ctx, n, payload)
}
