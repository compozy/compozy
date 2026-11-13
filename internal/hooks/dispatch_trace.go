package hooks

import "context"

// DispatchTracer traces one hook dispatch that matched hooks: it returns the context the dispatch runs under and a
// function that ends the trace with whether the dispatch was denied and its error. The daemon supplies it, since the
// telemetry package cannot be imported here.
type DispatchTracer func(ctx context.Context, event HookEvent) (context.Context, func(denied bool, err error))

// WithDispatchTracer injects the hook.dispatch span tracer.
func WithDispatchTracer(tracer DispatchTracer) Option {
	return func(hooks *Hooks) {
		hooks.dispatchTracer = tracer
	}
}

func (h *Hooks) startDispatchTrace(ctx context.Context, event HookEvent) (context.Context, func(bool, error)) {
	if h.dispatchTracer == nil {
		return ctx, func(bool, error) {}
	}
	return h.dispatchTracer(ctx, event)
}
