package testutil

import (
	"context"

	"github.com/compozy/compozy/internal/store"
)

func (s StubNetworkStore) ListNetworkChannels(
	ctx context.Context,
	query store.NetworkChannelQuery,
) ([]store.NetworkChannelEntry, error) {
	if s.ListNetworkChannelsFn != nil {
		return s.ListNetworkChannelsFn(ctx, query)
	}
	if !query.ReadScope.Matches(store.DefaultProfileID) {
		return nil, nil
	}
	projections, err := s.ListNetworkChannelProjections(ctx, store.NetworkChannelProjectionQuery{
		WorkspaceID: query.WorkspaceID,
	})
	if err != nil {
		return nil, err
	}
	entries := make([]store.NetworkChannelEntry, 0, len(projections))
	for _, projection := range projections {
		entries = append(entries, store.NetworkChannelEntry{
			ProfileID:   store.DefaultProfileID,
			WorkspaceID: projection.WorkspaceID,
			Channel:     projection.Channel,
		})
	}
	return entries, nil
}
