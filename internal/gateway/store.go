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

// IngressStore owns public endpoint identity and workspace-aware subject bindings.
type IngressStore interface {
	ResolveIngressSubject(context.Context, IngressSubjectRef) (IngressSubject, error)
	GetIngressEndpoint(context.Context, Tier) (IngressEndpointState, error)
	SyncIngressEndpoint(context.Context, Tier, string, time.Time) (IngressEndpointState, bool, error)
	MarkIngressEndpointUnavailable(context.Context, Tier, time.Time) error
	GetIngressBinding(context.Context, IngressSubjectRef) (IngressBinding, error)
	PutIngressBinding(
		context.Context,
		IngressSubject,
		uint64,
		time.Time,
	) (IngressBinding, bool, error)
	DeleteIngressBinding(context.Context, IngressSubject) (bool, error)
	ListIngressBindings(context.Context) ([]IngressBinding, error)
	SweepOrphanedIngressBindings(context.Context) (int64, error)
}
