package core

import (
	"context"

	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network"

	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

func indexNetworkTimelineViewByMessageID(views []networkTimelineMessageView, messageID string) int {
	target := strings.TrimSpace(messageID)
	for index, view := range views {
		if strings.TrimSpace(view.entry.MessageID) == target {
			return index
		}
	}
	return -1
}

func (h *BaseHandlers) loadPeerAuditEntries(
	ctx context.Context,
	networkStore NetworkStore,
	peer network.PeerInfo,
) ([]store.NetworkAuditEntry, error) {
	if peer.SessionID != nil {
		return networkStore.ListNetworkAudit(ctx, store.NetworkAuditQuery{
			WorkspaceID: strings.TrimSpace(peer.WorkspaceID),
			SessionID:   strings.TrimSpace(*peer.SessionID),
		})
	}

	entries, err := networkStore.ListNetworkAudit(ctx, store.NetworkAuditQuery{
		WorkspaceID: strings.TrimSpace(peer.WorkspaceID),
		Channel:     strings.TrimSpace(peer.Channel),
	})
	if err != nil {
		return nil, err
	}

	filtered := make([]store.NetworkAuditEntry, 0, len(entries))
	for _, entry := range entries {
		if networkAuditMatchesPeer(peer, entry) {
			filtered = append(filtered, entry)
		}
	}
	return filtered, nil
}

func networkAuditMatchesPeer(peer network.PeerInfo, entry store.NetworkAuditEntry) bool {
	targetPeerID := strings.TrimSpace(peer.PeerID)
	if targetPeerID == "" {
		return false
	}
	if peer.SessionID != nil && strings.TrimSpace(entry.SessionID) == strings.TrimSpace(*peer.SessionID) {
		return true
	}
	return strings.TrimSpace(entry.PeerFrom) == targetPeerID || strings.TrimSpace(entry.PeerTo) == targetPeerID
}

func summarizePeerMetrics(peer network.PeerInfo, entries []store.NetworkAuditEntry) contract.NetworkPeerMetricsPayload {
	metrics := contract.NetworkPeerMetricsPayload{}
	for _, entry := range entries {
		if !networkAuditMatchesPeer(peer, entry) {
			continue
		}
		entrySize := int64(entry.Size)
		metrics.TotalSizeBytes += entrySize
		switch strings.TrimSpace(entry.Direction) {
		case network.AuditDirectionSent:
			metrics.Sent++
			metrics.SentSizeBytes += entrySize
		case network.AuditDirectionReceived:
			metrics.Received++
			metrics.ReceivedSizeBytes += entrySize
		case network.AuditDirectionRejected:
			metrics.Rejected++
			metrics.RejectedSizeBytes += entrySize
		case network.AuditDirectionDelivered:
			metrics.Delivered++
			metrics.DeliveredSizeBytes += entrySize
		}
	}
	return metrics
}

// NetworkPeerDetailPayloadFromInfo converts one peer info plus metrics into the shared detail payload.
func NetworkPeerDetailPayloadFromInfo(
	peer network.PeerInfo,
	sessionsByID map[string]*session.Info,
	metrics contract.NetworkPeerMetricsPayload,
) contract.NetworkPeerDetailPayload {
	payload := contract.NetworkPeerDetailPayload{
		SessionID:         peer.SessionID,
		PeerID:            peer.PeerID,
		DisplayName:       networkPeerDisplayName(peer, sessionsByID),
		Channel:           peer.Channel,
		Local:             peer.Local,
		PeerCard:          NetworkPeerPayloadFromInfo(peer).PeerCard,
		CapabilityCatalog: networkCapabilityCatalogPayload(peer),
		JoinedAt:          cloneTimePtr(peer.JoinedAt),
		PresenceState:     networkPresenceState(peer),
		Metrics:           metrics,
	}
	return payload
}
