package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

func bindReplyWatchResult(admission store.SessionPromptAdmission, result *SendPromptResult) error {
	result.TargetWorkspaceID = admission.WorkspaceID
	if admission.FingerprintVersion != sessionPromptFingerprintVersion {
		return nil
	}
	origin, err := decodePromptOrigin(admission.Origin)
	if err != nil {
		return err
	}
	if origin != nil && origin.NotifyOnComplete && origin.ReplyWatchID != "" {
		result.ReplyWatch = &ReplyWatchRef{ID: origin.ReplyWatchID, State: store.ReplyWatchArmed}
	}
	return nil
}

func (m *Manager) replyWatchSendResult(ctx context.Context, admission store.SessionPromptAdmission, sendErr error) {
	service := m.ReplyWatches()
	if service == nil {
		return
	}
	var result SendPromptResult
	if err := bindReplyWatchResult(admission, &result); err != nil || result.ReplyWatch == nil {
		return
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultLifecycleTimeout)
	defer cancel()
	service.OnSendResult(cleanupCtx, result.ReplyWatch.ID, sendErr)
}

func (m *Manager) completePromptAdmission(
	ctx context.Context,
	admission store.SessionPromptAdmission,
	result SendPromptResult,
) (SendPromptResult, error) {
	if err := bindReplyWatchResult(admission, &result); err != nil {
		return SendPromptResult{}, err
	}
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
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultLifecycleTimeout)
	defer cancel()
	reason := "dispatch failed after the at-most-once boundary: " + cause.Error()
	markErr := m.promptAdmissionStore.MarkSessionPromptAdmissionIndeterminate(
		cleanupCtx,
		admission.WorkspaceID,
		admission.SessionID,
		admission.IdempotencyKey,
		reason,
		m.now(),
	)
	indeterminate := fmt.Errorf("%w: %s", store.ErrSessionPromptDispatchIndeterminate, reason)
	return errors.Join(indeterminate, markErr)
}
