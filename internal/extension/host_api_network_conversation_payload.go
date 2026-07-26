package extensionpkg

import (
	"strings"

	apicontract "github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/store"
)

func hostAPINetworkThreadSummaryPayloads(
	threads []store.NetworkThreadSummary,
) []apicontract.NetworkThreadSummaryPayload {
	payload := make([]apicontract.NetworkThreadSummaryPayload, 0, len(threads))
	for _, thread := range threads {
		payload = append(payload, hostAPINetworkThreadSummaryPayload(thread))
	}
	return payload
}

func hostAPINetworkThreadSummaryPayload(thread store.NetworkThreadSummary) apicontract.NetworkThreadSummaryPayload {
	return apicontract.NetworkThreadSummaryPayload{
		WorkspaceID:        strings.TrimSpace(thread.WorkspaceID),
		Channel:            strings.TrimSpace(thread.Channel),
		ThreadID:           strings.TrimSpace(thread.ThreadID),
		RootMessageID:      strings.TrimSpace(thread.RootMessageID),
		Title:              strings.TrimSpace(thread.Title),
		OpenedByPeerID:     strings.TrimSpace(thread.OpenedByPeerID),
		OpenedSessionID:    strings.TrimSpace(thread.OpenedSessionID),
		OpenedAt:           hostAPITimeValuePtr(thread.OpenedAt),
		LastActivityAt:     hostAPITimeValuePtr(thread.LastActivityAt),
		MessageCount:       thread.MessageCount,
		ParticipantCount:   thread.ParticipantCount,
		OpenWorkCount:      thread.OpenWorkCount,
		LastMessagePreview: strings.TrimSpace(thread.LastMessagePreview),
	}
}

func hostAPINetworkDirectRoomPayloads(
	directs []store.NetworkDirectRoomSummary,
) []apicontract.NetworkDirectRoomPayload {
	payload := make([]apicontract.NetworkDirectRoomPayload, 0, len(directs))
	for _, direct := range directs {
		payload = append(payload, hostAPINetworkDirectRoomPayload(direct))
	}
	return payload
}

func hostAPINetworkDirectRoomPayload(direct store.NetworkDirectRoomSummary) apicontract.NetworkDirectRoomPayload {
	return apicontract.NetworkDirectRoomPayload{
		WorkspaceID:        strings.TrimSpace(direct.WorkspaceID),
		Channel:            strings.TrimSpace(direct.Channel),
		DirectID:           strings.TrimSpace(direct.DirectID),
		SessionA:           strings.TrimSpace(direct.SessionA),
		SessionB:           strings.TrimSpace(direct.SessionB),
		OpenedAt:           hostAPITimeValuePtr(direct.OpenedAt),
		LastActivityAt:     hostAPITimeValuePtr(direct.LastActivityAt),
		MessageCount:       direct.MessageCount,
		OpenWorkCount:      direct.OpenWorkCount,
		LastMessagePreview: strings.TrimSpace(direct.LastMessagePreview),
	}
}

func hostAPINetworkConversationMessagePayloads(
	messages []store.NetworkConversationMessage,
) []apicontract.NetworkConversationMessagePayload {
	payload := make([]apicontract.NetworkConversationMessagePayload, 0, len(messages))
	for _, message := range messages {
		payload = append(payload, hostAPINetworkConversationMessagePayload(message))
	}
	return payload
}

func hostAPINetworkConversationMessagePayload(
	message store.NetworkConversationMessage,
) apicontract.NetworkConversationMessagePayload {
	return apicontract.NetworkConversationMessagePayload{
		MessageID:   strings.TrimSpace(message.MessageID),
		WorkspaceID: strings.TrimSpace(message.WorkspaceID),
		Channel:     strings.TrimSpace(message.Channel),
		Surface:     strings.TrimSpace(message.Surface),
		ThreadID:    strings.TrimSpace(message.ThreadID),
		DirectID:    strings.TrimSpace(message.DirectID),
		Kind:        strings.TrimSpace(message.Kind),
		Direction:   strings.TrimSpace(message.Direction),
		PeerFrom:    strings.TrimSpace(message.PeerFrom),
		PeerTo:      strings.TrimSpace(message.PeerTo),
		SessionID:   strings.TrimSpace(message.SessionID),
		WorkID:      strings.TrimSpace(message.WorkID),
		ReplyTo:     strings.TrimSpace(message.ReplyTo),
		TraceID:     strings.TrimSpace(message.TraceID),
		CausationID: strings.TrimSpace(message.CausationID),
		Intent:      strings.TrimSpace(message.Intent),
		Text:        strings.TrimSpace(message.Text),
		PreviewText: hostAPINetworkMessagePreview(message),
		Body:        hostAPICloneRawMessage(message.Body),
		Timestamp:   message.Timestamp.UTC(),
	}
}

func hostAPINetworkWorkPayload(work store.NetworkWorkEntry) apicontract.NetworkWorkPayload {
	return apicontract.NetworkWorkPayload{
		WorkID:          strings.TrimSpace(work.WorkID),
		WorkspaceID:     strings.TrimSpace(work.WorkspaceID),
		Channel:         strings.TrimSpace(work.Channel),
		Surface:         strings.TrimSpace(work.Surface),
		ThreadID:        strings.TrimSpace(work.ThreadID),
		DirectID:        strings.TrimSpace(work.DirectID),
		OpenedSessionID: strings.TrimSpace(work.OpenedBySessionID),
		State:           strings.TrimSpace(work.State),
		OpenedAt:        hostAPITimeValuePtr(work.OpenedAt),
		LastActivityAt:  hostAPITimeValuePtr(work.LastActivityAt),
		TerminalAt:      hostAPICloneTimePtr(work.TerminalAt),
	}
}

func hostAPINetworkSendPayloadFromRequest(
	id string,
	req apicontract.NetworkSendRequest,
) apicontract.NetworkSendPayload {
	return apicontract.NetworkSendPayload{
		ID:          strings.TrimSpace(id),
		WorkspaceID: strings.TrimSpace(req.WorkspaceID),
		SessionID:   strings.TrimSpace(req.SessionID),
		Channel:     strings.TrimSpace(req.Channel),
		Surface:     strings.TrimSpace(req.Surface),
		ThreadID:    strings.TrimSpace(req.ThreadID),
		DirectID:    strings.TrimSpace(req.DirectID),
		Kind:        strings.TrimSpace(req.Kind),
		To:          strings.TrimSpace(req.To),
		Mentions:    hostAPICloneTrimmedStrings(req.Mentions),
		WorkID:      strings.TrimSpace(req.WorkID),
		ReplyTo:     strings.TrimSpace(req.ReplyTo),
		TraceID:     strings.TrimSpace(req.TraceID),
		CausationID: strings.TrimSpace(req.CausationID),
		ExpiresAt:   hostAPICloneInt64Ptr(req.ExpiresAt),
		Ext:         hostAPICloneRawMap(req.Ext),
	}
}

func hostAPINetworkMessagePreview(message store.NetworkConversationMessage) string {
	if preview := strings.TrimSpace(message.PreviewText); preview != "" {
		return preview
	}
	if text := strings.TrimSpace(message.Text); text != "" {
		return text
	}
	return strings.TrimSpace(string(message.Body))
}
