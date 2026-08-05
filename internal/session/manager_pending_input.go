package session

import (
	"context"
	"errors"
	"strings"

	commandpkg "github.com/compozy/compozy/internal/command"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

// ListPendingInputs returns the daemon-owned current-generation input queue.
func (m *Manager) ListPendingInputs(ctx context.Context, id string) ([]PendingInput, error) {
	if m == nil {
		return nil, errors.New("session: manager is required")
	}
	if ctx == nil {
		return nil, errors.New("session: list pending inputs context is required")
	}
	if m.inputQueue == nil {
		return []PendingInput{}, nil
	}
	session, err := m.lookupPromptSession(ctx, id)
	if err != nil {
		return nil, err
	}
	entries, err := m.inputQueue.List(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	inputs := make([]PendingInput, 0, len(entries))
	for index := range entries {
		inputs = append(inputs, pendingInputFromStore(&entries[index]))
	}
	return inputs, nil
}

// ReplacePendingInput atomically swaps one queued item for a new durable identity.
func (m *Manager) ReplacePendingInput(
	ctx context.Context,
	id string,
	entryID string,
	opts ReplacePendingInputOpts,
) (PendingInput, error) {
	if m == nil || m.inputQueue == nil {
		return PendingInput{}, errors.New("session: input queue is not configured")
	}
	if ctx == nil {
		return PendingInput{}, errors.New("session: replace pending input context is required")
	}
	if err := validateCommandIngress(opts.AllowCommands, opts.Caller); err != nil {
		return PendingInput{}, err
	}
	session, err := m.lookupPromptSession(ctx, id)
	if err != nil {
		return PendingInput{}, err
	}
	request := promptRequest{target: session.ID, authoredMessage: opts.Text}
	if opts.AllowCommands {
		if err := m.preparePromptSkillInvocations(ctx, &request); err != nil {
			return PendingInput{}, err
		}
	}
	entry, _, err := m.inputQueue.Replace(
		ctx,
		session.ID,
		strings.TrimSpace(entryID),
		opts.Text,
		opts.MessageID,
		opts.IdempotencyKey,
		request.skillInvocations,
	)
	if err != nil {
		return PendingInput{}, err
	}
	return pendingInputFromStore(&entry), nil
}

// PromotePendingInputToSteer atomically prioritizes queued input and interrupts its target turn.
func (m *Manager) PromotePendingInputToSteer(
	ctx context.Context,
	id string,
	entryID string,
	opts PromotePendingInputOpts,
) (SendPromptResult, error) {
	if m == nil || m.inputQueue == nil {
		return SendPromptResult{}, errors.New("session: input queue is not configured")
	}
	if ctx == nil {
		return SendPromptResult{}, errors.New("session: promote pending input context is required")
	}
	if err := validateCommandIngress(opts.AllowCommands, opts.Caller); err != nil {
		return SendPromptResult{}, err
	}
	session, err := m.lookupPromptSession(ctx, id)
	if err != nil {
		return SendPromptResult{}, err
	}
	targetTurnID := strings.TrimSpace(opts.ExpectedTurnID)
	if targetTurnID == "" {
		return SendPromptResult{}, errors.Join(ErrActiveTurnMismatch, errors.New("expected turn id is required"))
	}
	request := promptRequest{target: session.ID, authoredMessage: opts.Text}
	if opts.AllowCommands {
		if err := m.preparePromptSkillInvocations(ctx, &request); err != nil {
			return SendPromptResult{}, err
		}
	}
	replayed, found, err := m.inputQueue.ReplayPromotion(
		ctx,
		session.ID,
		strings.TrimSpace(entryID),
		opts.Text,
		targetTurnID,
		opts.MessageID,
		opts.IdempotencyKey,
		request.skillInvocations,
	)
	if err != nil {
		return SendPromptResult{}, err
	}
	if found {
		return promotedInputResult(&replayed), nil
	}
	targetTurnID, err = requireExpectedActiveTurn(session, targetTurnID)
	if err != nil {
		return SendPromptResult{}, err
	}
	entry, created, err := m.inputQueue.PromoteToSteer(
		ctx,
		session.ID,
		strings.TrimSpace(entryID),
		opts.Text,
		targetTurnID,
		opts.MessageID,
		opts.IdempotencyKey,
		request.skillInvocations,
	)
	if err != nil {
		return SendPromptResult{}, err
	}
	if err := m.ensureInterruptingInputActivated(ctx, session, &entry); err != nil {
		return SendPromptResult{}, m.cleanupInterruptingInputActivationFailure(ctx, &entry, err)
	}
	if created {
		m.emitTranscriptMarker(
			ctx,
			session,
			targetTurnID,
			transcript.MarkerPromptSteered,
			"Queued input promoted to steering for the active turn.",
			queueEntryEvidence(entry.ID, entry.SessionGeneration, entry.Status, entry.Mode, 0),
		)
	}
	return promotedInputResult(&entry), nil
}

func promotedInputResult(entry *store.SessionInputQueueEntry) SendPromptResult {
	return SendPromptResult{
		Status:          store.SessionPromptResultStatusSteering,
		Mode:            BusyInputModeSteer,
		Delivery:        entry.Delivery,
		MessageID:       entry.MessageID,
		IdempotencyKey:  entry.IdempotencyKey,
		QueueEntryID:    entry.ID,
		QueueGeneration: entry.SessionGeneration,
	}
}

func pendingInputFromStore(entry *store.SessionInputQueueEntry) PendingInput {
	runtime := runtimeSelectionFromStore(entry.Runtime)
	return PendingInput{
		ID:               entry.ID,
		SessionID:        entry.SessionID,
		MessageID:        entry.MessageID,
		IdempotencyKey:   entry.IdempotencyKey,
		TargetTurnID:     entry.TargetTurnID,
		Status:           entry.Status,
		Mode:             BusyInputMode(entry.Mode),
		Delivery:         entry.Delivery,
		Text:             entry.Text,
		QueueGeneration:  entry.SessionGeneration,
		EnqueuedAt:       entry.EnqueuedAt,
		Runtime:          runtime,
		SkillInvocations: append([]commandpkg.Invocation(nil), entry.SkillInvocations...),
	}
}
