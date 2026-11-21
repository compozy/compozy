package tools

import "context"

type hostedWaitProgressKey struct{}

// WithHostedWaitProgress attaches the transport's progress reporter to a tool call.
func WithHostedWaitProgress(ctx context.Context, start func() func()) context.Context {
	return context.WithValue(ctx, hostedWaitProgressKey{}, start)
}

// BeginHostedWaitProgress marks an intentionally unbounded phase until the returned function is called.
func BeginHostedWaitProgress(ctx context.Context) func() {
	if start, ok := ctx.Value(hostedWaitProgressKey{}).(func() func()); ok {
		return start()
	}
	return func() {}
}
