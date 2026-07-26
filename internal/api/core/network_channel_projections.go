package core

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/store"
)

func listNetworkChannelProjections(
	ctx context.Context,
	networkStore NetworkStore,
	workspaceID string,
	channel string,
) ([]store.NetworkChannelProjection, error) {
	projectionStore, ok := networkStore.(store.NetworkChannelProjectionStore)
	if !ok {
		return nil, errors.New("api: network channel projection store is required")
	}
	return projectionStore.ListNetworkChannelProjections(ctx, store.NetworkChannelProjectionQuery{
		WorkspaceID: strings.TrimSpace(workspaceID),
		Channel:     strings.TrimSpace(channel),
	})
}

func applyNetworkChannelProjections(
	aggregates map[string]*networkChannelAggregate,
	projections []store.NetworkChannelProjection,
) {
	for _, projection := range projections {
		aggregate := ensureNetworkChannelAggregate(
			aggregates,
			projection.WorkspaceID,
			projection.Channel,
		)
		aggregate.messageCount = projection.MessageCount
		aggregate.presenceCount = projection.PresenceCount
		aggregate.historicalParticipantCount = projection.HistoricalParticipantCount
		aggregate.lastActivityAt = cloneTimePtr(projection.LastActivityAt)
		aggregate.lastActivitySequence = projection.LastActivitySequence
		aggregate.lastPresenceAt = cloneTimePtr(projection.LastPresenceAt)
		aggregate.lastPresenceSequence = projection.LastPresenceSequence
		aggregate.lastMessagePreview = strings.TrimSpace(projection.LastMessagePreview)
	}
}

func firstNetworkChannelProjection(
	projections []store.NetworkChannelProjection,
) store.NetworkChannelProjection {
	if len(projections) == 0 {
		return store.NetworkChannelProjection{}
	}
	return projections[0]
}

func networkChannelHasPersistedProjection(
	ctx context.Context,
	networkStore NetworkStore,
	workspaceID string,
	channel string,
) (bool, error) {
	projections, err := listNetworkChannelProjections(ctx, networkStore, workspaceID, channel)
	if err != nil {
		return false, err
	}
	return len(projections) > 0, nil
}

func listNetworkChannelKindCountPayloads(
	ctx context.Context,
	networkStore NetworkStore,
	ref store.NetworkChannelRef,
) ([]contract.NetworkChannelKindCountPayload, error) {
	projectionStore, ok := networkStore.(store.NetworkChannelProjectionStore)
	if !ok {
		return nil, errors.New("api: network channel projection store is required")
	}
	counts, err := projectionStore.ListNetworkChannelKindCounts(ctx, ref)
	if err != nil {
		return nil, err
	}
	payloads := make([]contract.NetworkChannelKindCountPayload, 0, len(counts))
	for _, count := range counts {
		payloads = append(payloads, contract.NetworkChannelKindCountPayload{
			Kind:  strings.TrimSpace(count.Kind),
			Count: count.Count,
		})
	}
	sort.Slice(payloads, func(i int, j int) bool {
		leftRank := networkKindSortRank(payloads[i].Kind)
		rightRank := networkKindSortRank(payloads[j].Kind)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return payloads[i].Kind < payloads[j].Kind
	})
	return payloads, nil
}
