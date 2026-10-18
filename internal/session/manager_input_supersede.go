package session

import (
	"context"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

const queueEntrySupersededReason = "superseded"

// settleSupersededQueuedInputs publishes the removal of every older message the new entry replaced
// in its admission transaction, and releases reply watches waiting on them with a dropped outcome.
func (m *Manager) settleSupersededQueuedInputs(
	ctx context.Context,
	session *Session,
	replacement *store.SessionInputQueueEntry,
) {
	for _, id := range replacement.SupersededIDs {
		older, err := m.inputQueue.Get(ctx, session.ID, id)
		if err != nil {
			m.sessionLogger(session).Warn("session: read superseded queued input failed", "entry_id", id, "error", err)
			continue
		}
		m.settleSupersededQueuedInput(ctx, session, replacement, &older)
	}
}

// settleSupersededQueuedInput publishes the removal of one replaced queued message and releases the
// reply watch waiting on it.
func (m *Manager) settleSupersededQueuedInput(
	ctx context.Context,
	session *Session,
	replacement *store.SessionInputQueueEntry,
	older *store.SessionInputQueueEntry,
) {
	evidence := queueEntryEvidence(older.ID, older.SessionGeneration, older.Status, older.Mode, 0)
	evidence["reason"] = queueEntrySupersededReason
	evidence["replacement_entry_id"] = replacement.ID
	evidence["message_id"] = older.MessageID
	evidence["replacement_message_id"] = replacement.MessageID
	m.emitTranscriptMarker(
		ctx,
		session,
		session.CurrentTurnID(),
		transcript.MarkerPromptDropped,
		"Queued message replaced by a newer message from the same session.",
		evidence,
	)
	if service := m.ReplyWatches(); service != nil {
		service.OnQueueEntryTerminal(ctx, older)
	}
}
