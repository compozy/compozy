package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type clarifySummaryWriter interface {
	WriteEventSummary(context.Context, store.EventSummary) error
}

type clarifyAnswerPublisher interface {
	PublishClarifyAnswer(context.Context, toolspkg.ClarifyEvent, toolspkg.Scope) (func(), error)
}

var _ clarifyAnswerPublisher = (*session.Manager)(nil)

type clarifyOrphanResolver interface {
	ResolveOrphanedClarification(
		context.Context,
		toolspkg.Scope,
		string,
		toolspkg.ClarifyAnswerRequest,
	) (toolspkg.ClarifyAnswerResult, error)
}

func (b *clarifyBridge) resolveOrphaned(
	ctx context.Context,
	scope toolspkg.Scope,
	requestID string,
	request toolspkg.ClarifyAnswerRequest,
) (toolspkg.ClarifyAnswerResult, error) {
	resolver, ok := b.publisher.(clarifyOrphanResolver)
	if !ok {
		return toolspkg.ClarifyAnswerResult{}, fmt.Errorf("%w: %s", toolspkg.ErrClarifyNotFound, requestID)
	}
	return resolver.ResolveOrphanedClarification(ctx, scope, requestID, request)
}

func (b *clarifyBridge) completeExplicit(
	ctx context.Context,
	handle *clarifyHandle,
	answer toolspkg.ClarifyAnswer,
) (func(), error) {
	handle.completionMu.Lock()
	defer handle.completionMu.Unlock()
	if handle.terminal || !handle.ready.Load() {
		return nil, fmt.Errorf("%w: %s", toolspkg.ErrClarifyNotFound, handle.pending.RequestID)
	}
	wake, err := b.publish(ctx, handle, toolspkg.ClarifyStatusResolved, &answer)
	if err != nil {
		return nil, fmt.Errorf("daemon: persist resolved clarification: %w", err)
	}
	b.release(handle, clarifyResult{answer: answer})
	return wake, nil
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
	if _, err := b.publish(ctx, handle, status, eventAnswer); err != nil {
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
	// Terminal debug line carries the last ping seq for correlation; debug
	// only, since unbounded waits ping for hours.
	b.logger.DebugContext(
		context.Background(),
		"clarification terminal transition",
		"session_id", handle.pending.SessionID,
		"request_id", handle.pending.RequestID,
		"last_ping_seq", handle.pingSeq,
	)
	handle.result <- result
}

func (b *clarifyBridge) publish(
	ctx context.Context,
	handle *clarifyHandle,
	status toolspkg.ClarifyStatus,
	answer *toolspkg.ClarifyAnswer,
) (func(), error) {
	event := toolspkg.ClarifyEvent{
		Status:  status,
		Request: handle.pending.Clone(),
		Answer:  cloneClarifyAnswer(answer),
		At:      b.now().UTC(),
	}
	if err := event.Validate(); err != nil {
		return nil, fmt.Errorf("validate clarification event: %w", err)
	}
	var wake func()
	publisher, canResume := b.publisher.(clarifyAnswerPublisher)
	if status == toolspkg.ClarifyStatusResolved && handle.askingScope.RunID != "" && canResume {
		var err error
		wake, err = publisher.PublishClarifyAnswer(ctx, event, handle.askingScope)
		if err != nil {
			return nil, err
		}
	} else if err := b.publisher.PublishClarifyEvent(ctx, event); err != nil {
		return nil, err
	}
	if b.summaries == nil {
		return wake, nil
	}
	if err := b.writeSummary(ctx, handle, event); err != nil {
		b.logger.WarnContext(
			ctx,
			"clarification lifecycle summary failed",
			"request_id", handle.pending.RequestID,
			"session_id", handle.pending.SessionID,
			"status", status,
			"error", err,
		)
	}
	return wake, nil
}

func (b *clarifyBridge) writeSummary(
	ctx context.Context,
	handle *clarifyHandle,
	event toolspkg.ClarifyEvent,
) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal clarification summary: %w", err)
	}
	profileID := ""
	if handle != nil {
		profileID = handle.askingScope.ProfileID
	}
	if profileID == "" && b.profileForSession != nil {
		profileID, err = b.profileForSession(ctx, event.Request.SessionID)
		if err != nil {
			return fmt.Errorf("resolve clarification session profile: %w", err)
		}
	}
	if profileID == "" {
		return errors.New("daemon: clarification summary profile is required")
	}
	return b.summaries.WriteEventSummary(ctx, daemonEventSummary(store.EventSummary{
		ProfileID:   profileID,
		ID:          event.Request.RequestID + "-" + string(event.Status),
		SessionID:   event.Request.SessionID,
		WorkspaceID: event.Request.WorkspaceID,
		Type:        "clarify." + clarifySummarySuffix(event.Status),
		AgentName:   event.Request.AgentName,
		Outcome:     clarifySummaryOutcome(event.Status),
		Summary:     clarifySummaryText(event.Status, event.Request.RequestID),
		Timestamp:   event.At,
	}, payload))
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
		cloned.Choice = new(*answer.Choice)
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
