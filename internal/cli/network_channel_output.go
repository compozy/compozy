package cli

import (
	"strconv"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
)

func networkStatusBundle(status NetworkStatusRecord) outputBundle {
	rows := []keyValue{
		{Label: networkEnabledValue, Value: strconv.FormatBool(status.Enabled)},
		{Label: networkStatusValue, Value: stringOrDash(status.Status)},
		{Label: "Live Participants", Value: strconv.Itoa(status.LocalPeers)},
		{Label: "Channels", Value: strconv.Itoa(status.Channels)},
		{Label: "Messages Sent", Value: strconv.FormatInt(status.MessagesSent, 10)},
		{Label: "Messages Received", Value: strconv.FormatInt(status.MessagesReceived, 10)},
		{Label: "Messages Rejected", Value: strconv.FormatInt(status.MessagesRejected, 10)},
		{Label: "Messages Delivered", Value: strconv.FormatInt(status.MessagesDelivered, 10)},
		{Label: "Workflow Tagged", Value: strconv.FormatInt(status.WorkflowTaggedEvents, 10)},
		{Label: "Handoff Tagged", Value: strconv.FormatInt(status.HandoffTaggedEvents, 10)},
	}
	fields := []string{
		networkEnabledKey, networkStatusKey, "local_peers", networkChannelsKey, "messages_sent",
		"messages_received", "messages_rejected", "messages_delivered",
		"workflow_tagged_events", "handoff_tagged_events",
	}
	values := []string{
		strconv.FormatBool(status.Enabled),
		status.Status,
		strconv.Itoa(status.LocalPeers),
		strconv.Itoa(status.Channels),
		strconv.FormatInt(status.MessagesSent, 10),
		strconv.FormatInt(status.MessagesReceived, 10),
		strconv.FormatInt(status.MessagesRejected, 10),
		strconv.FormatInt(status.MessagesDelivered, 10),
		strconv.FormatInt(status.WorkflowTaggedEvents, 10),
		strconv.FormatInt(status.HandoffTaggedEvents, 10),
	}

	return outputBundle{
		jsonValue: status,
		human: func() (string, error) {
			return renderHumanBlocks(
				renderHumanSection("Network", rows),
				renderHumanTable(
					"Kind Metrics",
					[]string{networkKindValue, "Sent", "Received", "Rejected", "Delivered"},
					networkKindMetricRows(status.KindMetrics),
				),
			), nil
		},
		toon: func() (string, error) {
			return renderHumanBlocks(
				renderToonObject(networkNetworkKey, fields, values),
				renderToonArray(
					"network_kind_metrics",
					[]string{networkKindKey, networkSentKey, "received", "rejected", "delivered"},
					networkKindMetricRows(status.KindMetrics),
				),
			), nil
		},
	}
}

func networkPeersBundle(peers []NetworkPeerRecord) outputBundle {
	return listBundle(
		peers,
		peers,
		"Network Peers",
		[]string{
			taskPeerValue,
			"Display",
			agentKernelSessionValue,
			networkChannelValue,
			networkPresenceValue,
			"Local",
			"Joined",
		},
		"network_peers",
		[]string{
			networkPeerIDKey,
			bridgeSetupDisplayNameKey,
			"session_id",
			networkChannelKey,
			"presence_state",
			networkLocalKey,
			"joined_at",
		},
		func(peer NetworkPeerRecord) []string {
			return []string{
				stringOrDash(peer.PeerID),
				stringOrDash(optionalString(peer.PeerCard.DisplayName)),
				stringOrDash(optionalString(peer.SessionID)),
				stringOrDash(peer.Channel),
				stringOrDash(networkPeerPresenceLabel(peer)),
				strconv.FormatBool(peer.Local),
				stringOrDash(formatTimePtr(peer.JoinedAt)),
			}
		},
		func(peer NetworkPeerRecord) []string {
			return []string{
				peer.PeerID,
				optionalString(peer.PeerCard.DisplayName),
				optionalString(peer.SessionID),
				peer.Channel,
				peer.PresenceState,
				strconv.FormatBool(peer.Local),
				formatTimePtr(peer.JoinedAt),
			}
		},
	)
}

func networkPeerPresenceLabel(peer NetworkPeerRecord) string {
	state := strings.TrimSpace(peer.PresenceState)
	if state == "" {
		return ""
	}
	return state
}

func networkChannelsBundle(channels []NetworkChannelRecord) outputBundle {
	return listBundle(
		channels,
		channels,
		"Network Channels",
		[]string{networkChannelValue, "Peers"},
		"network_channels",
		[]string{networkChannelKey, "peer_count"},
		func(channel NetworkChannelRecord) []string {
			return []string{
				stringOrDash(channel.Channel),
				strconv.Itoa(channel.PeerCount),
			}
		},
		func(channel NetworkChannelRecord) []string {
			return []string{
				channel.Channel,
				strconv.Itoa(channel.PeerCount),
			}
		},
	)
}

