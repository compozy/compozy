package session

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/session/inputqueue"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

func (m *Manager) enqueueBusyPrompt(
	ctx context.Context,
	session *Session,
	req promptRequest,
) (SendPromptResult, error) {
	if m.inputQueue == nil {
		return SendPromptResult{}, ErrPromptInProgress
	}
	generation, err := m.currentInputGeneration(ctx, session.ID)
	if err != nil {
		return SendPromptResult{}, err
	}
	entry, position, err := m.inputQueue.Enqueue(
		ctx,
		inputqueue.InputRequest{
			SessionID:        session.ID,
			Text:             req.message,
			Generation:       generation,
			Runtime:          storeRuntimeSelection(req.runtime),
			SkillInvocations: req.skillInvocations,
			Attachments:      storeAttachments(req.attachments),
		},
	)
	if err != nil {
		if errors.Is(err, store.ErrSessionInputQueueFull) {
			m.emitTranscriptMarker(
				ctx,
				session,
				session.CurrentTurnID(),
				transcript.MarkerPromptDropped,
				"Queued input rejected because the session input queue is full.",
				map[string]any{promptEvidenceQueueGenerationKey: generation, "queue_cap": m.busyInput.QueueCap},
			)
		}
		return SendPromptResult{}, err
	}
	m.emitTranscriptMarker(
		ctx,
		session,
		session.CurrentTurnID(),
		transcript.MarkerPromptQueued,
		"Input queued while the session is busy.",
		queueEntryEvidence(entry.ID, entry.SessionGeneration, entry.Status, entry.Mode, position),
	)
	// The active turn can settle after busy classification but before this
	// durable enqueue. Kick the selector here as the other half of that handoff;
	// ClaimNext keeps concurrent turn-end kicks at-most-once.
	m.startNextQueuedInputPrompt(session.ID)
	return SendPromptResult{
		Status:          store.SessionPromptResultStatusQueued,
		Mode:            BusyInputModeQueue,
		Delivery:        entry.Delivery,
		QueueEntryID:    entry.ID,
		QueuePosition:   position,
		QueueGeneration: entry.SessionGeneration,
	}, nil
}
