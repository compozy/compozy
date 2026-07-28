package store

import "context"

// NetworkChannelStore manages durable network channel metadata.
type NetworkChannelStore interface {
	CreateNetworkChannel(ctx context.Context, entry NetworkChannelEntry) error
	WriteNetworkChannel(ctx context.Context, entry NetworkChannelEntry) error
	GetNetworkChannel(ctx context.Context, ref NetworkChannelRef) (NetworkChannelEntry, error)
	ListNetworkChannels(ctx context.Context, query NetworkChannelQuery) ([]NetworkChannelEntry, error)
	PatchNetworkChannel(ctx context.Context, ref NetworkChannelRef, patch NetworkChannelPatch) error
	DeleteNetworkChannel(ctx context.Context, ref NetworkChannelRef) error
}

// NetworkChannelProjectionStore reads materialized channel timeline totals.
type NetworkChannelProjectionStore interface {
	ListNetworkChannelProjections(
		ctx context.Context,
		query NetworkChannelProjectionQuery,
	) ([]NetworkChannelProjection, error)
	ListNetworkChannelKindCounts(
		ctx context.Context,
		ref NetworkChannelRef,
	) ([]NetworkChannelKindCount, error)
}

// NetworkRecentStore reads cross-channel recent conversation summaries.
type NetworkRecentStore interface {
	ListNetworkRecents(ctx context.Context, query NetworkRecentQuery) ([]NetworkRecentSummary, error)
}
