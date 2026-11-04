package session

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

// emitInterruptSupersededSteers projects committed supersessions without delaying cancellation on failure.
func (m *Manager) emitInterruptSupersededSteers(
	ctx context.Context,
	session *Session,
	replacement *store.SessionInputQueueEntry,
) {
	for _, id := range replacement.SupersededIDs {
		entry, err := m.inputQueue.Get(ctx, session.ID, id)
		if err == nil {
			err = m.recordSteerSupersession(ctx, session, &entry)
		}
		if err != nil {
			m.sessionLogger(session).Warn("session: project steering supersession failed", "entry_id", id, "error", err)
		}
	}
}

// reconcileSteerSupersessions repairs the crash window between the queue commit and transcript append.
func (m *Manager) reconcileSteerSupersessions(ctx context.Context, session *Session) error {
	afterID := ""
	for {
		entries, err := m.inputQueue.ListSupersededSteers(ctx, session.ID, afterID)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			return nil
		}
		for i := range entries {
			if err := m.recordSteerSupersession(ctx, session, &entries[i]); err != nil {
				return err
			}
		}
		afterID = entries[len(entries)-1].ID
	}
}

// recordSteerSupersession uses a stable ledger identity so replay never adds a second marker.
func (m *Manager) recordSteerSupersession(
	ctx context.Context,
	session *Session,
	entry *store.SessionInputQueueEntry,
) error {
	if entry.SupersededBy == "" {
		return nil
	}
	eventID := "steer_superseded_" + entry.ID
	recorder := session.recorderHandle()
	if recorder == nil {
		return errors.New("session: supersession recorder is unavailable")
	}
	// A confirmed injection is already represented by its authored input event.
	if entry.EventID != "" {
		delivered, err := recorder.Query(ctx, store.EventQuery{ID: entry.EventID, Limit: 1})
		if err != nil || len(delivered) > 0 {
			return err
		}
	}
	existing, err := recorder.Query(ctx, store.EventQuery{ID: eventID, Limit: 1})
	if err != nil || len(existing) > 0 {
		return err
	}
	evidence := map[string]any{promptEvidenceQueueEntryIDKey: entry.ID, "replacement_entry_id": entry.SupersededBy}
	addSteerAuthoredEvidence(evidence, entry)
	return m.recordTranscriptMarker(ctx, session, entry.TargetTurnID, transcriptMarkerInput{
		EventID: eventID, Kind: transcript.MarkerPromptSuperseded,
		Summary: "Undelivered steering replaced by an explicit interrupt.", Evidence: evidence,
	})
}
