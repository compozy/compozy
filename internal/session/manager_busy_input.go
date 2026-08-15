package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

const (
	promptEvidenceQueueEntryIDKey    = "queue_entry_id"
	promptEvidenceQueueGenerationKey = "queue_generation"
)

// SendPrompt submits a user-facing prompt and applies busy-input policy when a turn is active.
func (m *Manager) SendPrompt(ctx context.Context, id string, opts SendPromptOpts) (SendPromptResult, error) {
	if m == nil {
		return SendPromptResult{}, errors.New("session: manager is required")
	}
	if ctx == nil {
		return SendPromptResult{}, errors.New("session: prompt context is required")
	}
	preparation, goalResult, err := m.prepareSendPrompt(ctx, id, opts)
	if err != nil {
		return SendPromptResult{}, err
	}
	if goalResult != nil {
		return *goalResult, nil
	}
	if preparation.request.hasPromptAdmissionIdentity() && opts.AllowCommands {
		_, matched, parseErr := ParseGoalCommand(preparation.request.authoredMessage)
		if parseErr != nil && !matched {
			return SendPromptResult{}, fmt.Errorf("session: parse Goal command: %w", parseErr)
		}
		if matched {
			// submitAdmittedGoalByTarget parses again to persist the deterministic Goal result.
			return m.submitAdmittedGoalByTarget(ctx, preparation, opts)
		}
	}
	if preparation.request.hasPromptAdmissionIdentity() {
		return m.submitAdmittedPromptByTarget(ctx, preparation)
	}
	session, err := m.lookupPromptRequestSession(ctx, preparation.request)
	if err != nil {
		return SendPromptResult{}, err
	}
	workspaceID, err := sessionPromptWorkspaceID(session)
	if err != nil {
		return SendPromptResult{}, err
	}
	preparation.request.attachments, err = m.canonicalPromptAttachments(
		ctx, workspaceID, session.ID, preparation.request.attachments,
	)
	if err != nil {
		return SendPromptResult{}, err
	}
	preparation.request.runtime, err = m.resolvePromptRuntimeAtAdmission(
		ctx,
		session,
		preparation.request.runtime,
	)
	if err != nil {
		return SendPromptResult{}, err
	}
	return m.submitPreparedPrompt(ctx, session, preparation)
}

