package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	eventspkg "github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	toolspkg "github.com/compozy/agh/internal/tools"
)

type clarifySummaryWriter interface {
	WriteEventSummary(context.Context, store.EventSummary) error
}

func (b *clarifyBridge) completeExplicit(
	ctx context.Context,
	handle *clarifyHandle,
	answer toolspkg.ClarifyAnswer,
) error {
	handle.completionMu.Lock()
	defer handle.completionMu.Unlock()
	if handle.terminal || !handle.ready.Load() {
		return fmt.Errorf("%w: %s", toolspkg.ErrClarifyNotFound, handle.pending.RequestID)
	}
	if err := b.publish(ctx, handle, toolspkg.ClarifyStatusResolved, &answer, true); err != nil {
		return fmt.Errorf("daemon: persist resolved clarification: %w", err)
	}
	b.release(handle, clarifyResult{answer: answer})
	return nil
}

func (b *clarifyBridge) completeAutomatic(
	handle *clarifyHandle,
	status toolspkg.ClarifyStatus,
	answer toolspkg.ClarifyAnswer,
	resultErr error,
) {
	b.completeAutomaticWithContext(context.Background(), handle, status, answer, resultErr)
}

func (b *clarifyBridge) completeAutomaticWithContext(
	parent context.Context,
	handle *clarifyHandle,
	status toolspkg.ClarifyStatus,
	answer toolspkg.ClarifyAnswer,
	resultErr error,
) {
	handle.completionMu.Lock()
	defer handle.completionMu.Unlock()
	if handle.terminal {
		return
	}
	ctx, cancel := context.WithTimeout(parent, clarifyObservabilityTimeout)
	defer cancel()
	var eventAnswer *toolspkg.ClarifyAnswer
	if status == toolspkg.ClarifyStatusTimedOut {
		eventAnswer = &answer
	}
	if err := b.publish(ctx, handle, status, eventAnswer, false); err != nil {
		b.logger.WarnContext(
			ctx,
			"clarification terminal observability failed",
			"request_id", handle.pending.RequestID,
			"session_id", handle.pending.SessionID,
			"status", status,
			"error", err,
		)
	}
	b.release(handle, clarifyResult{answer: answer, err: resultErr})
}

func (b *clarifyBridge) release(handle *clarifyHandle, result clarifyResult) {
	b.mu.Lock()
	if b.pending[handle.pending.SessionID] == handle {
		delete(b.pending, handle.pending.SessionID)
	}
	handle.terminal = true
	b.mu.Unlock()
	handle.result <- result
}

func (b *clarifyBridge) publish(
	ctx context.Context,
	handle *clarifyHandle,
	status toolspkg.ClarifyStatus,
	answer *toolspkg.ClarifyAnswer,
	requireEvent bool,
) error {
	event := toolspkg.ClarifyEvent{
		Status:  status,
		Request: handle.pending.Clone(),
		Answer:  cloneClarifyAnswer(answer),
		At:      b.now().UTC(),
	}
	if err := event.Validate(); err != nil {
		return fmt.Errorf("validate clarification event: %w", err)
	}
	if err := b.publisher.PublishClarifyEvent(ctx, event); err != nil {
		if requireEvent {
			return err
		}
		return fmt.Errorf("publish clarification session event: %w", err)
	}
	if b.summaries == nil {
		return nil
	}
	if err := b.writeSummary(ctx, event); err != nil {
		b.logger.WarnContext(
			ctx,
			"clarification lifecycle summary failed",
			"request_id", handle.pending.RequestID,
			"session_id", handle.pending.SessionID,
			"status", status,
			"error", err,
		)
	}
	return nil
}

func (b *clarifyBridge) writeSummary(ctx context.Context, event toolspkg.ClarifyEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal clarification summary: %w", err)
	}
	return b.summaries.WriteEventSummary(ctx, store.EventSummary{
		ID:          event.Request.RequestID + "-" + string(event.Status),
		SessionID:   event.Request.SessionID,
		WorkspaceID: event.Request.WorkspaceID,
		Type:        "clarify." + clarifySummarySuffix(event.Status),
		AgentName:   event.Request.AgentName,
		Outcome:     clarifySummaryOutcome(event.Status),
		Content:     payload,
		Summary:     clarifySummaryText(event.Status, event.Request.RequestID),
		Timestamp:   event.At,
	})
}

func clarifySummarySuffix(status toolspkg.ClarifyStatus) string {
	if status == toolspkg.ClarifyStatusPending {
		return "asked"
	}
	return string(status)
}

func clarifySummaryOutcome(status toolspkg.ClarifyStatus) string {
	switch status {
	case toolspkg.ClarifyStatusResolved:
		return string(eventspkg.OutcomeSuccess)
	case toolspkg.ClarifyStatusTimedOut, toolspkg.ClarifyStatusCanceled:
		return string(eventspkg.OutcomeWarning)
	default:
		return string(eventspkg.OutcomeInfo)
	}
}

func clarifySummaryText(status toolspkg.ClarifyStatus, requestID string) string {
	return fmt.Sprintf("Clarification %s (%s).", strings.ReplaceAll(string(status), "_", " "), requestID)
}

func cloneClarifyAnswer(answer *toolspkg.ClarifyAnswer) *toolspkg.ClarifyAnswer {
	if answer == nil {
		return nil
	}
	cloned := *answer
	if answer.Choice != nil {
		choice := *answer.Choice
		cloned.Choice = &choice
	}
	return &cloned
}

func (b *clarifyBridge) CancelSession(sessionID string) {
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return
	}
	b.mu.Lock()
	handle := b.pending[target]
	b.mu.Unlock()
	if handle != nil {
		b.completeAutomatic(
			handle,
			toolspkg.ClarifyStatusCanceled,
			toolspkg.ClarifyAnswer{},
			toolspkg.ErrClarifyCanceled,
		)
	}
}

func (b *clarifyBridge) Close(ctx context.Context) error {
	if ctx == nil {
		return errors.New("daemon: clarification close context is required")
	}
	b.mu.Lock()
	b.closed = true
	handles := make([]*clarifyHandle, 0, len(b.pending))
	for _, handle := range b.pending {
		handles = append(handles, handle)
	}
	b.mu.Unlock()
	for _, handle := range handles {
		b.completeAutomaticWithContext(
			ctx,
			handle,
			toolspkg.ClarifyStatusCanceled,
			toolspkg.ClarifyAnswer{},
			toolspkg.ErrClarifyCanceled,
		)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("daemon: wait for clarification callers: %w", err)
	}
	waitersDone := make(chan struct{})
	go func() {
		b.waiters.Wait()
		close(waitersDone)
	}()
	select {
	case <-waitersDone:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("daemon: wait for clarification callers: %w", ctx.Err())
	}
}

func (b *clarifyBridge) OnSessionCreated(context.Context, *session.Session) {}

func (b *clarifyBridge) OnSessionFinalizing(_ context.Context, stopping *session.Session) {
	if stopping != nil {
		b.CancelSession(stopping.ID)
	}
}

func (b *clarifyBridge) OnSessionStopped(_ context.Context, stopped *session.Session) {
	if stopped != nil {
		b.CancelSession(stopped.ID)
	}
}

var _ sessionLifecycleObserver = (*clarifyBridge)(nil)
var _ sessionFinalizationObserver = (*clarifyBridge)(nil)
