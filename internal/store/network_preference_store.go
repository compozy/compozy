package store

import "context"

// NetworkPreferenceStore manages network delivery preferences and network-task links.
type NetworkPreferenceStore interface {
	PutNetworkSubscription(ctx context.Context, entry NetworkSubscriptionEntry) error
	PutNetworkSubscriptionWithChannel(
		ctx context.Context,
		channel NetworkChannelEntry,
		entry NetworkSubscriptionEntry,
	) error
	ListNetworkSubscriptions(ctx context.Context, query NetworkSubscriptionQuery) ([]NetworkSubscriptionEntry, error)
	DeleteNetworkSubscription(ctx context.Context, ref NetworkSubscriptionRef) error
	PutNetworkTaskThreadOrigin(ctx context.Context, origin NetworkTaskThreadOrigin) error
	ListNetworkTaskThreadOrigins(
		ctx context.Context,
		query NetworkTaskThreadOriginQuery,
	) ([]NetworkTaskThreadOrigin, error)
	PutTaskDesignationRollup(ctx context.Context, rollup TaskDesignationRollup) error
	ListTaskDesignationRollups(ctx context.Context, query TaskDesignationRollupQuery) ([]TaskDesignationRollup, error)
}
