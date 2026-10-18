package session

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

// errSteerDeliveredUnresolved marks a steer the provider already accepted whose durable records could
// not be saved. Its reservation stays leased: requeueing it would deliver the same guidance twice.
var errSteerDeliveredUnresolved = errors.New("session: delivered steer could not be resolved")

// steerPersistAttempts bounds the synchronous retries of each durable record of an accepted steer.
const steerPersistAttempts = 3

func (m *Manager) activateSteeringInput(
	ctx context.Context,
	session *Session,
	entry *store.SessionInputQueueEntry,
) error {
	return m.activateSteeringInputText(ctx, session, entry, "")
}

// activateSteeringInputText delivers entry as steering. A non-empty providerText (an input.pre_submit
// rewrite) replaces the text the provider receives; the authored entry text stays the durable record.
func (m *Manager) activateSteeringInputText(
	ctx context.Context,
	session *Session,
	entry *store.SessionInputQueueEntry,
	providerText string,
) error {
	defer m.replyWatchSteerResolved(context.WithoutCancel(ctx), session.ID)
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
	if steerer, ok := m.injectedSteerer(proc); ok && session.CurrentTurnID() == entry.TargetTurnID &&
		!m.agentSteerYieldsToOperator(ctx, session, entry) {
		steerCtx, cancel := context.WithTimeout(m.fallbackLifecycleContext(), defaultLifecycleTimeout)
		origin, originErr := decodePromptOrigin(entry.Origin)
		if originErr != nil {
			cancel()
			return originErr
		}
		session.raiseTurnHop(entry.TargetTurnID, originHop(origin))
		text := entry.Text
		if providerText != "" {
			text = providerText
		}
		attempt, steerErr := steerer.Steer(steerCtx, proc, entry.TargetTurnID, promptOriginMessage(origin, text))
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
	entry.SteerDelivery = delivery
	var resolved store.SessionInputQueueEntry
	if delivery == store.SteerDeliveryInterruptFallback {
		resolved, err = m.inputQueue.ResolveSteer(m.fallbackLifecycleContext(), session.ID, entry.ID, delivery)
		if err != nil {
			return err
		}
	} else if resolved, err = m.persistAcceptedSteer(session, entry); err != nil {
		return errors.Join(errSteerDeliveredUnresolved, err)
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
	if steerFallbackWaitsForTurn(entry) || session.CurrentTurnID() != entry.TargetTurnID {
		m.startNextQueuedInputPrompt(session.ID)
		return nil
	}
	return m.activateInterruptingInput(ctx, session, entry)
}

// agentSteerYieldsToOperator keeps another session's steer out of the live turn while operator input
// waits: injecting it would let that session cut ahead of the operator. The steer then degrades to
// after-turn delivery behind the operator's input, the same as a provider without injection.
func (m *Manager) agentSteerYieldsToOperator(
	ctx context.Context, session *Session, entry *store.SessionInputQueueEntry,
) bool {
	if entry.OriginClass != store.SessionInputOriginClassAgent {
		return false
	}
	entries, err := m.inputQueue.List(ctx, session.ID)
	if err != nil {
		m.sessionLogger(session).Warn("session: list queued input before agent steer failed; deferring steer",
			"entry_id", entry.ID, "error", err)
		return true
	}
	return operatorInputWaits(entries)
}

// operatorInputWaits reports undelivered operator input among a session's pending entries.
func operatorInputWaits(entries []store.SessionInputQueueEntry) bool {
	for index := range entries {
		if entries[index].OwnerKind == "" && store.IsHumanSessionInputOriginClass(entries[index].OriginClass) {
			return true
		}
	}
	return false
}

// injectedSteerer returns the driver that can inject guidance into proc's live turn, when the
// provider advertises or is configured with a steering capability.
func (m *Manager) injectedSteerer(proc *AgentProcess) (AgentSteerer, bool) {
	steerer, ok := m.driver.(AgentSteerer)
	if !ok || proc == nil {
		return nil, false
	}
	capability := proc.CapsSnapshot().SteerCapability
	if capability != config.SteerCapabilityExtension && capability != config.SteerCapabilityConcurrentPrompt {
		return nil, false
	}
	return steerer, true
}

func (m *Manager) emitSteerMarker(ctx context.Context, session *Session, entry *store.SessionInputQueueEntry) {
	for _, id := range entry.SupersededIDs {
		evidence := map[string]any{"queue_entry_id": id, "replacement_entry_id": entry.ID}
		older, err := m.inputQueue.Get(ctx, session.ID, id)
		switch {
		case err != nil:
			m.sessionLogger(session).Warn("session: read superseded guidance failed", "entry_id", id, "error", err)
		case older.Mode == store.SessionInputQueueModeQueue:
			// A superseding steer also replaced the sender's older queued message.
			m.settleSupersededQueuedInput(ctx, session, entry, &older)
			continue
		default:
			addSteerAuthoredEvidence(evidence, &older)
		}
		m.emitTranscriptMarker(ctx, session, entry.TargetTurnID, transcript.MarkerPromptSuperseded,
			"Undelivered steering replaced by newer guidance.", evidence,
		)
	}
	message := "Steering interrupted and replaced the active turn."
	switch {
	case entry.SteerDelivery == store.SteerDeliveryInjected:
		message = "Steering delivered into the live turn."
	case entry.SteerDelivery == store.SteerDeliveryPendingInjection:
		message = "Steering is waiting for the active tool to finish."
	case steerFallbackWaitsForTurn(entry):
		message = "Steering from another session will be delivered after the active turn."
	}
	evidence := queueEntryEvidence(entry.ID, entry.SessionGeneration, entry.Status, entry.Mode, 0)
	addSteerAuthoredEvidence(evidence, entry)
	evidence["steer_delivery"] = entry.SteerDelivery
	m.emitTranscriptMarker(ctx, session, entry.TargetTurnID, transcript.MarkerPromptSteered, message, evidence)
}

// persistAcceptedSteer saves the durable records of guidance the provider accepted: its input event
// first, then its resolution. The entry is never marked sent before its input event is durable, so the
// guidance cannot vanish from history; on failure it stays leased instead of returning to the queue.
func (m *Manager) persistAcceptedSteer(
	session *Session, entry *store.SessionInputQueueEntry,
) (store.SessionInputQueueEntry, error) {
	ctx := m.fallbackLifecycleContext()
	if err := m.recordAcceptedSteerInput(ctx, session, entry); err != nil {
		return store.SessionInputQueueEntry{}, err
	}
	return m.resolveAcceptedSteer(ctx, session.ID, entry.ID, entry.SteerDelivery)
}

// recordAcceptedSteerInput retries the input event of delivered guidance a bounded number of times. Every
// attempt carries the same event identity and timestamp, so a retry after a write that landed returns the
// stored row instead of a second event.
func (m *Manager) recordAcceptedSteerInput(
	ctx context.Context, session *Session, entry *store.SessionInputQueueEntry,
) error {
	recordedAt := m.now()
	var err error
	for range steerPersistAttempts {
		if err = m.recordInjectedSteerInput(ctx, session, entry, recordedAt); err == nil {
			return nil
		}
	}
	return err
}

func (m *Manager) resolveAcceptedSteer(
	ctx context.Context, sessionID, entryID string, delivery store.SteerDeliveryMode,
) (store.SessionInputQueueEntry, error) {
	var (
		resolved store.SessionInputQueueEntry
		err      error
	)
	for range steerPersistAttempts {
		if resolved, err = m.inputQueue.ResolveSteer(ctx, sessionID, entryID, delivery); err == nil {
			return resolved, nil
		}
	}
	return store.SessionInputQueueEntry{}, err
}

// A pending injection is represented by its durable authored receipt until the
// provider confirms delivery. Only confirmed injection becomes a sent user
// message; fallback keeps the ordinary queue-dispatch identity and turn.
func (m *Manager) recordInjectedSteerInput(
	ctx context.Context, session *Session, entry *store.SessionInputQueueEntry, recordedAt time.Time,
) error {
	if entry.SteerDelivery != store.SteerDeliveryInjected {
		return nil
	}
	origin, err := decodePromptOrigin(entry.Origin)
	if err != nil {
		return err
	}
	req := promptRequest{
		meta:   acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser, Origin: origin},
		target: session.ID, turnID: entry.TargetTurnID, turnSource: TurnSourceUser,
		message: entry.Text, authoredMessage: entry.Text,
		messageID: entry.MessageID, idempotencyKey: entry.IdempotencyKey, eventID: entry.EventID,
		runtime: runtimeSelectionFromStore(entry.Runtime), skillInvocations: entry.SkillInvocations,
		inputAt: recordedAt,
	}
	return m.recordPromptInputEvent(ctx, session, &req)
}

// steerFallbackWaitsForTurn reports a steer from another session or the daemon
// whose live injection was unavailable. It never interrupts the active turn:
// the store degrades it to after-turn delivery so it cannot displace human input.
func steerFallbackWaitsForTurn(entry *store.SessionInputQueueEntry) bool {
	return entry.SteerDelivery == store.SteerDeliveryInterruptFallback &&
		entry.Delivery == store.SessionInputDeliveryAfterTurn
}

func addSteerAuthoredEvidence(evidence map[string]any, entry *store.SessionInputQueueEntry) {
	evidence["message_id"] = entry.MessageID
	evidence["authored_text"] = entry.Text
	evidence["input_event_id"] = entry.EventID
	evidence["target_turn_id"] = entry.TargetTurnID
}
