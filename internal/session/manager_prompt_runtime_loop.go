package session

import (
	"context"

	"errors"

	"strings"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/transcript"
)

func (m *Manager) pumpPrompt(
	lifecycleCtx context.Context,
	deliveryCtx context.Context,
	session *Session,
	turnState *promptTurnDispatchState,
	source <-chan acp.AgentEvent,
	runtime <-chan acp.AgentEvent,
	out chan<- acp.AgentEvent,
	activity *promptActivitySupervisor,
	releaseExecution context.CancelFunc,
) {
	fatal := &promptPumpFatal{}

	defer func() {
		m.finishPromptPump(
			lifecycleCtx,
			session,
			turnState,
			activity,
			releaseExecution,
			out,
			fatal.failure,
			fatal.errorText,
		)
	}()

	m.runPromptPumpLoop(&promptPumpRun{
		lifecycleCtx: lifecycleCtx,
		deliveryCtx:  deliveryCtx,
		session:      session,
		turnState:    turnState,
		source:       source,
		runtime:      runtime,
		out:          out,
		activity:     activity,
		fatal:        fatal,
	})
}

func (m *Manager) nextPromptPumpEventWithPendingChunk(
	ctx context.Context,
	loop *promptPumpLoopState,
	coalescer *promptChunkCoalescer,
) (acp.AgentEvent, bool, bool, bool) {
	if !coalescer.hasPending() {
		event, runtimeEvent, ok := nextPromptPumpEvent(ctx, loop)
		return event, runtimeEvent, ok, false
	}
	waitCtx, cancel := context.WithTimeout(ctx, promptChunkCoalesceInterval)
	defer cancel()
	event, runtimeEvent, ok := nextPromptPumpEvent(waitCtx, loop)
	if ok {
		return event, runtimeEvent, true, false
	}
	if ctx.Err() != nil {
		return acp.AgentEvent{}, false, false, false
	}
	return acp.AgentEvent{}, false, false, true
}

func (m *Manager) flushPromptChunkCoalescer(
	ctx context.Context,
	deliveryCtx context.Context,
	session *Session,
	turnState *promptTurnDispatchState,
	out chan<- acp.AgentEvent,
	loop *promptPumpLoopState,
	coalescer *promptChunkCoalescer,
) (*store.SessionFailure, string, bool) {
	events, ok := coalescer.take()
	if !ok {
		return nil, "", false
	}
	return m.handlePromptPumpChunkBatch(ctx, deliveryCtx, session, turnState, out, loop, events)
}

func (m *Manager) stopSessionAfterFatalPromptFailure(
	ctx context.Context,
	session *Session,
	failure *store.SessionFailure,
	errorText string,
) {
	if m == nil || session == nil || failure == nil {
		return
	}
	if info := session.Info(); info == nil || info.State != StateActive {
		return
	}

	proc := session.processHandle()
	if proc == nil {
		return
	}

	summary := firstNonEmptySessionFailureText(
		failureSummary(failure, errorText),
		strings.TrimSpace(errorText),
		"agent runtime became unavailable during prompt",
	)
	proc.setWaitErrorOverride(acp.WrapFailure(failure.Kind, summary, errors.New(summary)))

	stopCtx, cancel := detachedPromptStopContext(ctx, m)
	defer cancel()

	if err := m.StopWithCause(stopCtx, session.ID, CauseProcessExited, summary); err != nil &&
		!errors.Is(err, ErrSessionNotFound) {
		m.sessionLogger(session).Warn(
			"session: stop after fatal prompt failure failed",
			"error", err,
			"failure_kind", failure.Kind,
		)
	}
}

func detachedPromptStopContext(ctx context.Context, m *Manager) (context.Context, context.CancelFunc) {
	base := ctx
	if base == nil && m != nil {
		base = m.lifecycleCtx
	}
	if base == nil {
		base = context.TODO()
	}
	return context.WithTimeout(context.WithoutCancel(base), defaultLifecycleTimeout)
}

