package core

import (
	"strings"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/store"
)

// NetworkThreadSummaryPayloadsFromStore converts stored thread summaries into public payloads.
func NetworkThreadSummaryPayloadsFromStore(
	threads []store.NetworkThreadSummary,
) []contract.NetworkThreadSummaryPayload {
	payload := make([]contract.NetworkThreadSummaryPayload, 0, len(threads))
	for _, thread := range threads {
		payload = append(payload, NetworkThreadSummaryPayloadFromStore(thread))
	}
	return payload
}

// NetworkThreadSummaryPayloadFromStore converts one stored thread summary into a public payload.
func NetworkThreadSummaryPayloadFromStore(thread store.NetworkThreadSummary) contract.NetworkThreadSummaryPayload {
	return contract.NetworkThreadSummaryPayload{
		WorkspaceID:        strings.TrimSpace(thread.WorkspaceID),
		Channel:            strings.TrimSpace(thread.Channel),
		ThreadID:           strings.TrimSpace(thread.ThreadID),
		RootMessageID:      strings.TrimSpace(thread.RootMessageID),
		Title:              strings.TrimSpace(thread.Title),
		OpenedByPeerID:     strings.TrimSpace(thread.OpenedByPeerID),
		OpenedSessionID:    strings.TrimSpace(thread.OpenedSessionID),
		OpenedAt:           cloneTimePtr(&thread.OpenedAt),
		LastActivityAt:     cloneTimePtr(&thread.LastActivityAt),
		MessageCount:       thread.MessageCount,
		ParticipantCount:   thread.ParticipantCount,
		OpenWorkCount:      thread.OpenWorkCount,
		CoordinationCost:   NetworkCoordinationCostPayloadFromStore(thread),
		LastMessagePreview: strings.TrimSpace(thread.LastMessagePreview),
	}
}

// NetworkCoordinationCostPayloadFromStore converts stored thread cost counters into public payloads.
func NetworkCoordinationCostPayloadFromStore(
	thread store.NetworkThreadSummary,
) *contract.NetworkCoordinationCostPayload {
	if thread.DeliveredCount == 0 && thread.PromptSizeBytes == 0 && thread.EstimatedPromptTokens == 0 {
		return nil
	}
	return &contract.NetworkCoordinationCostPayload{
		DeliveredCount:        thread.DeliveredCount,
		PromptSizeBytes:       thread.PromptSizeBytes,
		EstimatedPromptTokens: thread.EstimatedPromptTokens,
	}
}

// NetworkThreadSessionCostPayloadsFromStore converts stored thread session cost rows into public payloads.
func NetworkThreadSessionCostPayloadsFromStore(
	stats []store.NetworkThreadSessionTokenStats,
) []contract.NetworkThreadSessionCostPayload {
	payload := make([]contract.NetworkThreadSessionCostPayload, 0, len(stats))
	for _, stat := range stats {
		payload = append(payload, NetworkThreadSessionCostPayloadFromStore(stat))
	}
	return payload
}

// NetworkThreadSessionCostPayloadFromStore converts one stored thread session cost row into a public payload.
func NetworkThreadSessionCostPayloadFromStore(
	stat store.NetworkThreadSessionTokenStats,
) contract.NetworkThreadSessionCostPayload {
	return contract.NetworkThreadSessionCostPayload{
		SessionID:             strings.TrimSpace(stat.SessionID),
		DeliveredCount:        stat.DeliveredCount,
		PromptSizeBytes:       stat.PromptSizeBytes,
		EstimatedPromptTokens: stat.EstimatedPromptTokens,
		FirstDeliveredAt:      cloneTimePtr(&stat.FirstDeliveredAt),
		LastDeliveredAt:       cloneTimePtr(&stat.LastDeliveredAt),
	}
}

// NetworkDirectRoomPayloadsFromStore converts stored direct rooms into public payloads.
func NetworkDirectRoomPayloadsFromStore(
	directs []store.NetworkDirectRoomSummary,
) []contract.NetworkDirectRoomPayload {
	payload := make([]contract.NetworkDirectRoomPayload, 0, len(directs))
	for _, direct := range directs {
		payload = append(payload, NetworkDirectRoomPayloadFromStore(direct))
	}
	return payload
}

// NetworkDirectRoomPayloadFromStore converts one stored direct-room summary into a public payload.
func NetworkDirectRoomPayloadFromStore(direct store.NetworkDirectRoomSummary) contract.NetworkDirectRoomPayload {
	return contract.NetworkDirectRoomPayload{
		WorkspaceID:        strings.TrimSpace(direct.WorkspaceID),
		Channel:            strings.TrimSpace(direct.Channel),
		DirectID:           strings.TrimSpace(direct.DirectID),
		SessionA:           strings.TrimSpace(direct.SessionA),
		SessionB:           strings.TrimSpace(direct.SessionB),
		OpenedAt:           cloneTimePtr(&direct.OpenedAt),
		LastActivityAt:     cloneTimePtr(&direct.LastActivityAt),
		MessageCount:       direct.MessageCount,
		OpenWorkCount:      direct.OpenWorkCount,
		LastMessagePreview: strings.TrimSpace(direct.LastMessagePreview),
	}
}

