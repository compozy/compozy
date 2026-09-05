package session

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

func (m *Manager) activateSteeringInput(
	ctx context.Context,
	session *Session,
	entry *store.SessionInputQueueEntry,
) error {
	superseded := entry.SupersededIDs
	reserved, claimed, err := m.inputQueue.ReserveSteer(ctx, session.ID, entry.ID)
	if err != nil {
		return err
	}
	*entry = reserved
	entry.SupersededIDs = superseded
	if !claimed {
		switch entry.Status {
		case store.SessionInputQueueStatusDispatching, store.SessionInputQueueStatusSent:
			return nil
		default:
			return store.ErrSessionInputQueueEntryNotQueued
		}
	}
	delivery := store.SteerDeliveryInterruptFallback
	var completion <-chan error
	proc := session.processHandle()
	steerer, supportsSteer := m.driver.(AgentSteerer)
	if supportsSteer && proc != nil && session.CurrentTurnID() == entry.TargetTurnID {
		capability := proc.CapsSnapshot().SteerCapability
		if capability == config.SteerCapabilityExtension || capability == config.SteerCapabilityConcurrentPrompt {
			steerCtx, cancel := context.WithTimeout(m.fallbackLifecycleContext(), defaultLifecycleTimeout)
			attempt, steerErr := steerer.Steer(steerCtx, proc, entry.TargetTurnID, entry.Text)
			cancel()
			if steerErr == nil {
				completion = attempt.Completion
				switch attempt.Attempt {
				case acp.SteerAttemptInjected:
					delivery = store.SteerDeliveryInjected
				case acp.SteerAttemptPendingInjection:
					delivery = store.SteerDeliveryPendingInjection
				}
			} else if !errors.Is(steerErr, acp.ErrSteerTurnMismatch) {
				m.sessionLogger(session).
					Warn("session: live steer failed; falling back", "entry_id", entry.ID, "error", steerErr)
			}
		}
	}
	resolved, err := m.inputQueue.ResolveSteer(m.fallbackLifecycleContext(), session.ID, entry.ID, delivery)
	if err != nil {
		return err
	}
	*entry = resolved
	entry.SupersededIDs = superseded
	session.mu.Lock()
	session.steerDelivery = delivery
	session.mu.Unlock()
	m.emitSteerMarker(ctx, session, entry)
	if completion != nil {
		m.watchSteerCompletion(session, entry.ID, entry.TargetTurnID, completion)
	}
	if delivery != store.SteerDeliveryInterruptFallback {
		return nil
	}
	if session.CurrentTurnID() != entry.TargetTurnID {
		m.startNextQueuedInputPrompt(session.ID)
		return nil
	}
	return m.activateInterruptingInput(ctx, session, entry)
}

func (m *Manager) emitSteerMarker(ctx context.Context, session *Session, entry *store.SessionInputQueueEntry) {
	for _, id := range entry.SupersededIDs {
		m.emitTranscriptMarker(ctx, session, entry.TargetTurnID, transcript.MarkerPromptSuperseded,
			"Undelivered steering replaced by newer guidance.",
			map[string]any{"queue_entry_id": id, "replacement_entry_id": entry.ID},
		)
	}
	message := "Steering interrupted and replaced the active turn."
	switch entry.SteerDelivery {
	case store.SteerDeliveryInjected:
		message = "Steering delivered into the live turn."
	case store.SteerDeliveryPendingInjection:
		message = "Steering is waiting for the active tool to finish."
	}
	evidence := queueEntryEvidence(entry.ID, entry.SessionGeneration, entry.Status, entry.Mode, 0)
	evidence["steer_delivery"] = entry.SteerDelivery
	m.emitTranscriptMarker(ctx, session, entry.TargetTurnID, transcript.MarkerPromptSteered, message, evidence)
}