func normalizePromptRuntimeSelectionFromMeta(
	meta store.SessionMeta,
	requested *RuntimeSelection,
) (*RuntimeSelection, error) {
	selection := requested
	if selection == nil {
		selected, _ := store.SessionRuntimeSelectionStateValues(meta.RuntimeSelection)
		selection = runtimeSelectionFromSessionStore(selected)
	}
	if selection == nil {
		selection = &RuntimeSelection{
			Provider: meta.Provider, Model: meta.Model,
			ReasoningEffort: meta.ReasoningEffort, Speed: meta.Speed,
		}
	}
	normalized, err := NormalizeRuntimeSelection(*selection)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

func (m *Manager) resolvePromptRuntimeAtAdmission(
	ctx context.Context,
	session *Session,
	requested *RuntimeSelection,
) (*RuntimeSelection, error) {
	selection, err := m.preparePromptRuntimeSelection(ctx, session, requested)
	if err != nil {
		return nil, err
	}
	if err := m.validateRuntimeModelAtAdmission(ctx, session, *selection); err != nil {
		return nil, err
	}
	return selection, nil
}

func (m *Manager) preparePromptRuntimeSelection(
	ctx context.Context,
	session *Session,
	requested *RuntimeSelection,
) (*RuntimeSelection, error) {
	selection, err := normalizePromptRuntimeSelection(session, requested)
	if err != nil {
		return nil, err
	}
	plan, err := m.preparePromptRuntimePlan(ctx, session, *selection)
	if err != nil {
		return nil, err
	}
	return &plan.selection, nil
}

type sendPromptPreparation struct {
	request      promptRequest
	mode         BusyInputMode
	rejectIfBusy bool
}

func (m *Manager) prepareSendPrompt(
	ctx context.Context,
	id string,
	opts SendPromptOpts,
) (sendPromptPreparation, *SendPromptResult, error) {
	if err := validateCommandIngress(opts.AllowCommands, opts.Caller); err != nil {
		return sendPromptPreparation{}, nil, err
	}
	mode, err := m.normalizeBusyInputMode(opts.Mode)
	if err != nil {
		return sendPromptPreparation{}, nil, err
	}
	req, err := m.parseSendPromptRequest(ctx, id, opts)
	if err != nil {
		return sendPromptPreparation{}, nil, err
	}
	if err := m.checkNewWorkAdmission(ctx); err != nil {
		return sendPromptPreparation{}, nil, err
	}
	req.messageID = strings.TrimSpace(opts.MessageID)
	req.idempotencyKey = strings.TrimSpace(opts.IdempotencyKey)
	req.expectedTurnID = strings.TrimSpace(opts.ExpectedTurnID)
	req.runtime = cloneRuntimeSelection(opts.Runtime)
	req.resumeStopped = strings.TrimSpace(string(opts.Mode)) == ""
	if opts.AllowCommands {
		if err := m.preparePromptSkillInvocations(ctx, &req); err != nil {
			return sendPromptPreparation{}, nil, err
		}
	}
	if err := req.validatePromptAdmissionIdentity(); err != nil {
		return sendPromptPreparation{}, nil, err
	}
	if req.hasPromptAdmissionIdentity() {
		return sendPromptPreparation{request: req, mode: mode}, nil, nil
	}
	rejectIfBusy, goalResult, err := m.applyGoalCommand(ctx, &req, opts)
	if err != nil {
		return sendPromptPreparation{}, nil, err
	}
	return sendPromptPreparation{
		request:      req,
		mode:         mode,
		rejectIfBusy: rejectIfBusy,
	}, goalResult, nil
}
func (m *Manager) submitPreparedPrompt(
	ctx context.Context,
	session *Session,
	preparation sendPromptPreparation,
) (SendPromptResult, error) {
	req := preparation.request
	if session.IsPrompting() {
		return m.submitBusyPreparedPrompt(ctx, session, req, preparation)
	}
	events, err := m.submitPromptRequest(ctx, req)
	if err != nil {
		if preparation.rejectIfBusy && errors.Is(err, ErrPromptInProgress) {
			return goalDraftBusyResult(), nil
		}
		return SendPromptResult{}, err
	}
	return SendPromptResult{
		Status:    store.SessionPromptResultStatusAccepted,
		Mode:      preparation.mode,
		Delivery:  store.SessionInputDeliveryDirect,
		Events:    events,
		NewTurnID: req.turnID,
	}, nil
}

func (m *Manager) submitBusyPreparedPrompt(
	ctx context.Context,
	session *Session,
	req promptRequest,
	preparation sendPromptPreparation,
) (SendPromptResult, error) {
	if preparation.rejectIfBusy {
		return goalDraftBusyResult(), nil
	}
	switch preparation.mode {
	case BusyInputModeQueue:
		return m.enqueueBusyPrompt(ctx, session, req)
	case BusyInputModeInterrupt:
		return m.interruptAndSubmitPrompt(ctx, session, req)
	case BusyInputModeSteer:
		return m.stageSteerPrompt(ctx, session, req)
	default:
		return SendPromptResult{}, fmt.Errorf("session: invalid busy input mode %q", preparation.mode)
	}
}

// SteerPrompt durably accepts guidance that replaces the fenced active turn.
func (m *Manager) SteerPrompt(ctx context.Context, id string, opts SteerPromptOpts) (SendPromptResult, error) {
	if m == nil {
		return SendPromptResult{}, errors.New("session: manager is required")
	}
	if ctx == nil {
		return SendPromptResult{}, errors.New("session: steer prompt context is required")
	}
	if err := validateCommandIngress(opts.AllowCommands, opts.Caller); err != nil {
		return SendPromptResult{}, err
	}
	req, err := m.parsePromptRequest(ctx, id, PromptOpts{Message: opts.Message, TurnSource: TurnSourceUser})
	if err != nil {
		return SendPromptResult{}, err
	}
	req.messageID = strings.TrimSpace(opts.MessageID)
	req.idempotencyKey = strings.TrimSpace(opts.IdempotencyKey)
	req.expectedTurnID = strings.TrimSpace(opts.ExpectedTurnID)
	if opts.AllowCommands {
		if err := m.preparePromptSkillInvocations(ctx, &req); err != nil {
			return SendPromptResult{}, err
		}
	}
	if err := req.validatePromptAdmissionIdentity(); err != nil {
		return SendPromptResult{}, err
	}
	session, err := m.lookupPromptSession(ctx, req.target)
	if err != nil {
		return SendPromptResult{}, err
	}
	if req.hasPromptAdmissionIdentity() {
		req.runtime, err = m.resolvePromptRuntimeAtAdmission(ctx, session, nil)
		if err != nil {
			return SendPromptResult{}, err
		}
		return m.submitAdmittedSteerCommand(ctx, session, req)
	}
	if !session.IsPrompting() {
		return SendPromptResult{}, fmt.Errorf("%w: %s", ErrPromptNotInProgress, session.ID)
	}
	return m.stageSteerPrompt(ctx, session, req)
}

// CancelQueuedPrompt cancels one pending busy-input queue entry.
func (m *Manager) CancelQueuedPrompt(ctx context.Context, id string, queueEntryID string) (SendPromptResult, error) {
	if m == nil {
		return SendPromptResult{}, errors.New("session: manager is required")
	}
	if ctx == nil {
		return SendPromptResult{}, errors.New("session: cancel queued prompt context is required")
	}
	if m.inputQueue == nil {
		return SendPromptResult{}, errors.New("session: input queue is not configured")
	}
	session, err := m.lookupPromptSession(ctx, id)
	if err != nil {
		return SendPromptResult{}, err
	}
	entry, err := m.inputQueue.Cancel(ctx, session.ID, queueEntryID)
	if err != nil {
		return SendPromptResult{}, err
	}
	m.emitTranscriptMarker(
		ctx,
		session,
		session.CurrentTurnID(),
		transcript.MarkerPromptDropped,
		"Queued input canceled by operator.",
		queueEntryEvidence(entry.ID, entry.SessionGeneration, entry.Status, entry.Mode, 0),
	)
	return SendPromptResult{
		Status:          store.SessionPromptResultStatusCanceled,
		Mode:            BusyInputMode(entry.Mode),
		Delivery:        store.SessionInputDeliveryNone,
		QueueEntryID:    entry.ID,
		QueueGeneration: entry.SessionGeneration,
	}, nil
}

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
		session.ID,
		req.message,
		generation,
		storeRuntimeSelection(req.runtime),
		req.skillInvocations,
		storeAttachments(req.attachments),
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
	return SendPromptResult{
		Status:          store.SessionPromptResultStatusQueued,
		Mode:            BusyInputModeQueue,
		Delivery:        entry.Delivery,
		QueueEntryID:    entry.ID,
		QueuePosition:   position,
		QueueGeneration: entry.SessionGeneration,
	}, nil
}