// NetworkConversationMessagePayloadsFromStore converts stored messages into public payloads.
func NetworkConversationMessagePayloadsFromStore(
	messages []store.NetworkConversationMessage,
) []contract.NetworkConversationMessagePayload {
	payload := make([]contract.NetworkConversationMessagePayload, 0, len(messages))
	for _, message := range messages {
		payload = append(payload, NetworkConversationMessagePayloadFromStore(message))
	}
	return payload
}

// NetworkConversationMessagePayloadFromStore converts one stored message into a public payload.
func NetworkConversationMessagePayloadFromStore(
	message store.NetworkConversationMessage,
) contract.NetworkConversationMessagePayload {
	return contract.NetworkConversationMessagePayload{
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
		Mentions:    cloneTrimmedStrings(message.Mentions),
		SessionID:   strings.TrimSpace(message.SessionID),
		WorkID:      strings.TrimSpace(message.WorkID),
		ReplyTo:     strings.TrimSpace(message.ReplyTo),
		TraceID:     strings.TrimSpace(message.TraceID),
		CausationID: strings.TrimSpace(message.CausationID),
		Intent:      strings.TrimSpace(message.Intent),
		Text:        strings.TrimSpace(message.Text),
		PreviewText: networkMessagePreview(message),
		SizeBytes:   message.SizeBytes,
		Body:        cloneRawMessage(message.Body),
		Timestamp:   message.Timestamp.UTC(),
	}
}

// NetworkSubscriptionPayloadsFromStore converts delivery preferences into public payloads.
func NetworkSubscriptionPayloadsFromStore(
	subscriptions []store.NetworkSubscriptionEntry,
) []contract.NetworkSubscriptionPayload {
	payload := make([]contract.NetworkSubscriptionPayload, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		payload = append(payload, NetworkSubscriptionPayloadFromStore(subscription))
	}
	return payload
}

// NetworkSubscriptionPayloadFromStore converts one delivery preference into a public payload.
func NetworkSubscriptionPayloadFromStore(
	subscription store.NetworkSubscriptionEntry,
) contract.NetworkSubscriptionPayload {
	return contract.NetworkSubscriptionPayload{
		WorkspaceID: strings.TrimSpace(subscription.WorkspaceID),
		Channel:     strings.TrimSpace(subscription.Channel),
		ThreadID:    strings.TrimSpace(subscription.ThreadID),
		SessionID:   strings.TrimSpace(subscription.SessionID),
		Mode:        strings.TrimSpace(subscription.Mode),
		CreatedAt:   cloneTimePtr(&subscription.CreatedAt),
		UpdatedAt:   cloneTimePtr(&subscription.UpdatedAt),
	}
}

// NetworkTaskThreadOriginPayloadsFromStore converts promoted-task origin links into public payloads.
func NetworkTaskThreadOriginPayloadsFromStore(
	origins []store.NetworkTaskThreadOrigin,
) []contract.NetworkTaskThreadOriginPayload {
	payload := make([]contract.NetworkTaskThreadOriginPayload, 0, len(origins))
	for _, origin := range origins {
		payload = append(payload, NetworkTaskThreadOriginPayloadFromStore(origin))
	}
	return payload
}

// NetworkTaskThreadOriginPayloadFromStore converts one promoted-task origin link into a public payload.
func NetworkTaskThreadOriginPayloadFromStore(
	origin store.NetworkTaskThreadOrigin,
) contract.NetworkTaskThreadOriginPayload {
	return contract.NetworkTaskThreadOriginPayload{
		TaskID:           strings.TrimSpace(origin.TaskID),
		WorkspaceID:      strings.TrimSpace(origin.WorkspaceID),
		Channel:          strings.TrimSpace(origin.Channel),
		ThreadID:         strings.TrimSpace(origin.ThreadID),
		OriginMessageID:  strings.TrimSpace(origin.OriginMessageID),
		Digest:           strings.TrimSpace(origin.Digest),
		SourceMessageIDs: cloneTrimmedStrings(origin.SourceMessageIDs),
		CreatedAt:        cloneTimePtr(&origin.CreatedAt),
		UpdatedAt:        cloneTimePtr(&origin.UpdatedAt),
	}
}

// NetworkWorkPayloadFromStore converts one network work row into a public payload.
func NetworkWorkPayloadFromStore(work store.NetworkWorkEntry) contract.NetworkWorkPayload {
	return contract.NetworkWorkPayload{
		WorkID:          strings.TrimSpace(work.WorkID),
		WorkspaceID:     strings.TrimSpace(work.WorkspaceID),
		Channel:         strings.TrimSpace(work.Channel),
		Surface:         strings.TrimSpace(work.Surface),
		ThreadID:        strings.TrimSpace(work.ThreadID),
		DirectID:        strings.TrimSpace(work.DirectID),
		OpenedSessionID: strings.TrimSpace(work.OpenedBySessionID),
		TargetSessionID: strings.TrimSpace(work.TargetSessionID),
		State:           strings.TrimSpace(work.State),
		OpenedAt:        cloneTimePtr(&work.OpenedAt),
		LastActivityAt:  cloneTimePtr(&work.LastActivityAt),
		TerminalAt:      cloneTimePtr(work.TerminalAt),
	}
}
