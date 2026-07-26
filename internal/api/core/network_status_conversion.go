package core

import (
	"sort"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network"
)

// NetworkStatusPayloadFromStatus converts the runtime network status snapshot into the shared payload.
func NetworkStatusPayloadFromStatus(status *network.Status) *contract.NetworkStatusPayload {
	if status == nil {
		return nil
	}

	kindMetrics := make([]contract.NetworkKindMetricPayload, 0, len(status.KindMetrics))
	for _, metric := range status.KindMetrics {
		kindMetrics = append(kindMetrics, contract.NetworkKindMetricPayload{
			Kind:      string(metric.Kind),
			Sent:      metric.Sent,
			Received:  metric.Received,
			Rejected:  metric.Rejected,
			Delivered: metric.Delivered,
		})
	}

	return &contract.NetworkStatusPayload{
		Enabled:              status.Enabled,
		Status:               strings.TrimSpace(status.Status),
		LocalPeers:           status.LocalPeers,
		Channels:             status.Channels,
		MessagesSent:         status.MessagesSent,
		MessagesReceived:     status.MessagesReceived,
		MessagesRejected:     status.MessagesRejected,
		MessagesDelivered:    status.MessagesDelivered,
		WorkflowTaggedEvents: status.WorkflowTaggedEvents,
		HandoffTaggedEvents:  status.HandoffTaggedEvents,
		OpenThreads:          status.OpenThreads,
		OpenDirectRooms:      status.OpenDirectRooms,
		OpenWorkItems:        status.OpenWorkItems,
		ConversationMessages: status.ConversationMessages,
		WorkTransitions:      status.WorkTransitions,
		DirectResolves:       status.DirectResolves,
		KindMetrics:          kindMetrics,
	}
}

// NetworkSendPayloadFromRequest builds the shared send response payload
// from the original request plus the assigned message id.
func NetworkSendPayloadFromRequest(id string, req contract.NetworkSendRequest) contract.NetworkSendPayload {
	return contract.NetworkSendPayload{
		ID:          strings.TrimSpace(id),
		WorkspaceID: strings.TrimSpace(req.WorkspaceID),
		SessionID:   strings.TrimSpace(req.SessionID),
		Channel:     strings.TrimSpace(req.Channel),
		Surface:     strings.TrimSpace(req.Surface),
		ThreadID:    strings.TrimSpace(req.ThreadID),
		DirectID:    strings.TrimSpace(req.DirectID),
		Kind:        strings.TrimSpace(req.Kind),
		To:          strings.TrimSpace(req.To),
		Mentions:    cloneTrimmedStrings(req.Mentions),
		WorkID:      strings.TrimSpace(req.WorkID),
		ReplyTo:     strings.TrimSpace(req.ReplyTo),
		TraceID:     strings.TrimSpace(req.TraceID),
		CausationID: strings.TrimSpace(req.CausationID),
		ExpiresAt:   cloneInt64Ptr(req.ExpiresAt),
		Ext:         cloneRawMap(req.Ext),
	}
}

// NetworkPeerPayloadsFromInfos converts the visible peer snapshot into shared payloads.
func NetworkPeerPayloadsFromInfos(peers []network.PeerInfo) []contract.NetworkPeerPayload {
	payload := make([]contract.NetworkPeerPayload, 0, len(peers))
	for _, peer := range peers {
		payload = append(payload, NetworkPeerPayloadFromInfo(peer))
	}
	sortNetworkPeerPayloads(payload)
	return payload
}

func sortNetworkPeerPayloads(peers []contract.NetworkPeerPayload) {
	sort.Slice(peers, func(i int, j int) bool {
		if peers[i].Local != peers[j].Local {
			return peers[i].Local
		}

		left := networkPeerSortTimestamp(peers[i])
		right := networkPeerSortTimestamp(peers[j])
		switch {
		case left != nil && right != nil && !left.Equal(*right):
			return left.After(*right)
		case left != nil && right == nil:
			return true
		case left == nil && right != nil:
			return false
		}

		leftName := networkPeerSortName(peers[i])
		rightName := networkPeerSortName(peers[j])
		if leftName != rightName {
			return leftName < rightName
		}
		if peers[i].PeerID != peers[j].PeerID {
			return peers[i].PeerID < peers[j].PeerID
		}
		return peers[i].Channel < peers[j].Channel
	})
}

func networkPeerSortTimestamp(peer contract.NetworkPeerPayload) *time.Time {
	return peer.JoinedAt
}

func networkPeerSortName(peer contract.NetworkPeerPayload) string {
	if value := strings.TrimSpace(peer.DisplayName); value != "" {
		return value
	}
	return strings.TrimSpace(peer.PeerID)
}

// NetworkPeerPayloadFromInfo converts one visible peer snapshot into the shared payload.
func NetworkPeerPayloadFromInfo(peer network.PeerInfo) contract.NetworkPeerPayload {
	displayName := peer.PeerID
	if peer.PeerCard.DisplayName != nil {
		if trimmed := strings.TrimSpace(*peer.PeerCard.DisplayName); trimmed != "" {
			displayName = trimmed
		}
	}
	return contract.NetworkPeerPayload{
		WorkspaceID:   strings.TrimSpace(peer.WorkspaceID),
		SessionID:     peer.SessionID,
		PeerID:        peer.PeerID,
		DisplayName:   displayName,
		Channel:       peer.Channel,
		Local:         peer.Local,
		PeerCard:      networkPeerCardPayload(peer),
		JoinedAt:      cloneTimePtr(peer.JoinedAt),
		PresenceState: networkPresenceState(peer),
	}
}

func networkPresenceState(peer network.PeerInfo) string {
	state := string(peer.PresenceState)
	if state == "" {
		state = contract.NetworkPresenceLocal
	}
	if state != contract.NetworkPresenceLocal {
		return contract.NetworkPresenceLocal
	}
	return state
}

func networkPeerCardPayload(peer network.PeerInfo) contract.NetworkPeerCardPayload {
	capabilityCatalog := peer.CapabilityCatalog
	if !peer.CapabilityCatalogKnown {
		capabilityCatalog = nil
	}

	return contract.NetworkPeerCardPayload{
		PeerID:              peer.PeerCard.PeerID,
		DisplayName:         peer.PeerCard.DisplayName,
		ProfilesSupported:   cloneNetworkStringSliceOrEmpty(peer.PeerCard.ProfilesSupported),
		Capabilities:        networkCapabilityBriefPayloads(peer.PeerCard, capabilityCatalog),
		ArtifactsSupported:  cloneNetworkStringSliceOrEmpty(peer.PeerCard.ArtifactsSupported),
		TrustModesSupported: cloneNetworkStringSliceOrEmpty(peer.PeerCard.TrustModesSupported),
		Ext:                 clonePeerCardExtWithoutCapabilityDiscovery(peer.PeerCard.Ext),
	}
}

func cloneNetworkStringSliceOrEmpty(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string(nil), values...)
}

func cloneTrimmedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		cloned = append(cloned, trimmed)
	}
	if len(cloned) == 0 {
		return nil
	}
	return cloned
}
