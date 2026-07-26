package transcript

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
)

// Projection is a complete deterministic materialization of canonical events.
type Projection struct {
	Segments       []ProjectionSegment
	EventEntryKeys map[string]string
	ToolRoutes     map[string]string
	State          ProjectionState
}

// BuildProjection routes and reduces a complete event log with the canonical projector.
func BuildProjection(events []store.SessionEvent, generation int64) (Projection, error) {
	state := ProjectionState{
		Version:    ProjectionVersion,
		Generation: generation,
	}
	projector, err := NewProjector(state, nil)
	if err != nil {
		return Projection{}, err
	}

	groups := make(map[string][]store.SessionEvent)
	order := make([]string, 0, len(events))
	eventKeys := make(map[string]string, len(events))
	ctx := context.Background()
	for _, stored := range sortedTranscriptEvents(events) {
		assignment, assignErr := projector.Assign(ctx, stored)
		if assignErr != nil {
			return Projection{}, assignErr
		}
		if _, ok := groups[assignment.Entry.Key]; !ok {
			order = append(order, assignment.Entry.Key)
		}
		groups[assignment.Entry.Key] = append(groups[assignment.Entry.Key], stored)
		eventKeys[stored.ID] = assignment.Entry.Key
	}

	usedMessageIDs := make(map[string]struct{})
	segments := make([]ProjectionSegment, 0, len(order))
	for _, key := range order {
		identity, ok := projector.Identity(key)
		if !ok {
			return Projection{}, fmt.Errorf("%w: missing identity for %q", ErrProjectionCorrupt, key)
		}
		identity.MessageID = strings.TrimSpace(identity.BaseMessageID)
		entry, projectErr := ProjectAssignedEntry(groups[key], identity)
		if projectErr != nil {
			return Projection{}, projectErr
		}
		if entry != nil {
			identity.MessageID = uniqueUIMessageID(identity.BaseMessageID, usedMessageIDs)
			usedMessageIDs[identity.MessageID] = struct{}{}
			entry, projectErr = ProjectAssignedEntry(groups[key], identity)
			if projectErr != nil {
				return Projection{}, projectErr
			}
		}
		segments = append(segments, ProjectionSegment{
			Identity: identity,
			Events:   append([]store.SessionEvent(nil), groups[key]...),
			Entry:    entry,
		})
	}
	return Projection{
		Segments:       segments,
		EventEntryKeys: eventKeys,
		ToolRoutes:     projector.ToolRoutes(),
		State:          projector.State(),
	}, nil
}

// ProjectAssignedEntry reduces only the raw events assigned to one stable entry.
func ProjectAssignedEntry(events []store.SessionEvent, identity EntryIdentity) (*Entry, error) {
	if len(events) == 0 {
		return nil, nil
	}
	sorted := sortedTranscriptEvents(events)
	messageID := fallbackMessageID(identity.MessageID, identity.BaseMessageID, identity.LogicalID)

	switch identity.Kind {
	case EntryKindUser:
		message := inputUIMessage(decodeStoredEvent(sorted[0]), UIRoleUser)
		return projectedInputEntry(message, identity), nil
	case EntryKindSystem:
		message := inputUIMessage(decodeStoredEvent(sorted[0]), UIRoleSystem)
		return projectedInputEntry(message, identity), nil
	case EntryKindMarker:
		decoded := decodeStoredEvent(sorted[0])
		markerText := transcriptMarkerText(decoded.parsed)
		if markerText == "" {
			return nil, nil
		}
		message := runtimeMarkerUIMessage(decoded)
		message.ID = messageID
		marker := decoded.parsed.Marker.Normalize()
		return &Entry{
			Message:       message,
			StartSequence: identity.StartSequence,
			Sequence:      identity.UpdatedSequence,
			EventType:     decoded.parsed.Type,
			Marker:        &marker,
		}, nil
	case EntryKindAssistant:
		toolLifecycles := make(map[string]*uiToolLifecycle)
		builder := newUIMessageBuilder(messageID, identity.LogicalID, UIRoleAssistant, toolLifecycles)
		for _, stored := range sorted {
			applyDecodedEvent(builder, decodeStoredEvent(stored))
		}
		message := builder.build(identity.Complete || builder.finished)
		if message == nil {
			return nil, nil
		}
		return &Entry{
			Message:       *message,
			StartSequence: identity.StartSequence,
			Sequence:      identity.UpdatedSequence,
		}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported entry kind %q", ErrProjectionCorrupt, identity.Kind)
	}
}

func projectedInputEntry(message *UIMessage, identity EntryIdentity) *Entry {
	if message == nil {
		return nil
	}
	message.ID = fallbackMessageID(identity.MessageID, identity.BaseMessageID, message.ID)
	return &Entry{
		Message:       *message,
		StartSequence: identity.StartSequence,
		Sequence:      identity.UpdatedSequence,
	}
}
