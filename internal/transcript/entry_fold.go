package transcript

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// ErrFoldOutOfOrder reports an event that cannot extend an incremental fold.
var ErrFoldOutOfOrder = errors.New("transcript: fold events must extend the applied sequence")

// EntryFold incrementally reduces the events assigned to one assistant entry.
// Appending events in sequence order yields the same entry as
// ProjectAssignedEntry over the full assigned event list, so a long turn costs
// one decode per event instead of one replay of the whole turn per append.
type EntryFold struct {
	key          string
	logicalID    string
	messageID    string
	builder      *uiMessageBuilder
	lastSequence int64
	applied      int
}

// NewEntryFold starts an empty fold for an assistant entry identity.
func NewEntryFold(identity EntryIdentity) (*EntryFold, error) {
	if identity.Kind != EntryKindAssistant {
		return nil, fmt.Errorf("transcript: fold requires an assistant entry, got %q", identity.Kind)
	}
	messageID := foldMessageID(identity)
	return &EntryFold{
		key:       strings.TrimSpace(identity.Key),
		logicalID: identity.LogicalID,
		messageID: messageID,
		builder: newUIMessageBuilder(
			messageID,
			identity.LogicalID,
			UIRoleAssistant,
			make(map[string]*uiToolLifecycle),
		),
	}, nil
}

// Matches reports whether the fold was started for the same entry and message identity.
func (f *EntryFold) Matches(identity EntryIdentity) bool {
	return f != nil && identity.Kind == EntryKindAssistant &&
		f.key == strings.TrimSpace(identity.Key) &&
		f.logicalID == identity.LogicalID &&
		f.messageID == foldMessageID(identity)
}

// LastSequence returns the highest applied event sequence.
func (f *EntryFold) LastSequence() int64 {
	return f.lastSequence
}

// Applied returns the number of applied events.
func (f *EntryFold) Applied() int {
	return f.applied
}

// Append applies events whose sequences extend the fold in ascending order.
// It rejects the whole batch before mutating the fold when any event would
// reorder the reduction.
func (f *EntryFold) Append(events []store.SessionEvent) error {
	sorted := sortedTranscriptEvents(events)
	last := f.lastSequence
	for _, stored := range sorted {
		if stored.Sequence <= last {
			return fmt.Errorf("%w: sequence %d after %d", ErrFoldOutOfOrder, stored.Sequence, last)
		}
		last = stored.Sequence
	}
	f.applySorted(sorted)
	return nil
}

func (f *EntryFold) applySorted(sorted []store.SessionEvent) {
	for _, stored := range sorted {
		applyDecodedEvent(f.builder, decodeStoredEvent(stored))
		f.lastSequence = max(f.lastSequence, stored.Sequence)
	}
	f.applied += len(sorted)
}

// Entry materializes the folded entry without mutating the fold.
func (f *EntryFold) Entry(identity EntryIdentity) *Entry {
	message := f.builder.build(identity.Complete || f.builder.finished)
	if message == nil {
		return nil
	}
	return &Entry{
		Message:       *message,
		StartSequence: identity.StartSequence,
		Sequence:      identity.UpdatedSequence,
	}
}

func foldMessageID(identity EntryIdentity) string {
	return fallbackMessageID(identity.MessageID, identity.BaseMessageID, identity.LogicalID)
}
