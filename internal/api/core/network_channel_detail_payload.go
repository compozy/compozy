package core

import (
	"context"

	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

func (h *BaseHandlers) createNetworkChannelSessions(
	ctx context.Context,
	channel string,
	workspaceID string,
	agentNames []string,
) ([]string, error) {
	createdIDs := make([]string, 0, len(agentNames))
	for _, agentName := range agentNames {
		sess, err := h.Sessions.Create(ctx, session.CreateOpts{
			AgentName:            agentName,
			Provider:             "",
			Workspace:            workspaceID,
			NetworkParticipation: namedParticipationRequest(channel),
			Type:                 session.SessionTypeUser,
		})
		if err != nil {
			return createdIDs, err
		}
		if sess != nil && sess.Info() != nil {
			createdIDs = append(createdIDs, sess.Info().ID)
		}
	}
	return createdIDs, nil
}

func (h *BaseHandlers) networkChannelDetailPayload(
	ctx context.Context,
	service NetworkService,
	workspaceID string,
	channel string,
) (contract.NetworkChannelDetailPayload, error) {
	networkStore, err := h.networkStoreRequired()
	if err != nil {
		return contract.NetworkChannelDetailPayload{}, err
	}
	trimmedWorkspaceID := strings.TrimSpace(workspaceID)
	if trimmedWorkspaceID == "" {
		return contract.NetworkChannelDetailPayload{}, errors.New("api: network workspace_id is required")
	}
	peers, err := service.ListPeers(ctx, trimmedWorkspaceID, channel)
	if err != nil {
		return contract.NetworkChannelDetailPayload{}, err
	}
	sessions, err := h.Sessions.ListAll(ctx)
	if err != nil {
		return contract.NetworkChannelDetailPayload{}, err
	}

	filteredSessions := sessionsForChannel(sessions, trimmedWorkspaceID, channel)
	metadata, err := h.loadNetworkChannelMetadata(ctx, networkStore, store.NetworkChannelRef{
		WorkspaceID: trimmedWorkspaceID,
		Channel:     channel,
	})
	if err != nil {
		return contract.NetworkChannelDetailPayload{}, err
	}
	projections, err := listNetworkChannelProjections(ctx, networkStore, trimmedWorkspaceID, channel)
	if err != nil {
		return contract.NetworkChannelDetailPayload{}, err
	}
	if len(filteredSessions) == 0 && len(peers) == 0 && len(projections) == 0 && metadata == nil {
		return contract.NetworkChannelDetailPayload{}, fmt.Errorf("%w: %s", errNetworkChannelNotFound, channel)
	}

	payloadPeers, localPeerCount := networkChannelPeerPayloads(peers, filteredSessions)

	metadataFields := networkChannelMetadataPayloadFields(metadata)
	projection := firstNetworkChannelProjection(projections)
	kindCounts, err := listNetworkChannelKindCountPayloads(ctx, networkStore, store.NetworkChannelRef{
		WorkspaceID: trimmedWorkspaceID,
		Channel:     channel,
	})
	if err != nil {
		return contract.NetworkChannelDetailPayload{}, err
	}

	return contract.NetworkChannelDetailPayload{
		Channel:                    channel,
		WorkspaceID:                firstNonEmpty(metadataFields.workspaceID, trimmedWorkspaceID),
		Purpose:                    metadataFields.purpose,
		FanoutPolicy:               metadataFields.fanoutPolicy,
		CoordinatorPeerID:          metadataFields.coordinatorPeerID,
		CreatedBy:                  metadataFields.createdBy,
		CreatedAt:                  metadataFields.createdAt,
		PeerCount:                  len(peers),
		LocalPeerCount:             localPeerCount,
		SessionCount:               len(filteredSessions),
		MessageCount:               projection.MessageCount,
		PresenceCount:              projection.PresenceCount,
		HistoricalParticipantCount: projection.HistoricalParticipantCount,
		LastActivityAt:             cloneTimePtr(projection.LastActivityAt),
		LastPresenceAt:             cloneTimePtr(projection.LastPresenceAt),
		LastMessagePreview:         strings.TrimSpace(projection.LastMessagePreview),
		KindCounts:                 kindCounts,
		Sessions:                   SessionPayloadsFromInfos(filteredSessions),
		Peers:                      payloadPeers,
	}, nil
}

func networkChannelPeerPayloads(
	peers []network.PeerInfo,
	sessions []*session.Info,
) ([]contract.NetworkPeerPayload, int) {
	sessionByID := sessionInfoMapByID(sessions)
	payloads := make([]contract.NetworkPeerPayload, 0, len(peers))
	localPeerCount := 0
	for _, peer := range peers {
		if peer.Local {
			localPeerCount++
		}
		payloads = append(payloads, networkPeerPayloadFromInfoWithSessions(peer, sessionByID))
	}
	sortNetworkPeerPayloads(payloads)
	return payloads, localPeerCount
}

func networkChannelMetadataPayloadFields(metadata *store.NetworkChannelEntry) networkChannelMetadataFields {
	if metadata == nil {
		return networkChannelMetadataFields{}
	}
	return networkChannelMetadataFields{
		createdAt:         cloneTimePtr(&metadata.CreatedAt),
		purpose:           strings.TrimSpace(metadata.Purpose),
		fanoutPolicy:      strings.TrimSpace(metadata.FanoutPolicy),
		coordinatorPeerID: strings.TrimSpace(metadata.CoordinatorPeerID),
		workspaceID:       strings.TrimSpace(metadata.WorkspaceID),
		createdBy:         strings.TrimSpace(metadata.CreatedBy),
	}
}

func ensureNetworkChannelAggregate(
	aggregates map[string]*networkChannelAggregate,
	workspaceID string,
	channel string,
) *networkChannelAggregate {
	trimmed := strings.TrimSpace(channel)
	aggregate, ok := aggregates[trimmed]
	if ok && aggregate != nil {
		return aggregate
	}
	aggregate = &networkChannelAggregate{
		workspaceID: strings.TrimSpace(workspaceID),
		channel:     trimmed,
	}
	aggregates[trimmed] = aggregate
	return aggregate
}

func sessionsForChannel(sessions []*session.Info, workspaceID string, channel string) []*session.Info {
	filtered := make([]*session.Info, 0, len(sessions))
	for _, info := range sessions {
		if !networkChannelSessionVisible(info) ||
			strings.TrimSpace(info.WorkspaceID) != strings.TrimSpace(workspaceID) ||
			strings.TrimSpace(info.NetworkParticipation.ChannelID) != channel {
			continue
		}
		filtered = append(filtered, info)
	}
	return filtered
}

func networkChannelExists(
	sessions []*session.Info,
	peers []network.PeerInfo,
	metadata *store.NetworkChannelEntry,
	workspaceID string,
	channel string,
) bool {
	if metadata != nil {
		return true
	}
	for _, info := range sessions {
		if networkChannelSessionVisible(info) &&
			strings.TrimSpace(info.WorkspaceID) == strings.TrimSpace(workspaceID) &&
			strings.TrimSpace(info.NetworkParticipation.ChannelID) == channel {
			return true
		}
	}
	for _, peer := range peers {
		if strings.TrimSpace(peer.WorkspaceID) == strings.TrimSpace(workspaceID) &&
			strings.TrimSpace(peer.Channel) == channel {
			return true
		}
	}
	return false
}

func networkChannelSessionVisible(info *session.Info) bool {
	if info == nil {
		return false
	}
	if info.State == session.StateStopped {
		return false
	}
	return info.NetworkParticipation.Mode == participation.ModeLive
}

func namedParticipationRequest(channelID string) *participation.Request {
	live := participation.ModeLive
	named := participation.StrategyNamed
	trimmed := strings.TrimSpace(channelID)
	return &participation.Request{
		Mode:            &live,
		ChannelStrategy: &named,
		ChannelID:       &trimmed,
	}
}

func isNetworkChannelNotFound(err error) bool {
	return errors.Is(err, errNetworkChannelNotFound)
}