func networkChannelBundle(channel NetworkChannelDetailRecord) outputBundle {
	fields := []string{
		networkChannelKey,
		automationWorkspaceIDKey,
		"purpose",
		createdByKey,
		"peer_count",
		"session_count",
		networkMessageCountKey,
	}
	values := []string{
		channel.Channel,
		channel.WorkspaceID,
		channel.Purpose,
		channel.CreatedBy,
		strconv.Itoa(channel.PeerCount),
		strconv.Itoa(channel.SessionCount),
		strconv.Itoa(channel.MessageCount),
	}
	return outputBundle{
		jsonValue: channel,
		human: func() (string, error) {
			return renderHumanBlocks(
				renderHumanSection("Network Channel", []keyValue{
					{Label: networkChannelValue, Value: stringOrDash(channel.Channel)},
					{Label: networkWorkspaceValue, Value: stringOrDash(channel.WorkspaceID)},
					{Label: "Purpose", Value: stringOrDash(channel.Purpose)},
					{Label: networkCreatedByValue, Value: stringOrDash(channel.CreatedBy)},
					{Label: "Peers", Value: strconv.Itoa(channel.PeerCount)},
					{Label: "Sessions", Value: strconv.Itoa(channel.SessionCount)},
					{Label: networkMessagesValue, Value: strconv.Itoa(channel.MessageCount)},
				}),
			), nil
		},
		toon: func() (string, error) {
			return renderToonObject("network_channel", fields, values), nil
		},
	}
}

func networkSubscriptionsBundle(subscriptions []NetworkSubscriptionRecord) outputBundle {
	return listBundle(
		contract.NetworkSubscriptionsResponse{Subscriptions: subscriptions},
		subscriptions,
		"Network Subscriptions",
		[]string{networkChannelValue, taskThreadValue, sessionIDLabel, networkModeValue},
		"network_subscriptions",
		[]string{networkChannelKey, networkThreadIDKey, taskSessionIDKey, networkModeKey},
		func(subscription NetworkSubscriptionRecord) []string {
			return []string{
				stringOrDash(subscription.Channel),
				stringOrDash(subscription.ThreadID),
				stringOrDash(subscription.SessionID),
				stringOrDash(subscription.Mode),
			}
		},
		func(subscription NetworkSubscriptionRecord) []string {
			return []string{
				subscription.Channel,
				subscription.ThreadID,
				subscription.SessionID,
				subscription.Mode,
			}
		},
	)
}

func networkSubscriptionBundle(subscription NetworkSubscriptionRecord) outputBundle {
	return outputBundle{
		jsonValue: contract.NetworkSubscriptionResponse{Subscription: subscription},
		human: func() (string, error) {
			return renderHumanSection("Network Subscription", []keyValue{
				{Label: networkChannelValue, Value: stringOrDash(subscription.Channel)},
				{Label: taskThreadValue, Value: stringOrDash(subscription.ThreadID)},
				{Label: sessionIDLabel, Value: stringOrDash(subscription.SessionID)},
				{Label: networkModeValue, Value: stringOrDash(subscription.Mode)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"network_subscription",
				[]string{networkChannelKey, networkThreadIDKey, taskSessionIDKey, networkModeKey},
				[]string{
					subscription.Channel,
					subscription.ThreadID,
					subscription.SessionID,
					subscription.Mode,
				},
			), nil
		},
	}
}

func networkSubscriptionDeleteBundle(channel string, threadID string, sessionID string) outputBundle {
	payload := map[string]any{
		networkChannelKey:  strings.TrimSpace(channel),
		networkThreadIDKey: strings.TrimSpace(threadID),
		taskSessionIDKey:   strings.TrimSpace(sessionID),
		networkDeletedKey:  true,
	}
	return outputBundle{
		jsonValue: payload,
		human: func() (string, error) {
			return renderHumanSection("Network Subscription", []keyValue{
				{Label: networkChannelValue, Value: stringOrDash(strings.TrimSpace(channel))},
				{Label: taskThreadValue, Value: stringOrDash(strings.TrimSpace(threadID))},
				{Label: sessionIDLabel, Value: stringOrDash(strings.TrimSpace(sessionID))},
				{Label: "Deleted", Value: networkDeletedValue},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"network_subscription",
				[]string{networkChannelKey, networkThreadIDKey, taskSessionIDKey, networkDeletedKey},
				[]string{
					strings.TrimSpace(channel),
					strings.TrimSpace(threadID),
					strings.TrimSpace(sessionID),
					networkDeletedValue,
				},
			), nil
		},
	}
}
