package admission

import "context"

type backgroundWorkContextKey struct{}

// WithBackgroundWork marks ctx as daemon-initiated background work (automation,
// Loop, scheduler wakes). Background work yields to host CPU pressure at
// admission; unmarked work is treated as user-initiated and never waits.
func WithBackgroundWork(ctx context.Context) context.Context {
	return context.WithValue(ctx, backgroundWorkContextKey{}, true)
}

// IsBackgroundWork reports whether ctx carries the background-work marker.
func IsBackgroundWork(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	marked, ok := ctx.Value(backgroundWorkContextKey{}).(bool)
	return ok && marked
}