func (m *Manager) stageSteerPrompt(
	ctx context.Context,
	session *Session,
	req promptRequest,
) (SendPromptResult, error) {
	if m.inputQueue == nil {
		return SendPromptResult{}, ErrPromptInProgress
	}
	targetTurnID, err := requireExpectedActiveTurn(session, req.expectedTurnID)
	if err != nil {
		return SendPromptResult{}, err
	}
	generation, err := m.currentInputGeneration(ctx, session.ID)
	if err != nil {
		return SendPromptResult{}, err
	}
	entry, err := m.inputQueue.StageSteer(
		ctx,
		session.ID,
		req.message,
		targetTurnID,
		generation,
		storeRuntimeSelection(req.runtime),
		req.skillInvocations,
		storeAttachments(req.attachments),
	)
	if err != nil {
		return SendPromptResult{}, err
	}
	if err := m.activateInterruptingInput(ctx, session, &entry); err != nil {
		return SendPromptResult{}, m.cleanupInterruptingInputActivationFailure(ctx, &entry, err)
	}
	m.emitTranscriptMarker(
		ctx,
		session,
		targetTurnID,
		transcript.MarkerPromptSteered,
		"Steering input accepted for the active turn.",
		queueEntryEvidence(entry.ID, entry.SessionGeneration, entry.Status, entry.Mode, 0),
	)
	return SendPromptResult{
		Status:          store.SessionPromptResultStatusSteering,
		Mode:            BusyInputModeSteer,
		Delivery:        entry.Delivery,
		QueueEntryID:    entry.ID,
		QueueGeneration: entry.SessionGeneration,
	}, nil
}

