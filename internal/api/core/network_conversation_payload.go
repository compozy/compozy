package core

import (
	"database/sql"

	"fmt"

	"sort"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network"

	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

// NetworkConversationMessagePayloadFromEntry converts one persisted timeline row into the shared payload.
func NetworkConversationMessagePayloadFromEntry(
	entry store.NetworkMessageEntry,
	sessionsByID map[string]*session.Info,
	peersByID map[string]network.PeerInfo,
) contract.NetworkConversationMessagePayload {
	return NetworkConversationMessagePayloadFromView(networkTimelineMessageView{entry: entry}, sessionsByID, peersByID)
}

func NetworkConversationMessagePayloadFromView(
	view networkTimelineMessageView,
	sessionsByID map[string]*session.Info,
	peersByID map[string]network.PeerInfo,
) contract.NetworkConversationMessagePayload {
	entry := view.entry
	storedSessionID := strings.TrimSpace(entry.SessionID)
	displayName := strings.TrimSpace(entry.PeerFrom)
	local := strings.TrimSpace(entry.Direction) == network.AuditDirectionSent
	payloadSessionID := ""

	if peer, ok := peersByID[strings.TrimSpace(entry.PeerFrom)]; ok {
		displayName = networkPeerDisplayName(peer, sessionsByID)
	}

	if local {
		payloadSessionID = storedSessionID
	}

	if local && payloadSessionID != "" {
		if info, ok := sessionsByID[payloadSessionID]; ok && info != nil {
			if value := strings.TrimSpace(info.Name); value != "" {
				displayName = value
			} else if value := strings.TrimSpace(info.AgentName); value != "" {
				displayName = value
			}
		}
	}

	return contract.NetworkConversationMessagePayload{
		MessageID:   strings.TrimSpace(entry.MessageID),
		WorkspaceID: strings.TrimSpace(entry.WorkspaceID),
		Channel:     strings.TrimSpace(entry.Channel),
		Surface:     strings.TrimSpace(entry.Surface),
		ThreadID:    strings.TrimSpace(entry.ThreadID),
		DirectID:    strings.TrimSpace(entry.DirectID),
		Kind:        strings.TrimSpace(entry.Kind),
		Direction:   strings.TrimSpace(entry.Direction),
		PeerFrom:    strings.TrimSpace(entry.PeerFrom),
		PeerTo:      strings.TrimSpace(entry.PeerTo),
		Mentions:    cloneTrimmedStrings(entry.Mentions),
		DisplayName: displayName,
		SessionID:   payloadSessionID,
		Local:       local,
		WorkID:      strings.TrimSpace(entry.WorkID),
		ReplyTo:     strings.TrimSpace(entry.ReplyTo),
		TraceID:     strings.TrimSpace(entry.TraceID),
		CausationID: strings.TrimSpace(entry.CausationID),
		Intent:      strings.TrimSpace(entry.Intent),
		Text:        strings.TrimSpace(entry.Text),
		PreviewText: networkMessagePreview(entry),
		SizeBytes:   entry.SizeBytes,
		Body:        cloneRawMessage(entry.Body),
		Timestamp:   entry.Timestamp.UTC(),
	}
}

func summarizeNetworkMessageHistory(messages []store.NetworkMessageEntry) networkMessageHistorySummary {
	summary := networkMessageHistorySummary{
		conversation:     make([]store.NetworkMessageEntry, 0, len(messages)),
		presenceMessages: make([]store.NetworkMessageEntry, 0),
	}
	if len(messages) == 0 {
		return summary
	}

	participants := make(map[string]struct{})
	for _, message := range messages {
		recordNetworkMessageParticipants(func(peerID string) {
			recordHistoricalParticipant(participants, peerID)
		}, message)
		if isPresenceMessage(message) {
			summary.presenceCount++
			summary.lastPresenceAt = laterTimePtr(summary.lastPresenceAt, message.Timestamp)
			presenceMessage := cloneNetworkMessageEntry(message)
			presenceMessage.PreviewText = networkMessagePreview(presenceMessage)
			summary.presenceMessages = append(summary.presenceMessages, presenceMessage)
			continue
		}

		summary.conversation = append(summary.conversation, cloneNetworkMessageEntry(message))
	}
	summary.historicalParticipantCount = len(participants)
	return summary
}

func networkTimelinePayloads(
	messages []store.NetworkMessageEntry,
	sessionByID map[string]*session.Info,
	peerByID map[string]network.PeerInfo,
	query store.NetworkMessageQuery,
) ([]contract.NetworkConversationMessagePayload, error) {
	history := summarizeNetworkMessageHistory(messages)
	views, err := paginateNetworkTimelineViews(history.timelineViews(query.IncludePresence), query)
	if err != nil {
		return nil, err
	}
	payload := make([]contract.NetworkConversationMessagePayload, 0, len(views))
	for _, view := range views {
		payload = append(payload, NetworkConversationMessagePayloadFromView(view, sessionByID, peerByID))
	}
	return payload, nil
}

func (s networkMessageHistorySummary) timelineViews(includePresence bool) []networkTimelineMessageView {
	if !includePresence {
		views := make([]networkTimelineMessageView, 0, len(s.conversation))
		for _, entry := range s.conversation {
			views = append(views, networkTimelineMessageView{entry: entry})
		}
		return views
	}

	views := make([]networkTimelineMessageView, 0, len(s.conversation)+len(s.presenceMessages))
	for _, entry := range s.conversation {
		views = append(views, networkTimelineMessageView{entry: entry})
	}
	for _, entry := range s.presenceMessages {
		views = append(views, networkTimelineMessageView{entry: entry})
	}
	sort.SliceStable(views, func(i int, j int) bool {
		left := views[i].entry.Timestamp.UTC()
		right := views[j].entry.Timestamp.UTC()
		if !left.Equal(right) {
			return left.Before(right)
		}
		return strings.TrimSpace(views[i].entry.MessageID) < strings.TrimSpace(views[j].entry.MessageID)
	})
	return views
}

func cloneNetworkMessageEntry(entry store.NetworkMessageEntry) store.NetworkMessageEntry {
	return store.NetworkMessageEntry{
		MessageID:   strings.TrimSpace(entry.MessageID),
		WorkspaceID: strings.TrimSpace(entry.WorkspaceID),
		SessionID:   strings.TrimSpace(entry.SessionID),
		Channel:     strings.TrimSpace(entry.Channel),
		Surface:     strings.TrimSpace(entry.Surface),
		ThreadID:    strings.TrimSpace(entry.ThreadID),
		DirectID:    strings.TrimSpace(entry.DirectID),
		Direction:   strings.TrimSpace(entry.Direction),
		PeerFrom:    strings.TrimSpace(entry.PeerFrom),
		PeerTo:      strings.TrimSpace(entry.PeerTo),
		Kind:        strings.TrimSpace(entry.Kind),
		WorkID:      strings.TrimSpace(entry.WorkID),
		ReplyTo:     strings.TrimSpace(entry.ReplyTo),
		TraceID:     strings.TrimSpace(entry.TraceID),
		CausationID: strings.TrimSpace(entry.CausationID),
		Intent:      strings.TrimSpace(entry.Intent),
		Text:        entry.Text,
		PreviewText: strings.TrimSpace(entry.PreviewText),
		Mentions:    cloneTrimmedStrings(entry.Mentions),
		Body:        cloneRawMessage(entry.Body),
		SizeBytes:   entry.SizeBytes,
		Timestamp:   entry.Timestamp.UTC(),
	}
}

func isPresenceMessage(entry store.NetworkMessageEntry) bool {
	return strings.TrimSpace(entry.Kind) == string(network.KindGreet)
}

func recordHistoricalParticipant(target map[string]struct{}, peerID string) {
	trimmed := strings.TrimSpace(peerID)
	if trimmed == "" {
		return
	}
	target[trimmed] = struct{}{}
}

func filterPeerTimelineMessages(messages []store.NetworkMessageEntry) []store.NetworkMessageEntry {
	filtered := make([]store.NetworkMessageEntry, 0, len(messages))
	for _, message := range messages {
		if isPresenceMessage(message) || isDirectedChannelMessage(message) {
			filtered = append(filtered, message)
		}
	}
	return filtered
}

func filterVisiblePublicChannelMessages(
	messages []store.NetworkMessageEntry,
	includePresence bool,
) []store.NetworkMessageEntry {
	if includePresence {
		return filterPublicChannelTimelineMessages(messages)
	}

	filtered := make([]store.NetworkMessageEntry, 0, len(messages))
	for _, message := range messages {
		if isPublicConversationMessage(message) {
			filtered = append(filtered, message)
		}
	}
	return filtered
}

func filterPublicChannelTimelineMessages(messages []store.NetworkMessageEntry) []store.NetworkMessageEntry {
	filtered := make([]store.NetworkMessageEntry, 0, len(messages))
	for _, message := range messages {
		if isPublicChannelTimelineMessage(message) {
			filtered = append(filtered, message)
		}
	}
	return filtered
}

func isPublicChannelTimelineMessage(message store.NetworkMessageEntry) bool {
	return isPresenceMessage(message) || !isDirectedChannelMessage(message)
}

func isPublicConversationMessage(message store.NetworkMessageEntry) bool {
	return !isPresenceMessage(message) && !isDirectedChannelMessage(message)
}

func isDirectedChannelMessage(message store.NetworkMessageEntry) bool {
	if strings.TrimSpace(message.PeerTo) != "" {
		return true
	}
	if strings.TrimSpace(message.DirectID) != "" {
		return true
	}
	return strings.TrimSpace(message.Surface) == string(network.SurfaceDirect)
}

func filterVisiblePeerMessages(messages []store.NetworkMessageEntry, includePresence bool) []store.NetworkMessageEntry {
	if includePresence {
		return filterPeerTimelineMessages(messages)
	}

	filtered := make([]store.NetworkMessageEntry, 0, len(messages))
	for _, message := range messages {
		if isDirectedChannelMessage(message) {
			filtered = append(filtered, message)
		}
	}
	return filtered
}

func paginateNetworkTimelineViews(
	views []networkTimelineMessageView,
	query store.NetworkMessageQuery,
) ([]networkTimelineMessageView, error) {
	paginated := views
	if before := strings.TrimSpace(query.BeforeMessageID); before != "" {
		index := indexNetworkTimelineViewByMessageID(paginated, before)
		if index < 0 {
			return nil, fmt.Errorf("network timeline before cursor: %w", sql.ErrNoRows)
		}
		paginated = paginated[:index]
	}
	if after := strings.TrimSpace(query.AfterMessageID); after != "" {
		index := indexNetworkTimelineViewByMessageID(paginated, after)
		if index < 0 {
			return nil, fmt.Errorf("network timeline after cursor: %w", sql.ErrNoRows)
		}
		paginated = paginated[index+1:]
	}
	if query.Limit <= 0 || len(paginated) <= query.Limit {
		return paginated, nil
	}
	if strings.TrimSpace(query.BeforeMessageID) != "" {
		return paginated[len(paginated)-query.Limit:], nil
	}
	if strings.TrimSpace(query.AfterMessageID) != "" {
		return paginated[:query.Limit], nil
	}
	return paginated[len(paginated)-query.Limit:], nil
}
