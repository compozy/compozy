package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

func (m *Manager) submitAdmittedSteerCommand(
	ctx context.Context,
	session *Session,
	req promptRequest,
) (SendPromptResult, error) {
	workspaceID, err := sessionPromptWorkspaceID(session)
	if err != nil {
		return SendPromptResult{}, err
	}
	admissionReq, err := m.newPromptAdmissionRequest(
		workspaceID,
		req,
		store.SessionPromptOperationSteer,
		BusyInputModeSteer,
	)
	if err != nil {
		return SendPromptResult{}, err
	}
	unlock := m.lockPromptAdmission(workspaceID, session.ID, admissionReq.IdempotencyKey)
	defer unlock()
	if replayed, err := m.replayPromptAdmission(ctx, admissionReq); err != nil || replayed != nil {
		if err != nil {
			return SendPromptResult{}, err
		}
		return *replayed, nil
	}
	if !session.IsPrompting() {
		return SendPromptResult{}, fmt.Errorf("%w: %s", ErrPromptNotInProgress, session.ID)
	}
	return m.stageAdmittedSteerPrompt(ctx, session, req, admissionReq)
}

func (m *Manager) submitAdmittedGoalByTarget(
	ctx context.Context,
	preparation sendPromptPreparation,
	opts SendPromptOpts,
) (SendPromptResult, error) {
	target, err := m.resolvePromptAdmissionTarget(ctx, preparation.request.target, preparation.request.runtime)
	if err != nil {
		return SendPromptResult{}, err
	}
	preparation.request.runtime = target.runtime
	admissionReq, err := m.newPromptAdmissionRequest(
		target.workspaceID,
		preparation.request,
		store.SessionPromptOperationPrompt,
		preparation.mode,
	)
	if err != nil {
		return SendPromptResult{}, err
	}
	unlock := m.lockPromptAdmission(target.workspaceID, preparation.request.target, admissionReq.IdempotencyKey)
	defer unlock()
	if replayed, err := m.replayPromptAdmission(ctx, admissionReq); err != nil || replayed != nil {
		if err != nil {
			return SendPromptResult{}, err
		}
		return *replayed, nil
	}
	if err := m.validateRuntimeModelAtAdmission(ctx, target.session, *target.runtime); err != nil {
		return SendPromptResult{}, err
	}
	return m.submitAdmittedGoalPrompt(ctx, target.session, preparation, opts, admissionReq)
}

func (m *Manager) submitAdmittedPromptByTarget(
	ctx context.Context,
	preparation sendPromptPreparation,
) (SendPromptResult, error) {
	target, err := m.resolvePromptAdmissionTarget(ctx, preparation.request.target, preparation.request.runtime)
	if err != nil {
		return SendPromptResult{}, err
	}
	preparation.request.runtime = target.runtime
	admissionReq, err := m.newPromptAdmissionRequest(
		target.workspaceID,
		preparation.request,
		store.SessionPromptOperationPrompt,
		preparation.mode,
	)
	if err != nil {
		return SendPromptResult{}, err
	}
	unlock := m.lockPromptAdmission(target.workspaceID, preparation.request.target, admissionReq.IdempotencyKey)
	defer unlock()
	if replayed, err := m.replayPromptAdmission(ctx, admissionReq); err != nil || replayed != nil {
		if err != nil {
			return SendPromptResult{}, err
		}
		return *replayed, nil
	}
	if err := m.validateRuntimeModelAtAdmission(ctx, target.session, *target.runtime); err != nil {
		return SendPromptResult{}, err
	}
	session := target.session
	if session == nil {
		session, err = m.lookupPromptRequestSession(ctx, preparation.request)
		if err != nil {
			return SendPromptResult{}, err
		}
	}

	if session.IsPrompting() {
		return m.submitAdmittedBusyPrompt(ctx, session, preparation.request, preparation.mode, admissionReq)
	}
	return m.submitAdmittedDirectPrompt(ctx, session, preparation.request, preparation.mode, admissionReq)
}

