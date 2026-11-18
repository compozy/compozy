package deadentity

import (
	"context"

	"github.com/compozy/compozy/internal/store"
)

type probeRetryKey struct{}

// WithProbeRetry admits an explicit retry for one entity without claiming recovery before discovery succeeds.
func WithProbeRetry(ctx context.Context, key store.DeadEntityKey) context.Context {
	return context.WithValue(ctx, probeRetryKey{}, key.Normalize())
}

func explicitProbeRetry(ctx context.Context, key store.DeadEntityKey) bool {
	requested, ok := ctx.Value(probeRetryKey{}).(store.DeadEntityKey)
	return ok && requested == key
}
