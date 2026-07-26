package core

import (
	"context"
	"database/sql"
	"errors"

	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network"

	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

func sessionInfoMapByID(sessions []*session.Info) map[string]*session.Info {
	index := make(map[string]*session.Info, len(sessions))
	for _, info := range sessions {
		if info == nil {
			continue
		}
		index[strings.TrimSpace(info.ID)] = info
	}
	return index
}

func peerInfoMapByID(peers []network.PeerInfo) map[string]network.PeerInfo {
	index := make(map[string]network.PeerInfo, len(peers))
	for _, peer := range peers {
		index[strings.TrimSpace(peer.PeerID)] = peer
	}
	return index
}

func networkChannelPayloadFromAggregate(
	aggregate *networkChannelAggregate,
) contract.NetworkChannelPayload {
	payload := contract.NetworkChannelPayload{
		Channel:                    aggregate.channel,
		WorkspaceID:                strings.TrimSpace(aggregate.workspaceID),
		PeerCount:                  aggregate.peerCount,
		LocalPeerCount:             aggregate.localPeerCount,
		SessionCount:               aggregate.sessionCount,
		MessageCount:               aggregate.messageCount,
		PresenceCount:              aggregate.presenceCount,
		HistoricalParticipantCount: aggregate.historicalParticipantCount,
		LastActivityAt:             cloneTimePtr(aggregate.lastActivityAt),
		LastPresenceAt:             cloneTimePtr(aggregate.lastPresenceAt),
		LastMessagePreview:         strings.TrimSpace(aggregate.lastMessagePreview),
	}
	if aggregate.metadata == nil {
		return payload
	}
	payload.WorkspaceID = firstNonEmpty(
		strings.TrimSpace(aggregate.metadata.WorkspaceID),
		strings.TrimSpace(aggregate.workspaceID),
	)
	payload.Purpose = strings.TrimSpace(aggregate.metadata.Purpose)
	payload.FanoutPolicy = strings.TrimSpace(aggregate.metadata.FanoutPolicy)
	payload.CoordinatorPeerID = strings.TrimSpace(aggregate.metadata.CoordinatorPeerID)
	payload.CreatedBy = strings.TrimSpace(aggregate.metadata.CreatedBy)
	payload.CreatedAt = cloneTimePtr(&aggregate.metadata.CreatedAt)
	return payload
}

func networkKindSortRank(kind string) int {
	switch strings.TrimSpace(kind) {
	case string(network.KindSay):
		return 0
	case string(network.KindReceipt):
		return 1
	case string(network.KindCapability):
		return 2
	case string(network.KindGreet):
		return 3
	case string(network.KindWhois):
		return 4
	case string(network.KindTrace):
		return 5
	default:
		return 100
	}
}

func networkMessagePreview(entry store.NetworkMessageEntry) string {
	if preview := strings.TrimSpace(entry.PreviewText); preview != "" {
		return preview
	}
	if text := strings.TrimSpace(entry.Text); text != "" {
		return text
	}
	return network.PreviewTextForRawBody(network.Kind(strings.TrimSpace(entry.Kind)), entry.Body)
}

func (h *BaseHandlers) loadNetworkChannelMetadata(
	ctx context.Context,
	networkStore NetworkStore,
	ref store.NetworkChannelRef,
) (*store.NetworkChannelEntry, error) {
	entry, err := networkStore.GetNetworkChannel(ctx, ref)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &entry, nil
}

func findPeerInfo(peers []network.PeerInfo, peerID string) (network.PeerInfo, bool) {
	target := strings.TrimSpace(peerID)
	for _, peer := range peers {
		if strings.TrimSpace(peer.PeerID) == target {
			return peer, true
		}
	}
	return network.PeerInfo{}, false
}

func laterTimePtr(current *time.Time, candidate time.Time) *time.Time {
	if candidate.IsZero() {
		return cloneTimePtr(current)
	}
	if current == nil || candidate.After(current.UTC()) {
		value := candidate.UTC()
		return &value
	}
	return cloneTimePtr(current)
}

func networkPeerPayloadFromInfoWithSessions(
	peer network.PeerInfo,
	sessionsByID map[string]*session.Info,
) contract.NetworkPeerPayload {
	payload := NetworkPeerPayloadFromInfo(peer)
	payload.DisplayName = networkPeerDisplayName(peer, sessionsByID)
	return payload
}

func networkPeerDisplayName(peer network.PeerInfo, sessionsByID map[string]*session.Info) string {
	if peer.PeerCard.DisplayName != nil {
		if value := strings.TrimSpace(*peer.PeerCard.DisplayName); value != "" {
			return value
		}
	}
	if peer.SessionID != nil && sessionsByID != nil {
		if info, ok := sessionsByID[strings.TrimSpace(*peer.SessionID)]; ok && info != nil {
			if value := strings.TrimSpace(info.Name); value != "" {
				return value
			}
			if value := strings.TrimSpace(info.AgentName); value != "" {
				return value
			}
		}
	}
	return strings.TrimSpace(peer.PeerID)
}
