package session

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) submitAdmittedGoalPrompt(
	ctx context.Context,
	session *Session,
	preparation sendPromptPreparation,
	opts SendPromptOpts,
	admissionReq store.SessionPromptAdmissionRequest,
) (result SendPromptResult, dispatchErr error) {
	command, matched, parseErr := ParseGoalCommand(preparation.request.authoredMessage)
	if matched && (parseErr != nil || command.Verb != GoalCommandVerbDraft) {
		ctx = store.WithPromptControlOnly(ctx)
	}
	admission, replayed, err := m.claimPromptAdmission(ctx, admissionReq)
	if err != nil {
		return SendPromptResult{}, err
	}
	if replayed != nil {
		return *replayed, nil
	}
	committed := false
	defer func() {
		if dispatchErr != nil && !committed {
			dispatchErr = errors.Join(dispatchErr, m.releaseUndispatchedReports(ctx, admission))
		}
	}()
	if store.PromptControlOnly(ctx) {
		if err := m.releaseUndispatchedReports(ctx, admission); err != nil {
			return SendPromptResult{}, err
		}
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
	committed = true
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
	if store.PromptControlOnly(ctx) {
		return SendPromptResult{}, m.promptDispatchIndeterminate(ctx, admission,
			errors.New("session: control-only Goal command requested provider dispatch"))
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
	result, err = m.submitPreparedPrompt(ctx, session, preparation)
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
	commit := func(commitCtx context.Context) error {
		return m.promptAdmissionStore.CommitSessionPromptDispatch(
			commitCtx, admission.WorkspaceID, admission.SessionID,
			admission.IdempotencyKey, m.now(),
		)
	}
	var err error
	if store.PromptControlOnly(ctx) || (admission.HumanIngress && !admission.AppIngress) {
		err = commit(ctx)
	} else {
		err = m.withDailyBudgetAdmission(ctx, admission.SessionID, commit)
	}
	if blocked, ok := errors.AsType[*store.DailyBudgetTurnBlockedError](err); ok &&
		blocked.PausedInstance != nil && m.instancePaused != nil {
		if pauseErr := m.instancePaused(m.fallbackLifecycleContext(), *blocked.PausedInstance); pauseErr != nil {
			m.sessionLogger(nil).ErrorContext(ctx, "session: apply daily budget pause effects",
				"instance_id", blocked.PausedInstance.InstanceID, "error", pauseErr)
		}
	}
	return err
}