type promptAdmissionTarget struct {
	session     *Session
	workspaceID string
	runtime     *RuntimeSelection
}

func (m *Manager) resolvePromptAdmissionTarget(
	ctx context.Context,
	target string,
	requested *RuntimeSelection,
) (promptAdmissionTarget, error) {
	if active, ok := m.Get(target); ok {
		resolved, err := m.preparePromptRuntimeSelection(ctx, active, requested)
		if err != nil {
			return promptAdmissionTarget{}, err
		}
		workspaceID, err := sessionPromptWorkspaceID(active)
		if err != nil {
			return promptAdmissionTarget{}, err
		}
		return promptAdmissionTarget{session: active, workspaceID: workspaceID, runtime: resolved}, nil
	}

	meta, err := m.readMetaWithContext(ctx, target)
	if err != nil {
		return promptAdmissionTarget{}, err
	}
	resolved, err := normalizePromptRuntimeSelectionFromMeta(meta, requested)
	if err != nil {
		return promptAdmissionTarget{}, err
	}
	return promptAdmissionTarget{workspaceID: meta.WorkspaceID, runtime: resolved}, nil
}

func (m *Manager) replayPromptAdmission(
	ctx context.Context,
	req store.SessionPromptAdmissionRequest,
) (*SendPromptResult, error) {
	admission, found, err := m.promptAdmissionStore.ReplaySessionPromptAdmission(ctx, req)
	if err != nil || !found || admission.State != store.SessionPromptAdmissionCompleted {
		return nil, err
	}
	result, err := sendPromptResultFromAdmission(admission)
	if err != nil {
		return nil, err
	}
	result.Replayed = true
	return &result, nil
}

func (m *Manager) submitAdmittedDirectPrompt(
	ctx context.Context,
	session *Session,
	req promptRequest,
	mode BusyInputMode,
	admissionReq store.SessionPromptAdmissionRequest,
) (SendPromptResult, error) {
	admission, replayed, err := m.claimPromptAdmission(ctx, admissionReq)
	if err != nil {
		return SendPromptResult{}, err
	}
	if replayed != nil {
		return *replayed, nil
	}
	req = bindPromptAdmissionRequest(req, admission)
	committed := false
	req.commitDispatch = func(dispatchCtx context.Context) error {
		if err := m.commitPromptAdmissionDispatch(dispatchCtx, admission); err != nil {
			return err
		}
		committed = true
		return nil
	}
	events, err := m.submitPromptRequest(ctx, req)
	if err != nil {
		if errors.Is(err, ErrPromptInProgress) && !committed {
			return m.submitAdmittedBusyPrompt(ctx, session, req, mode, admissionReq)
		}
		if committed {
			return SendPromptResult{}, m.promptDispatchIndeterminate(ctx, admission, err)
		}
		return SendPromptResult{}, err
	}
	result := SendPromptResult{
		Status: store.SessionPromptResultStatusAccepted,
		Mode:   mode, Delivery: store.SessionInputDeliveryDirect, Events: events,
		MessageID: admission.MessageID, IdempotencyKey: admission.IdempotencyKey,
		NewTurnID: admission.TurnID,
	}
	return m.completePromptAdmission(ctx, admission, result)
}

func (m *Manager) submitAdmittedBusyPrompt(
	ctx context.Context,
	session *Session,
	req promptRequest,
	mode BusyInputMode,
	admissionReq store.SessionPromptAdmissionRequest,
) (SendPromptResult, error) {
	switch mode {
	case BusyInputModeQueue:
		return m.enqueueAdmittedBusyPrompt(ctx, session, admissionReq)
	case BusyInputModeSteer:
		return m.stageAdmittedSteerPrompt(ctx, session, req, admissionReq)
	case BusyInputModeInterrupt:
		return m.interruptAdmittedPrompt(ctx, session, req, admissionReq)
	default:
		return SendPromptResult{}, fmt.Errorf("session: invalid busy input mode %q", mode)
	}
}