func (m *Manager) interruptAndSubmitPrompt(
	ctx context.Context,
	session *Session,
	req promptRequest,
) (SendPromptResult, error) {
	previousTurnID, err := requireExpectedActiveTurn(session, req.expectedTurnID)
	if err != nil {
		return SendPromptResult{}, err
	}
	entry, canceled, err := m.inputQueue.EnqueueInterrupt(
		ctx,
		session.ID,
		req.message,
		previousTurnID,
		storeRuntimeSelection(req.runtime),
		req.skillInvocations,
		storeAttachments(req.attachments),
	)
	if err != nil {
		return SendPromptResult{}, err
	}
	if err := m.activateInterruptingInput(ctx, session, &entry); err != nil {
		return SendPromptResult{}, m.cleanupInterruptingInputActivationFailure(ctx, &entry, err)
	}
	m.emitTranscriptMarker(
		ctx,
		session,
		previousTurnID,
		transcript.MarkerPromptInterrupted,
		"Replacement input accepted and the active turn was interrupted.",
		map[string]any{
			promptEvidenceQueueGenerationKey: entry.SessionGeneration,
			"canceled_queue_entries":         canceled,
			promptEvidenceQueueEntryIDKey:    entry.ID,
		},
	)
	return SendPromptResult{
		Status:                store.SessionPromptResultStatusInterrupting,
		Mode:                  BusyInputModeInterrupt,
		Delivery:              entry.Delivery,
		QueueEntryID:          entry.ID,
		PreviousTurnID:        previousTurnID,
		QueueGeneration:       entry.SessionGeneration,
		CanceledQueuedEntries: canceled,
	}, nil
}

func validateCommandIngress(allowCommands bool, caller PromptCaller) error {
	if !allowCommands {
		return nil
	}
	if err := caller.Validate(); err != nil {
		return fmt.Errorf("session: authorize slash commands: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(caller.Kind), "human") {
		return errors.New("session: slash commands require human operator ingress")
	}
	return nil
}

func (m *Manager) normalizeBusyInputMode(mode BusyInputMode) (BusyInputMode, error) {
	value := strings.TrimSpace(string(mode))
	if value == "" {
		value = strings.TrimSpace(m.busyInput.Normalize().DefaultMode)
	}
	switch BusyInputMode(value) {
	case BusyInputModeQueue, BusyInputModeInterrupt, BusyInputModeSteer:
		return BusyInputMode(value), nil
	default:
		return "", fmt.Errorf("session: invalid busy input mode %q", value)
	}
}

func (m *Manager) currentInputGeneration(ctx context.Context, sessionID string) (int64, error) {
	if m.inputQueue == nil {
		return 0, nil
	}
	return m.inputQueue.CurrentGeneration(ctx, sessionID)
}

func queueEntryEvidence(entryID string, generation int64, status string, mode string, position int) map[string]any {
	evidence := map[string]any{
		promptEvidenceQueueEntryIDKey:    entryID,
		promptEvidenceQueueGenerationKey: generation,
		"queue_status":                   status,
		"mode":                           mode,
	}
	if position > 0 {
		evidence["queue_position"] = position
	}
	return evidence
}
