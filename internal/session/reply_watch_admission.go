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

// refreshReplyWatchResult reports the persisted watch state on a send receipt. A read failure after
// dispatch must not turn a delivered message into a send error, so it keeps the admission-time state.
func (m *Manager) refreshReplyWatchResult(ctx context.Context, result *SendPromptResult) {
	if result.ReplyWatch == nil {
		return
	}
	watches, ok := m.inputQueueStore.(store.ReplyWatchStore)
	if !ok {
		return
	}
	watch, err := watches.GetReplyWatch(ctx, result.ReplyWatch.ID)
	if err != nil {
		m.logger.WarnContext(ctx, "reply_watch.state_refresh_failed", "id", result.ReplyWatch.ID, "error", err)
		return
	}
	result.ReplyWatch.State = watch.State
}

func (m *Manager) replyWatchSteerResolved(ctx context.Context, sessionID string) {
	if service := m.ReplyWatches(); service != nil {
		service.OnSteerResolved(ctx, sessionID, "")
	}
}

func (m *Manager) replyWatchSessionDeleted(ctx context.Context, sessionID string) {
	if service := m.ReplyWatches(); service != nil {
		service.OnTargetGone(ctx, sessionID)
		if err := service.OnSenderGone(ctx, sessionID); err != nil {
			m.logger.ErrorContext(ctx, "reply_watch.abandon_failed", "session_id", sessionID, "error", err)
		}
	}
}
