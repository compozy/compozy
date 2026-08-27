package core

import (
	"context"
	"fmt"
)

func (h *BaseHandlers) callsOperationContext(
	ctx context.Context,
) (context.Context, context.CancelFunc, error) {
	timeout, err := h.Config.Calls.OperationTimeoutDuration()
	if err != nil {
		return nil, nil, fmt.Errorf("api: resolve call operation timeout: %w", err)
	}
	operationCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	return operationCtx, cancel, nil
}
