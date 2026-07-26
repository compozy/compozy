package session

import "context"

func (m *Manager) promptExecutionContext(
	ctx context.Context,
	preserveCallerCancellation bool,
) (context.Context, context.CancelFunc) {
	base := ctx
	if base != nil && !preserveCallerCancellation {
		base = context.WithoutCancel(base)
	}
	if base == nil {
		base = m.fallbackLifecycleContext()
	}
	executionCtx, cancel := context.WithCancel(base)
	lifecycleStop := context.AfterFunc(m.fallbackLifecycleContext(), cancel)
	return executionCtx, func() {
		lifecycleStop()
		cancel()
	}
}
