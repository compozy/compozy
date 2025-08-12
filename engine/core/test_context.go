package core

import (
	"context"
)

type ctxKeyExpectedOutputs struct{}

// WithExpectedOutputs returns a new context derived from parent that carries the provided expected outputs.
// The map is stored under an unexported package key for use in tests; retrieve it with ExpectedOutputsFromContext.
func WithExpectedOutputs(parent context.Context, outputs map[string]Output) context.Context {
	return context.WithValue(parent, ctxKeyExpectedOutputs{}, outputs)
}

// ExpectedOutputsFromContext returns the expected outputs map previously stored in ctx by WithExpectedOutputs.
// If no such value is present or the stored value has a different type, it returns nil.
func ExpectedOutputsFromContext(ctx context.Context) map[string]Output {
	if v, ok := ctx.Value(ctxKeyExpectedOutputs{}).(map[string]Output); ok {
		return v
	}
	return nil
}
