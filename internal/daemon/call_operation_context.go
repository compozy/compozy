package daemon

import (
	"context"
	"fmt"
)

func (n *daemonNativeTools) callOperationContext(
	ctx context.Context,
) (context.Context, context.CancelFunc, error) {
	if n == nil || n.deps == nil {
		return nil, nil, fmt.Errorf("daemon: native call tools are required")
	}
	timeout, err := n.deps.Config.Calls.OperationTimeoutDuration()
	if err != nil {
		return nil, nil, fmt.Errorf("daemon: resolve call operation timeout: %w", err)
	}
	operationCtx, cancel := detachedDaemonOperationContext(ctx, timeout)
	return operationCtx, cancel, nil
}