func (m *Manager) enqueueAdmittedBusyPrompt(
	ctx context.Context,
	session *Session,
	admissionReq store.SessionPromptAdmissionRequest,
) (SendPromptResult, error) {
	if m.inputQueue == nil {
		return SendPromptResult{}, ErrPromptInProgress
	}
	generation, err := m.currentInputGeneration(ctx, session.ID)
	if err != nil {
		return SendPromptResult{}, err
	}
	admission, entry, position, created, err := m.inputQueue.EnqueueAdmitted(ctx, admissionReq, generation)
	if err != nil {
		if errors.Is(err, store.ErrSessionInputQueueFull) {
			m.emitTranscriptMarker(
				ctx, session, session.CurrentTurnID(), transcript.MarkerPromptDropped,
				"Queued input rejected because the session input queue is full.",
				map[string]any{promptEvidenceQueueGenerationKey: generation, "queue_cap": m.busyInput.QueueCap},
			)
		}
		return SendPromptResult{}, err
	}
	if created {
		m.emitTranscriptMarker(
			ctx, session, session.CurrentTurnID(), transcript.MarkerPromptQueued,
			"Input queued while the session is busy.",
			queueEntryEvidence(entry.ID, entry.SessionGeneration, entry.Status, entry.Mode, position),
		)
	}
	result, err := sendPromptResultFromAdmission(admission)
	if err != nil {
		return SendPromptResult{}, err
	}
	result.Replayed = !created
	return result, nil
}

func (m *Manager) stageAdmittedSteerPrompt(
	ctx context.Context,
	session *Session,
	req promptRequest,
	admissionReq store.SessionPromptAdmissionRequest,
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
	admission, entry, created, err := m.inputQueue.StageAdmittedSteer(
		ctx,
		admissionReq,
		targetTurnID,
		generation,
	)
	if err != nil {
		return SendPromptResult{}, err
	}
	if err := m.ensureInterruptingInputActivated(ctx, session, &entry); err != nil {
		activationErr := m.cleanupInterruptingInputActivationFailure(ctx, &entry, err)
		return SendPromptResult{}, m.promptDispatchIndeterminate(ctx, admission, activationErr)
	}
	if created {
		m.emitTranscriptMarker(
			ctx, session, targetTurnID, transcript.MarkerPromptSteered,
			"Steering input accepted for the active turn.",
			queueEntryEvidence(entry.ID, entry.SessionGeneration, entry.Status, entry.Mode, 0),
		)
	}
	result, err := sendPromptResultFromAdmission(admission)
	if err != nil {
		return SendPromptResult{}, err
	}
	result.Replayed = !created
	return result, nil
}

func (m *Manager) interruptAdmittedPrompt(
	ctx context.Context,
	session *Session,
	req promptRequest,
	admissionReq store.SessionPromptAdmissionRequest,
) (SendPromptResult, error) {
	if m.inputQueue == nil {
		return SendPromptResult{}, ErrPromptInProgress
	}
	previousTurnID, err := requireExpectedActiveTurn(session, req.expectedTurnID)
	if err != nil {
		return SendPromptResult{}, err
	}
	admission, entry, created, err := m.inputQueue.EnqueueAdmittedInterrupt(
		ctx,
		admissionReq,
		previousTurnID,
	)
	if err != nil {
		return SendPromptResult{}, err
	}
	if err := m.ensureInterruptingInputActivated(ctx, session, &entry); err != nil {
		activationErr := m.cleanupInterruptingInputActivationFailure(ctx, &entry, err)
		return SendPromptResult{}, m.promptDispatchIndeterminate(ctx, admission, activationErr)
	}
	if created {
		m.emitTranscriptMarker(
			ctx,
			session,
			previousTurnID,
			transcript.MarkerPromptInterrupted,
			"Replacement input accepted and the active turn was interrupted.",
			map[string]any{
				promptEvidenceQueueGenerationKey: entry.SessionGeneration,
				promptEvidenceQueueEntryIDKey:    entry.ID,
			},
		)
	}
	result, err := sendPromptResultFromAdmission(admission)
	if err != nil {
		return SendPromptResult{}, err
	}
	result.Replayed = !created
	return result, nil
}

