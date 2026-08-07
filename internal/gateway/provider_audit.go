package gateway

import "context"

// ProviderRefusalKind identifies which provider boundary refused the attempt.
type ProviderRefusalKind string

const (
	ProviderRefusalAuthority ProviderRefusalKind = "authority"
	ProviderRefusalEndpoint  ProviderRefusalKind = "endpoint"
)

// ProviderAuditSink durably records provider verification decisions at the owning transition.
type ProviderAuditSink interface {
	RecordProviderRefusal(context.Context, ProviderActivation, ProviderRefusalKind, error) error
	RecordEndpointVerified(context.Context, ProviderActivation, AdvertisedEndpoint) error
	RecordProviderStateChanged(context.Context, ProviderActivation, ProviderObservedState, string) error
}

// ProviderStateReporter records reconciled provider lifecycle changes.
type ProviderStateReporter interface {
	RecordProviderStateChanged(context.Context, ProviderActivation, ProviderObservedState, string) error
}

type noopProviderAuditSink struct{}

func (noopProviderAuditSink) RecordProviderRefusal(
	context.Context,
	ProviderActivation,
	ProviderRefusalKind,
	error,
) error {
	return nil
}

func (noopProviderAuditSink) RecordEndpointVerified(
	context.Context,
	ProviderActivation,
	AdvertisedEndpoint,
) error {
	return nil
}

func (noopProviderAuditSink) RecordProviderStateChanged(
	context.Context,
	ProviderActivation,
	ProviderObservedState,
	string,
) error {
	return nil
}