func nextPromptPumpEvent(
	ctx context.Context,
	loop *promptPumpLoopState,
) (acp.AgentEvent, bool, bool) {
	for {
		if loop.sourceProbeRequired && loop.source != nil {
			select {
			case event, ok := <-loop.source:
				loop.sourceProbeRequired = false
				if !ok {
					if loop.sourceClosedShouldReturn() {
						return acp.AgentEvent{}, false, false
					}
					continue
				}
				return event, false, true
			default:
				loop.sourceProbeRequired = false
			}
		}
		if loop.runtime != nil {
			select {
			case event, ok := <-loop.runtime:
				if !ok {
					if loop.runtimeClosedShouldReturn() {
						return acp.AgentEvent{}, false, false
					}
				} else {
					loop.sourceProbeRequired = true
					return event, true, true
				}
			default:
			}
		}
		select {
		case <-ctx.Done():
			return acp.AgentEvent{}, false, false
		case event, ok := <-loop.source:
			loop.sourceProbeRequired = false
			if !ok {
				if loop.sourceClosedShouldReturn() {
					return acp.AgentEvent{}, false, false
				}
				continue
			}
			return event, false, true
		case event, ok := <-loop.runtime:
			if !ok {
				if loop.runtimeClosedShouldReturn() {
					return acp.AgentEvent{}, false, false
				}
				continue
			}
			loop.sourceProbeRequired = true
			return event, true, true
		}
	}
}

func (m *Manager) handlePromptPumpEvent(
	ctx context.Context,
	deliveryCtx context.Context,
	session *Session,
	turnState *promptTurnDispatchState,
	out chan<- acp.AgentEvent,
	loop *promptPumpLoopState,
	event acp.AgentEvent,
	runtimeEvent bool,
) (*store.SessionFailure, string, bool) {
	if failure, errorText, stop, handled := m.emitPromptDeadlineWarningBeforeError(
		ctx,
		deliveryCtx,
		session,
		turnState,
		out,
		loop,
		event,
		runtimeEvent,
	); handled {
		return failure, errorText, stop
	}

	normalized, skip := m.preparePromptPumpEventForDelivery(ctx, session, turnState, loop, event, runtimeEvent)
	if skip {
		return nil, "", false
	}

	fatalPromptFailure := promptFatalFailure(normalized)
	if err := m.observeRecordAndNotifyPromptEvent(ctx, session, turnState, loop, normalized, runtimeEvent); err != nil {
		failure, errorText := m.promptPersistenceFailure(session, turnState.turnID, err)
		return failure, errorText, true
	}
	if stop := m.sendPromptPumpEvent(ctx, deliveryCtx, out, loop, normalized, runtimeEvent); stop {
		return fatalPromptFailure, normalized.Error, true
	}
	if stop := m.finishPromptTurnIfNeeded(ctx, turnState, loop, normalized); stop {
		return fatalPromptFailure, normalized.Error, true
	}

	return fatalPromptFailure, normalized.Error, false
}

func (m *Manager) emitPromptDeadlineWarningBeforeError(
	ctx context.Context,
	deliveryCtx context.Context,
	session *Session,
	turnState *promptTurnDispatchState,
	out chan<- acp.AgentEvent,
	loop *promptPumpLoopState,
	event acp.AgentEvent,
	runtimeEvent bool,
) (*store.SessionFailure, string, bool, bool) {
	if runtimeEvent || event.Type != acp.EventTypeError || loop.activity == nil {
		return nil, "", false, false
	}

	warning, ok := loop.activity.pendingPromptDeadlineWarning()
	if !ok && strings.Contains(event.Error, "context deadline exceeded") {
		warning, ok = loop.activity.promptDeadlineWarningEvent(m.now())
	}
	if !ok {
		return nil, "", false, false
	}

	failure, errorText, stop := m.handlePromptPumpEvent(
		ctx,
		deliveryCtx,
		session,
		turnState,
		out,
		loop,
		warning,
		true,
	)
	if failure == nil && errorText == "" && !stop {
		return nil, "", false, false
	}
	return failure, errorText, stop, true
}

func (m *Manager) preparePromptPumpEventForDelivery(
	ctx context.Context,
	session *Session,
	turnState *promptTurnDispatchState,
	loop *promptPumpLoopState,
	event acp.AgentEvent,
	runtimeEvent bool,
) (acp.AgentEvent, bool) {
	normalized := m.normalizeEvent(session, turnState.turnID, event)
	if runtimeEvent && loop.activity != nil && loop.activity.shouldSkipDeliveredPromptDeadlineWarning(normalized) {
		return acp.AgentEvent{}, true
	}
	normalized = m.attachPromptFailureDiagnostics(ctx, session, normalized)
	normalized = m.preparePromptEvent(ctx, turnState, normalized)
	normalized = transcript.RedactAgentEvent(normalized)
	return normalized, false
}

func promptFatalFailure(event acp.AgentEvent) *store.SessionFailure {
	if !isFatalPromptFailureEvent(event) {
		return nil
	}
	return store.CloneSessionFailure(event.Failure)
}