func (m *Manager) submitAdmittedGoalPrompt(
	ctx context.Context,
	session *Session,
	preparation sendPromptPreparation,
	opts SendPromptOpts,
	admissionReq store.SessionPromptAdmissionRequest,
) (SendPromptResult, error) {
	admission, replayed, err := m.claimPromptAdmission(ctx, admissionReq)
	if err != nil {
		return SendPromptResult{}, err
	}
	if replayed != nil {
		return *replayed, nil
	}
	preparation.request = bindPromptAdmissionRequest(preparation.request, admission)
	if err := m.validateActiveCursorRuntimeModel(session, preparation.request.runtime); err != nil {
		return SendPromptResult{}, err
	}
	if err := m.commitPromptAdmissionDispatch(ctx, admission); err != nil {
		return SendPromptResult{}, err
	}
	rejectIfBusy, goalResult, err := m.applyGoalCommand(ctx, &preparation.request, opts)
	if err != nil {
		return SendPromptResult{}, m.promptDispatchIndeterminate(ctx, admission, err)
	}
	if goalResult != nil {
		goalResult.Mode = preparation.mode
		goalResult.MessageID = admission.MessageID
		goalResult.IdempotencyKey = admission.IdempotencyKey
		return m.completePromptAdmission(ctx, admission, *goalResult)
	}
	if session == nil {
		session, err = m.lookupPromptRequestSession(ctx, preparation.request)
		if err != nil {
			return SendPromptResult{}, m.promptDispatchIndeterminate(ctx, admission, err)
		}
	}
	preparation.rejectIfBusy = rejectIfBusy
	result, err := m.submitPreparedPrompt(ctx, session, preparation)
	if err != nil {
		return SendPromptResult{}, m.promptDispatchIndeterminate(ctx, admission, err)
	}
	result.MessageID = admission.MessageID
	result.IdempotencyKey = admission.IdempotencyKey
	return m.completePromptAdmission(ctx, admission, result)
}

func (m *Manager) commitPromptAdmissionDispatch(
	ctx context.Context,
	admission store.SessionPromptAdmission,
) error {
	return m.promptAdmissionStore.CommitSessionPromptDispatch(
		ctx,
		admission.WorkspaceID,
		admission.SessionID,
		admission.IdempotencyKey,
		m.now(),
	)
}

func (m *Manager) completePromptAdmission(
	ctx context.Context,
	admission store.SessionPromptAdmission,
	result SendPromptResult,
) (SendPromptResult, error) {
	stored, err := sessionPromptAdmissionResult(result)
	if err != nil {
		return SendPromptResult{}, m.promptDispatchIndeterminate(ctx, admission, err)
	}
	if _, err := m.promptAdmissionStore.CompleteSessionPromptAdmission(
		ctx,
		admission.WorkspaceID,
		admission.SessionID,
		admission.IdempotencyKey,
		stored,
		m.now(),
	); err != nil {
		return SendPromptResult{}, m.promptDispatchIndeterminate(ctx, admission, err)
	}
	result.MessageID = admission.MessageID
	result.IdempotencyKey = admission.IdempotencyKey
	return result, nil
}

func (m *Manager) promptDispatchIndeterminate(
	ctx context.Context,
	admission store.SessionPromptAdmission,
	cause error,
) error {
	reason := "dispatch failed after the at-most-once boundary: " + cause.Error()
	markErr := m.promptAdmissionStore.MarkSessionPromptAdmissionIndeterminate(
		context.WithoutCancel(ctx),
		admission.WorkspaceID,
		admission.SessionID,
		admission.IdempotencyKey,
		reason,
		m.now(),
	)
	indeterminate := fmt.Errorf("%w: %s", store.ErrSessionPromptDispatchIndeterminate, reason)
	return errors.Join(indeterminate, markErr)
}
