package extensionpkg

import (
	"sort"
	"strings"

	apicontract "github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/network"
)

func hostAPINetworkStatusPayload(status *network.Status) apicontract.NetworkStatusPayload {
	if status == nil {
		return apicontract.NetworkStatusPayload{}
	}
	kindMetrics := make([]apicontract.NetworkKindMetricPayload, 0, len(status.KindMetrics))
	for _, metric := range status.KindMetrics {
		kindMetrics = append(kindMetrics, apicontract.NetworkKindMetricPayload{
			Kind:      string(metric.Kind),
			Sent:      metric.Sent,
			Received:  metric.Received,
			Rejected:  metric.Rejected,
			Delivered: metric.Delivered,
		})
	}
	return apicontract.NetworkStatusPayload{
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

func hostAPINetworkChannelPayloads(channels []network.ChannelInfo) []apicontract.NetworkChannelPayload {
	payload := make([]apicontract.NetworkChannelPayload, 0, len(channels))
	for _, channel := range channels {
		payload = append(payload, apicontract.NetworkChannelPayload{
			WorkspaceID: strings.TrimSpace(channel.WorkspaceID),
			Channel:     strings.TrimSpace(channel.Channel),
			PeerCount:   channel.PeerCount,
		})
	}
	sort.Slice(payload, func(i int, j int) bool {
		return payload[i].Channel < payload[j].Channel
	})
	return payload
}

func hostAPINetworkPeerPayloads(peers []network.PeerInfo) []apicontract.NetworkPeerPayload {
	payload := make([]apicontract.NetworkPeerPayload, 0, len(peers))
	for _, peer := range peers {
		payload = append(payload, hostAPINetworkPeerPayload(peer))
	}
	sort.Slice(payload, func(i int, j int) bool {
		if payload[i].Local != payload[j].Local {
			return payload[i].Local
		}
		if payload[i].PeerID != payload[j].PeerID {
			return payload[i].PeerID < payload[j].PeerID
		}
		return payload[i].Channel < payload[j].Channel
	})
	return payload
}

func hostAPINetworkPeerPayload(peer network.PeerInfo) apicontract.NetworkPeerPayload {
	displayName := strings.TrimSpace(peer.PeerID)
	if peer.PeerCard.DisplayName != nil {
		if trimmed := strings.TrimSpace(*peer.PeerCard.DisplayName); trimmed != "" {
			displayName = trimmed
		}
	}
	return apicontract.NetworkPeerPayload{
		WorkspaceID: strings.TrimSpace(peer.WorkspaceID),
		SessionID:   hostAPICloneStringPtr(peer.SessionID),
		PeerID:      strings.TrimSpace(peer.PeerID),
		DisplayName: displayName,
		Channel:     strings.TrimSpace(peer.Channel),
		Local:       peer.Local,
		PeerCard: apicontract.NetworkPeerCardPayload{
			PeerID:              strings.TrimSpace(peer.PeerCard.PeerID),
			DisplayName:         hostAPICloneStringPtr(peer.PeerCard.DisplayName),
			ProfilesSupported:   append([]string(nil), peer.PeerCard.ProfilesSupported...),
			Capabilities:        hostAPINetworkCapabilityBriefPayloads(peer.PeerCard.Capabilities),
			ArtifactsSupported:  append([]string(nil), peer.PeerCard.ArtifactsSupported...),
			TrustModesSupported: append([]string(nil), peer.PeerCard.TrustModesSupported...),
			Ext:                 hostAPICloneRawMap(peer.PeerCard.Ext),
		},
		JoinedAt:      hostAPICloneTimePtr(peer.JoinedAt),
		PresenceState: apicontract.NetworkPresenceLocal,
	}
}

func hostAPINetworkCapabilityBriefPayloads(ids []string) []apicontract.NetworkCapabilityBriefPayload {
	payload := make([]apicontract.NetworkCapabilityBriefPayload, 0, len(ids))
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		payload = append(payload, apicontract.NetworkCapabilityBriefPayload{ID: trimmed})
	}
	if len(payload) == 0 {
		return []apicontract.NetworkCapabilityBriefPayload{}
	}
	return payload
}
