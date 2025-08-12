package core

import (
	"context"
)

type ctxKeyExpectedOutputs struct{}

// WithExpectedOutputs adds expected outputs to context for testing
func WithExpectedOutputs(parent context.Context, outputs map[string]Output) context.Context {
	return context.WithValue(parent, ctxKeyExpectedOutputs{}, outputs)
}

// ExpectedOutputsFromContext retrieves expected outputs from context
func ExpectedOutputsFromContext(ctx context.Context) map[string]Output {
	if v, ok := ctx.Value(ctxKeyExpectedOutputs{}).(map[string]Output); ok {
		return v
	}
	return nil
}
