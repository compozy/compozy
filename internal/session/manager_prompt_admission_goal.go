package session

import (
	"context"

	"github.com/compozy/compozy/internal/store"
)

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
	preparation.request, err = bindPromptAdmissionRequest(preparation.request, admission)
	if err != nil {
		return SendPromptResult{}, err
	}
	if session != nil {
		if err := m.preflightAdmittedPromptAttachments(
			ctx,
			session,
			preparation.request.runtime,
			preparation.request.attachments,
		); err != nil {
			return SendPromptResult{}, err
		}
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
	// Control-only Goal commands must not implicitly resume a stopped session.
	// Only a command that dispatches a prompt (Draft) needs a live runtime.
	if session == nil {
		session, err = m.lookupPromptRequestSession(ctx, preparation.request)
		if err != nil {
			return SendPromptResult{}, m.promptDispatchIndeterminate(ctx, admission, err)
		}
		if err := m.preflightAdmittedPromptAttachments(ctx, session,
			preparation.request.runtime, preparation.request.attachments); err != nil {
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
