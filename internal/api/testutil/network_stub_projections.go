package testutil

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
)

func (s StubNetworkStore) deriveNetworkChannelProjections(
	ctx context.Context,
	query store.NetworkChannelProjectionQuery,
) ([]store.NetworkChannelProjection, error) {
	if s.ListNetworkMessagesFn == nil {
		return nil, nil
	}
	messages, err := s.ListNetworkMessagesFn(ctx, store.NetworkMessageQuery{
		WorkspaceID: query.WorkspaceID,
		Channel:     query.Channel,
	})
	if err != nil {
		return nil, err
	}
	type projectionState struct {
		projection   store.NetworkChannelProjection
		lastMessage  string
		participants map[string]struct{}
	}
	states := make(map[string]*projectionState)
	for _, message := range messages {
		messageWorkspaceID := strings.TrimSpace(message.WorkspaceID)
		if messageWorkspaceID != "" && messageWorkspaceID != strings.TrimSpace(query.WorkspaceID) {
			continue
		}
		if query.Channel != "" && strings.TrimSpace(message.Channel) != strings.TrimSpace(query.Channel) {
			continue
		}
		channel := strings.TrimSpace(message.Channel)
		state := states[channel]
		if state == nil {
			state = &projectionState{
				projection: store.NetworkChannelProjection{
					WorkspaceID: strings.TrimSpace(query.WorkspaceID),
					Channel:     channel,
				},
				participants: make(map[string]struct{}),
			}
			states[channel] = state
		}
		for _, peerID := range append([]string{message.PeerFrom, message.PeerTo}, message.Mentions...) {
			if trimmed := strings.TrimSpace(peerID); trimmed != "" {
				state.participants[trimmed] = struct{}{}
			}
		}
		switch {
		case strings.TrimSpace(message.Kind) == store.NetworkKindGreet:
			state.projection.PresenceCount++
			state.projection.LastPresenceAt = laterStubTime(state.projection.LastPresenceAt, message.Timestamp)
		case stubMessageIsPublic(message):
			state.projection.MessageCount++
			if stubMessageIsLater(state.projection.LastActivityAt, state.lastMessage, message) {
				state.projection.LastActivityAt = laterStubTime(nil, message.Timestamp)
				state.lastMessage = strings.TrimSpace(message.MessageID)
				state.projection.LastMessagePreview = stubMessagePreview(message)
			}
		}
	}
	projections := make([]store.NetworkChannelProjection, 0, len(states))
	for _, state := range states {
		state.projection.HistoricalParticipantCount = len(state.participants)
		projections = append(projections, state.projection)
	}
	sort.Slice(projections, func(i int, j int) bool {
		return projections[i].Channel < projections[j].Channel
	})
	if query.Limit > 0 && len(projections) > query.Limit {
		projections = projections[:query.Limit]
	}
	return projections, nil
}

func (s StubNetworkStore) deriveNetworkChannelKindCounts(
	ctx context.Context,
	ref store.NetworkChannelRef,
) ([]store.NetworkChannelKindCount, error) {
	if s.ListNetworkMessagesFn == nil {
		return nil, nil
	}
	messages, err := s.ListNetworkMessagesFn(ctx, store.NetworkMessageQuery{
		WorkspaceID: ref.WorkspaceID,
		Channel:     ref.Channel,
	})
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int)
	for _, message := range messages {
		if stubMessageIsPublic(message) {
			counts[strings.TrimSpace(message.Kind)]++
		}
	}
	result := make([]store.NetworkChannelKindCount, 0, len(counts))
	for kind, count := range counts {
		result = append(result, store.NetworkChannelKindCount{
			WorkspaceID: strings.TrimSpace(ref.WorkspaceID),
			Channel:     strings.TrimSpace(ref.Channel),
			Kind:        kind,
			Count:       count,
		})
	}
	return result, nil
}

func stubMessageIsPublic(message store.NetworkMessageEntry) bool {
	return strings.TrimSpace(message.Kind) != store.NetworkKindGreet &&
		strings.TrimSpace(message.PeerTo) == "" &&
		strings.TrimSpace(message.DirectID) == "" &&
		strings.TrimSpace(message.Surface) != store.NetworkSurfaceDirect
}

func stubMessageIsLater(current *time.Time, currentID string, message store.NetworkMessageEntry) bool {
	return current == nil || message.Timestamp.After(current.UTC()) ||
		(message.Timestamp.Equal(current.UTC()) && strings.TrimSpace(message.MessageID) > currentID)
}

func laterStubTime(current *time.Time, candidate time.Time) *time.Time {
	if current != nil && !candidate.After(current.UTC()) {
		return current
	}
	value := candidate.UTC()
	return &value
}

func stubMessagePreview(message store.NetworkMessageEntry) string {
	if preview := strings.TrimSpace(message.PreviewText); preview != "" {
		return preview
	}
	return strings.TrimSpace(message.Text)
}

func filterStubNetworkMessages(
	messages []store.NetworkMessageEntry,
	query store.NetworkMessageQuery,
) []store.NetworkMessageEntry {
	if !query.PublicOnly && !query.DirectedOnly {
		return messages
	}
	filtered := make([]store.NetworkMessageEntry, 0, len(messages))
	for _, message := range messages {
		presence := strings.TrimSpace(message.Kind) == store.NetworkKindGreet
		directed := !stubMessageIsPublic(message) && !presence
		visible := false
		switch {
		case query.PublicOnly:
			visible = (!presence && !directed) || (query.IncludePresence && presence)
		case query.DirectedOnly:
			visible = directed || (query.IncludePresence && presence)
		}
		if visible {
			filtered = append(filtered, message)
		}
	}
	return filtered
}
