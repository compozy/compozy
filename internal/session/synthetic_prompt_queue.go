package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/session/inputqueue"
	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) enqueueDurableSyntheticPrompt(
	ctx context.Context,
	session *Session,
	req promptRequest,
) (store.SessionInputQueueEntry, error) {
	generation, err := m.currentInputGeneration(ctx, session.ID)
	if err != nil {
		return store.SessionInputQueueEntry{}, err
	}
	metadata, err := json.Marshal(req.meta.Synthetic)
	if err != nil {
		return store.SessionInputQueueEntry{}, fmt.Errorf("session: encode synthetic input: %w", err)
	}
	return m.inputQueue.EnqueueSynthetic(ctx, inputqueue.InputRequest{
		SessionID: session.ID, Text: req.message, TargetTurnID: session.CurrentTurnID(), Generation: generation,
	}, req.turnID, &store.SessionInputSyntheticPrompt{RunID: req.runID, Metadata: metadata}, req.meta.Synthetic.TaskRunID)
}

// This observer owns only delivery completion. Durable rows and the common pump
// own admission and execution, including when this caller or daemon disappears.
func (m *Manager) waitSyntheticPromptPersistence(session *Session, admitted *store.SessionInputQueueEntry) {
	ctx := m.fallbackLifecycleContext()
	ticker := time.NewTicker(promptDeliveryCatchUpInterval)
	defer ticker.Stop()
	for {
		entry, err := m.inputQueue.Get(ctx, session.ID, admitted.ID)
		if err != nil {
			m.sessionLogger(session).
				WarnContext(ctx, "session: observe synthetic input failed", "entry_id", admitted.ID, "error", err)
			return
		}
		switch entry.Status {
		case store.SessionInputQueueStatusFailed, store.SessionInputQueueStatusCanceled:
			return
		case store.SessionInputQueueStatusSent:
			if session.CurrentTurnID() != admitted.TurnID {
				return
			}
		}
		if session.Info().State != StateActive {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
