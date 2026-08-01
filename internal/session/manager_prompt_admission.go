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
	if salvage, ok := m.pendingInterruptedPromptSalvage(session.ID); ok {
		admission, replayed, claimErr := m.claimPromptAdmission(ctx, admissionReq)
		if claimErr != nil {
			return SendPromptResult{}, claimErr
		}
		if replayed != nil {
			return *replayed, nil
		}
		req = bindPromptAdmissionRequest(req, admission)
		if err := m.commitPromptAdmissionDispatch(ctx, admission); err != nil {
			return SendPromptResult{}, err
		}
		result, submitErr := m.submitInterruptedPromptSalvage(ctx, session, req, salvage)
		if submitErr != nil {
			return SendPromptResult{}, m.promptDispatchIndeterminate(ctx, admission, submitErr)
		}
		result.MessageID = admission.MessageID
		result.IdempotencyKey = admission.IdempotencyKey
		return m.completePromptAdmission(ctx, admission, result)
	}
	if !session.IsPrompting() {
		return SendPromptResult{}, fmt.Errorf("%w: %s", ErrPromptNotInProgress, session.ID)
	}
	return m.stageAdmittedSteerPrompt(ctx, session, admissionReq)
}

func (m *Manager) submitAdmittedGoalByTarget(
	ctx context.Context,
	preparation sendPromptPreparation,
	opts SendPromptOpts,
) (SendPromptResult, error) {
	var session *Session
	workspaceID := ""
	if active, ok := m.Get(preparation.request.target); ok {
		session = active
		resolved, err := m.resolvePromptRuntimeAtAdmission(ctx, active, preparation.request.runtime)
		if err != nil {
			return SendPromptResult{}, err
		}
		preparation.request.runtime = resolved
		workspaceID, err = sessionPromptWorkspaceID(active)
		if err != nil {
			return SendPromptResult{}, err
		}
	} else {
		meta, err := m.readMetaWithContext(ctx, preparation.request.target)
		if err != nil {
			return SendPromptResult{}, err
		}
		workspaceID = meta.WorkspaceID
		resolved, err := NormalizeRuntimeSelection(RuntimeSelection{
			Provider: meta.Provider, Model: meta.Model,
			ReasoningEffort: meta.ReasoningEffort, Speed: meta.Speed,
		})
		if err != nil {
			return SendPromptResult{}, err
		}
		preparation.request.runtime = &resolved
	}
	admissionReq, err := m.newPromptAdmissionRequest(
		workspaceID,
		preparation.request,
		store.SessionPromptOperationPrompt,
		preparation.mode,
	)
	if err != nil {
		return SendPromptResult{}, err
	}
	unlock := m.lockPromptAdmission(workspaceID, preparation.request.target, admissionReq.IdempotencyKey)
	defer unlock()
	return m.submitAdmittedGoalPrompt(ctx, session, preparation, opts, admissionReq)
}

func (m *Manager) submitAdmittedPrompt(
	ctx context.Context,
	session *Session,
	preparation sendPromptPreparation,
) (SendPromptResult, error) {
	workspaceID, err := sessionPromptWorkspaceID(session)
	if err != nil {
		return SendPromptResult{}, err
	}
	admissionReq, err := m.newPromptAdmissionRequest(
		workspaceID,
		preparation.request,
		store.SessionPromptOperationPrompt,
		preparation.mode,
	)
	if err != nil {
		return SendPromptResult{}, err
	}
	unlock := m.lockPromptAdmission(workspaceID, session.ID, admissionReq.IdempotencyKey)
	defer unlock()

	if session.IsPrompting() {
		return m.submitAdmittedBusyPrompt(ctx, session, preparation.request, preparation.mode, admissionReq)
	}
	return m.submitAdmittedDirectPrompt(ctx, session, preparation.request, preparation.mode, admissionReq)
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
		Status: promptStatusAccepted, Mode: mode, Events: events,
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
		return m.stageAdmittedSteerPrompt(ctx, session, admissionReq)
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
	admissionReq store.SessionPromptAdmissionRequest,
) (SendPromptResult, error) {
	if m.inputQueue == nil {
		return SendPromptResult{}, ErrPromptInProgress
	}
	generation, err := m.currentInputGeneration(ctx, session.ID)
	if err != nil {
		return SendPromptResult{}, err
	}
	admission, entry, created, err := m.inputQueue.StageAdmittedSteer(ctx, admissionReq, generation)
	if err != nil {
		return SendPromptResult{}, err
	}
	if created {
		m.emitTranscriptMarker(
			ctx, session, session.CurrentTurnID(), transcript.MarkerPromptSteered,
			"Steering input staged while the session is busy.",
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
	admission, replayed, err := m.claimPromptAdmission(ctx, admissionReq)
	if err != nil {
		return SendPromptResult{}, err
	}
	if replayed != nil {
		return *replayed, nil
	}
	req = bindPromptAdmissionRequest(req, admission)
	if err := m.commitPromptAdmissionDispatch(ctx, admission); err != nil {
		return SendPromptResult{}, err
	}
	result, err := m.interruptAndSubmitPrompt(ctx, session, req)
	if err != nil {
		return SendPromptResult{}, m.promptDispatchIndeterminate(ctx, admission, err)
	}
	result.MessageID = admission.MessageID
	result.IdempotencyKey = admission.IdempotencyKey
	return m.completePromptAdmission(ctx, admission, result)
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
		return SendPromptResult{}, m.promptDispatchIndeterminate(
			ctx,
			admission,
			fmt.Errorf("%w: %s", ErrSessionNotActive, preparation.request.target),
		)
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
