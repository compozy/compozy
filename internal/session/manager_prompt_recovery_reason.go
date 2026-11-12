package session

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

// authRecoveryContinueMessage resumes a turn whose provider sign-in dropped after it had started
// work; the reloaded provider session already holds that work, so resending the request would repeat it.
const authRecoveryContinueMessage = "Your previous turn stopped because the provider sign-in was " +
	"briefly unavailable. It is available again: continue the interrupted work. Verify the current " +
	"state before repeating actions: a tool call without a recorded result may already have completed. " +
	"Do not assume that an interrupted action had no effect."

// promptRecoveryReasonFor maps a turn failure to the automatic recovery that can wait it out.
func promptRecoveryReasonFor(event acp.AgentEvent) (store.SessionRuntimeRecoveryReason, bool) {
	if event.Type != acp.EventTypeError || event.Failure == nil {
		return "", false
	}
	switch event.Failure.Kind {
	case store.FailureProcess, store.FailureTransport:
		return store.SessionRuntimeRecoveryRuntimeExit, true
	case store.FailurePrompt:
		if event.ProviderError != nil && event.ProviderError.Code == acp.ProviderErrorAuthRequired &&
			event.ProviderError.NextAction == acp.ProviderFailureActionLogin {
			return store.SessionRuntimeRecoveryProviderAuth, true
		}
	}
	return "", false
}

// notePromptEventDelivered tracks provider work for the recovery payload choice and re-arms the
// sign-in wait once a turn completes.
func notePromptEventDelivered(session *Session, turnState *promptTurnDispatchState, event acp.AgentEvent) {
	turnState.recovery.observeDelivered(event.Type)
	if event.Type == acp.EventTypeDone && event.StopReason != string(acp.PromptStopReasonCancelled) {
		session.setAuthRecoveryExhausted(false)
	}
}

func (m *Manager) promptRecoverySchedule(reason store.SessionRuntimeRecoveryReason) []time.Duration {
	if reason == store.SessionRuntimeRecoveryProviderAuth {
		return m.authRecoveryDelays
	}
	return m.promptRecoveryDelays
}

func (b *promptRecoveryBudget) remainingDelay(now time.Time, delay time.Duration) (time.Duration, bool) {
	if b.expiresAt.IsZero() {
		return delay, true
	}
	remaining := b.expiresAt.Sub(now)
	return min(delay, remaining), remaining > 0
}

// promptRecoveryAvailable reports whether this failure class may start or continue recovering.
func promptRecoveryAvailable(session *Session, reason store.SessionRuntimeRecoveryReason) bool {
	return reason != store.SessionRuntimeRecoveryProviderAuth || session.authRecoveryAvailable()
}

// recoveredPromptPayload chooses what the replacement runtime receives: the original request
// while the provider has not acted on the turn, or a continuation once a lost sign-in
// interrupted work the reloaded provider session already holds.
func recoveredPromptPayload(
	recovery *promptRecoveryState,
	reason store.SessionRuntimeRecoveryReason,
) (acp.PromptRequest, string) {
	request := clonePromptRecoveryRequest(recovery.request)
	if reason != store.SessionRuntimeRecoveryProviderAuth || !recovery.workObserved || request.BypassesContext() {
		return request, recovery.originalMessage
	}
	request.MentionLinks = nil
	return request, authRecoveryContinueMessage
}

func (m *Manager) recoverPromptRuntimeBefore(
	ctx context.Context,
	session *Session,
	expiresAt time.Time,
) (*AgentProcess, string, error) {
	if !expiresAt.IsZero() {
		remaining := expiresAt.Sub(m.now())
		if remaining <= 0 {
			return nil, "", context.DeadlineExceeded
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, remaining)
		defer cancel()
	}
	return m.recoverPromptRuntimeWithLoad(ctx, session, !expiresAt.IsZero())
}

// settleStoppedPromptRecovery leaves a non-fatal failure on the runtime the session kept; a
// runtime-exit failure is finalized by the fatal prompt path instead.
func (m *Manager) settleStoppedPromptRecovery(
	ctx context.Context,
	session *Session,
	reason store.SessionRuntimeRecoveryReason,
	terminal acp.AgentEvent,
) error {
	canceled := terminal.Type == acp.EventTypeDone && terminal.StopReason == string(acp.PromptStopReasonCancelled)
	if reason != store.SessionRuntimeRecoveryProviderAuth && !canceled || !session.abandonAutomaticRecovery(m.now()) {
		return nil
	}
	// Fatal replacement failures keep the dead binding for stop finalization.
	if canceled || terminal.ProviderError != nil && terminal.ProviderError.Code == acp.ProviderErrorAuthRequired {
		if proc := session.processHandle(); proc != nil && isProcessDone(proc) {
			return m.releaseBoundRuntime(ctx, session)
		}
	}
	return m.persistSessionLifecycleState(ctx, session, false)
}

func (m *Manager) maxPromptRecoveryWait() time.Duration {
	var total time.Duration
	for _, delay := range m.promptRecoveryDelays {
		total += delay + defaultLifecycleTimeout
	}
	if len(m.authRecoveryDelays) > 0 {
		total += m.authRecoveryWindow
	}
	return total
}

func (m *Manager) recoverPromptPumpEvent(
	ctx context.Context,
	session *Session,
	state *promptTurnDispatchState,
	loop *promptPumpLoopState,
	event acp.AgentEvent,
) (bool, *acp.AgentEvent, error) {
	event = promptErrorForExitedProcess(session.processHandle(), m.normalizeEvent(session, state.turnID, event))
	if event.Type == acp.EventTypeError {
		if err := m.failDispatchedReports(ctx, session, state); err != nil {
			return false, nil, err
		}
	}
	reason, ok := promptRecoveryReasonFor(event)
	if !ok || !promptRecoveryAllowed(loop) || m.daemonShuttingDown() {
		return false, nil, nil
	}
	return m.attemptPromptRecovery(ctx, session, state, loop, event, reason)
}

func runtimeRecoveryNeedsSignIn(reason store.SessionRuntimeRecoveryReason, err error) bool {
	return reason != store.SessionRuntimeRecoveryProviderAuth && recoveryNeedsSignIn(err)
}

func (m *Manager) stoppedPromptRecovery(
	ctx context.Context, session *Session, state *promptTurnDispatchState, trigger acp.AgentEvent,
	reason store.SessionRuntimeRecoveryReason,
) (bool, *acp.AgentEvent, error) {
	if session.promptCancellationRequested(state.turnID) || state.recovery.executionCtx.Err() != nil ||
		m.daemonShuttingDown() {
		trigger = promptCancellationTerminalEvent()
	}
	return false, &trigger, m.settleStoppedPromptRecovery(ctx, session, reason, trigger)
}

func (m *Manager) publishPromptRecoveryAttempt(
	ctx context.Context, session *Session, state *promptTurnDispatchState, loop *promptPumpLoopState,
	trigger acp.AgentEvent, attempt automaticRecoveryAttempt,
) error {
	if err := session.beginAutomaticRecovery(attempt, m.now()); err != nil {
		return err
	}
	if err := m.persistSessionLifecycleState(ctx, session, false); err != nil {
		return err
	}
	return m.recordPromptRecoveryEvent(ctx, session, state, loop, acp.EventTypeRuntimeRecoveryStarted,
		trigger, attempt.reason, attempt.attempt, attempt.maxAttempts)
}
