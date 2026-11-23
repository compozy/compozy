package gateway

import (
	"context"
	"time"
)

// Store owns the atomic desired-state transitions used by Policy.
type Store interface {
	Snapshot(context.Context) (Snapshot, error)
	Transition(context.Context, TransitionRequest, time.Time) (bool, error)
	DisableAll(context.Context) (bool, error)
	SetObserved(
		context.Context,
		TierPlan,
		ProviderObservedState,
		SurfaceObservedState,
		time.Time,
		string,
	) (bool, error)
}
