package ingress

import (
	"context"
	"time"
)

// Store owns public endpoint identity and workspace-aware subject bindings.
type Store interface {
	ResolveIngressSubject(context.Context, SubjectRef) (Subject, error)
	GetIngressEndpoint(context.Context) (EndpointState, error)
	SyncIngressEndpoint(context.Context, string, time.Time) (EndpointState, bool, error)
	MarkIngressEndpointUnavailable(context.Context, time.Time) error
	GetIngressBinding(context.Context, SubjectRef) (Binding, error)
	PutIngressBinding(
		context.Context,
		Subject,
		uint64,
		time.Time,
	) (Binding, bool, error)
	DeleteIngressBinding(context.Context, Subject) (bool, error)
	ListIngressBindings(context.Context) ([]Binding, error)
	SweepOrphanedIngressBindings(context.Context) (int64, error)
}
