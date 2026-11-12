package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

type promptRecoveryEventData struct {
	Reason        string `json:"reason,omitempty"`
	Attempt       int    `json:"attempt"`
	MaxAttempts   int    `json:"max_attempts"`
	Generation    int64  `json:"generation"`
	FailureKind   string `json:"failure_kind,omitempty"`
	FailureDetail string `json:"failure_detail,omitempty"`
}

func clonePromptRecoveryRequest(request acp.PromptRequest) acp.PromptRequest {
	cloned := request
	cloned.Sections = slices.Clone(request.Sections)
	cloned.MentionLinks = slices.Clone(request.MentionLinks)
	cloned.Attachments = make([]acp.PromptAttachment, len(request.Attachments))
	for index, attachment := range request.Attachments {
		cloned.Attachments[index] = attachment
		cloned.Attachments[index].Data = append([]byte(nil), attachment.Data...)
	}
	return cloned
}

// attemptPromptRecovery spends the existing bounded recovery schedule, waking early on verified sign-in.
func (m *Manager) attemptPromptRecovery(
	ctx context.Context,
	session *Session,
	turnState *promptTurnDispatchState,
	loop *promptPumpLoopState,
	trigger acp.AgentEvent,
	reason store.SessionRuntimeRecoveryReason,
) (bool, *acp.AgentEvent, error) {
	if m == nil || session == nil || turnState == nil || turnState.recovery == nil {
		return false, nil, nil
	}
	recovery := turnState.recovery
	terminal := trigger
	delays, budget, ok := m.startablePromptRecovery(session, turnState, reason)
	if !ok {
		return m.stoppedPromptRecovery(ctx, session, turnState, trigger, reason)
	}
	maxAttempts := len(delays)
	for budget.attempts < maxAttempts {
		attempt := budget.attempts + 1
		now := m.now()
		delay, withinWindow := budget.remainingDelay(now, delays[budget.attempts])
		if !withinWindow {
			break
		}
		budget.attempts = attempt
		if err := m.publishPromptRecoveryAttempt(ctx, session, turnState, loop, trigger, automaticRecoveryAttempt{
			reason: reason, attempt: attempt, maxAttempts: maxAttempts,
			startedAt: budget.startedAt, nextAttemptAt: now.Add(delay),
		}); err != nil {
			return false, nil, err
		}
		// A stop can claim the turn between startablePromptRecovery and publishing
		// the recovery state. In that case it did not yet see a daemon-owned wait.
		// Daemon shutdown may begin during the delay; it must not spawn a replacement runtime.
		if !m.waitForPromptRecovery(session, turnState, delay) {
			return m.stoppedPromptRecovery(ctx, session, turnState, trigger, reason)
		}
		if !budget.expiresAt.IsZero() && !m.now().Before(budget.expiresAt) {
			break
		}

		request, message := recoveredPromptPayload(recovery, reason)
		source, _, err := m.startRecoveredPrompt(
			recovery.executionCtx,
			session,
			turnState,
			request,
			message,
			budget.expiresAt,
		)
		if err != nil {
			if errors.Is(err, ErrDaemonShuttingDown) {
				// A spawn the shutdown refused is the interruption, not a spent recovery attempt.
				return m.stoppedPromptRecovery(ctx, session, turnState, trigger, reason)
			}
			if persistErr := m.persistPromptRecoveryFailure(ctx, session, err); persistErr != nil {
				return false, nil, persistErr
			}
			if runtimeRecoveryNeedsSignIn(reason, err) {
				return m.attemptPromptRecovery(ctx, session, turnState, loop,
					m.recoveryFailureEvent(session, turnState, err), store.SessionRuntimeRecoveryProviderAuth)
			}
			terminal = m.recoveryFailureEvent(session, turnState, err)
			continue
		}
		loop.source = source
		loop.turnEnded = false
		loop.sourceProbeRequired = false
		if err := m.recordPromptRecoveryEvent(
			ctx, session, turnState, loop, acp.EventTypeRuntimeRecoverySucceeded, trigger, reason, attempt, maxAttempts,
		); err != nil {
			return false, nil, err
		}
		return true, nil, nil
	}

	return false, &terminal, m.exhaustPromptRecovery(
		ctx,
		session,
		turnState,
		loop,
		terminal,
		reason,
		budget,
		maxAttempts,
	)
}

// startablePromptRecovery returns the schedule and budget a turn failure may still spend.
func (m *Manager) startablePromptRecovery(
	session *Session,
	turnState *promptTurnDispatchState,
	reason store.SessionRuntimeRecoveryReason,
) ([]time.Duration, *promptRecoveryBudget, bool) {
	recovery := turnState.recovery
	if recovery.executionCtx == nil || recovery.executionCtx.Err() != nil ||
		session.promptCancellationRequested(turnState.turnID) {
		return nil, nil, false
	}
	delays := m.promptRecoverySchedule(reason)
	budget := recovery.budget(reason)
	if len(delays) == 0 || budget.exhaustedRecorded || !promptRecoveryAvailable(session, reason) {
		return nil, nil, false
	}
	if budget.startedAt.IsZero() {
		budget.startedAt = m.now()
		if reason == store.SessionRuntimeRecoveryProviderAuth {
			budget.expiresAt = budget.startedAt.Add(m.authRecoveryWindow)
		}
	}
	return delays, budget, true
}

func (m *Manager) exhaustPromptRecovery(
	ctx context.Context,
	session *Session,
	turnState *promptTurnDispatchState,
	loop *promptPumpLoopState,
	trigger acp.AgentEvent,
	reason store.SessionRuntimeRecoveryReason,
	budget *promptRecoveryBudget,
	maxAttempts int,
) error {
	if err := m.recordPromptRecoveryEvent(
		ctx,
		session,
		turnState,
		loop,
		acp.EventTypeRuntimeRecoveryExhausted,
		trigger,
		reason,
		budget.attempts,
		maxAttempts,
	); err != nil {
		return err
	}
	budget.exhaustedRecorded = true
	if reason == store.SessionRuntimeRecoveryProviderAuth {
		session.setAuthRecoveryExhausted(true)
	}
	return m.settleStoppedPromptRecovery(ctx, session, reason, trigger)
}

func (m *Manager) persistPromptRecoveryFailure(ctx context.Context, session *Session, recoveryErr error) error {
	session.recordAutomaticRecoveryFailure(recoveryErr, m.now())
	if err := m.persistSessionLifecycleState(ctx, session, false); err != nil {
		return errors.Join(recoveryErr, err)
	}
	return nil
}

func (m *Manager) startRecoveredPrompt(
	ctx context.Context,
	session *Session,
	state *promptTurnDispatchState,
	request acp.PromptRequest,
	message string,
	expiresAt time.Time,
) (<-chan acp.AgentEvent, string, error) {
	m.releaseHostedPromptRun(session, state)
	m.clearActivePromptRunForState(session, state)
	process, replayBlock, err := m.recoverPromptRuntimeBefore(ctx, session, expiresAt)
	if err != nil {
		return nil, "", err
	}
	if !expiresAt.IsZero() && !m.now().Before(expiresAt) {
		return nil, "", context.DeadlineExceeded
	}
	replayRequest := clonePromptRecoveryRequest(request)
	replayRequest.Generation = session.Info().RuntimeGeneration
	if !replayRequest.BypassesContext() {
		replayRequest.Message = promptWithResumeReplay(replayBlock, message)
	}
	source, err := m.startHostedPromptRun(ctx, session, process, state, replayRequest)
	if err != nil {
		return nil, "", err
	}
	if !request.BypassesContext() {
		m.consumeResumeReplay(session.ID, replayBlock)
	}
	return source, replayBlock, nil
}

// waitForPromptRecoverySignal waits for backoff, verified credentials, or cancellation.
func waitForPromptRecoverySignal(ctx context.Context, delay time.Duration, ready <-chan struct{}) error {
	if delay <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	case <-ready:
		return nil
	}
}

func (m *Manager) recordPromptRecoveryEvent(
	ctx context.Context,
	session *Session,
	turnState *promptTurnDispatchState,
	loop *promptPumpLoopState,
	eventType string,
	trigger acp.AgentEvent,
	reason store.SessionRuntimeRecoveryReason,
	attempt int,
	maxAttempts int,
) error {
	failureKind := ""
	failureDetail := strings.TrimSpace(trigger.Error)
	if trigger.Failure != nil {
		failureKind = string(trigger.Failure.Kind)
		failureDetail = firstTrimmedNonEmpty(trigger.Failure.Summary, failureDetail)
	}
	info := session.Info()
	generation := info.RuntimeGeneration
	if info.RuntimeRecovery != nil {
		generation = info.RuntimeRecovery.Generation
	}
	data, err := json.Marshal(promptRecoveryEventData{
		Reason: string(reason), Attempt: attempt, MaxAttempts: maxAttempts, Generation: generation,
		FailureKind: failureKind, FailureDetail: failureDetail,
	})
	if err != nil {
		return fmt.Errorf("session: marshal prompt recovery event: %w", err)
	}
	event := m.normalizeEvent(session, turnState.turnID, acp.AgentEvent{
		Type: eventType, Timestamp: m.now(), Raw: data,
	})
	event = m.preparePromptEvent(ctx, turnState, event)
	event = transcript.RedactAgentEvent(event)
	if err := m.observeRecordAndNotifyPromptEvent(ctx, session, turnState, loop, event, true); err != nil {
		if m.discardPromptEventAfterStop(ctx, session, turnState.turnID, event.Type, err) {
			return nil
		}
		return err
	}
	m.dispatchRuntimeRecoveryObservation(
		ctx,
		session,
		turnState.turnID,
		turnState.runID,
		recoveryHookEvent(eventType),
		attempt,
		maxAttempts,
		generation,
		failureKind,
		failureDetail,
	)
	return nil
}

func recoveryHookEvent(eventType string) hookspkg.HookEvent {
	switch eventType {
	case acp.EventTypeRuntimeRecoveryStarted:
		return hookspkg.HookSessionRuntimeRecoveryStarted
	case acp.EventTypeRuntimeRecoverySucceeded:
		return hookspkg.HookSessionRuntimeRecoverySucceeded
	case acp.EventTypeRuntimeRecoveryExhausted:
		return hookspkg.HookSessionRuntimeRecoveryExhausted
	default:
		return ""
	}
}

func (m *Manager) recoverPromptRuntime(
	ctx context.Context,
	session *Session,
) (*AgentProcess, string, error) {
	return m.recoverPromptRuntimeWithLoad(ctx, session, false)
}

func (m *Manager) recoverPromptRuntimeWithLoad(
	ctx context.Context,
	session *Session,
	loadSession bool,
) (*AgentProcess, string, error) {
	snapshot := session.runtimeBindingSnapshot()
	// Recovery keeps the accepted seat: the accepted route's explicit command, if any.
	plan, err := m.preparePromptRuntimePlanForRoute(
		ctx, session, snapshot.selection, snapshot.acceptedCommand, snapshot.acceptedSessionEnv,
	)
	if err != nil {
		return nil, "", err
	}
	runtime := plan.runtime
	runtime.mcpServers, err = m.sessionMCPServers(ctx, &plan.spec, runtime.agent, runtime.agentDef)
	if err != nil {
		return nil, "", fmt.Errorf("session: resolve recovery MCP servers: %w", err)
	}
	plan.spec.resumeReplay = true
	if loadSession {
		plan.spec.acpSessionID = snapshot.acpSessionID
	}
	plan.spec.startAction = "automatic recovery"

	candidate, err := m.startPromptRecoveryCandidate(ctx, session, plan, &runtime)
	if err != nil {
		return nil, "", err
	}
	previous, err := session.completeRecoveryTransition(
		snapshot.process,
		candidate,
		plan.selection,
		runtime.agentDef,
		m.now(),
	)
	if err != nil {
		return nil, "", errors.Join(err, m.stopReplacedRuntime(session, candidate, false))
	}
	attempt := 0
	if snapshot.acceptedRoute != nil {
		attempt = snapshot.acceptedRoute.Attempt
	}
	session.commitAcceptedRoute(
		acceptedRouteRecord(attempt, runtime.agent, plan.selection.Model), plan.spec.command, plan.spec.sessionEnv,
	)
	session.setPendingResumeReplay(plan.spec.resumeReplayBlock)
	if err := m.persistSessionLifecycleState(ctx, session, false); err != nil {
		session.restoreRecoveryBinding(candidate, &snapshot, err.Error(), m.now())
		stopErr := m.stopReplacedRuntime(session, candidate, false)
		return nil, "", errors.Join(err, stopErr)
	}

	m.stageResumeReplay(session.ID, plan.spec.resumeReplayBlock)
	if previous != nil && previous != candidate {
		if stopErr := m.stopReplacedRuntime(session, previous, true); stopErr != nil && !isProcessDone(previous) {
			m.sessionLogger(session).Warn("session: stop recovered runtime generation failed", "error", stopErr)
		}
	}
	m.dispatchAgentSpawned(ctx, session, candidate, runtime.agent)
	m.watchProcess(session)
	return candidate, plan.spec.resumeReplayBlock, nil
}

func (m *Manager) startPromptRecoveryCandidate(
	ctx context.Context,
	session *Session,
	plan *promptRuntimePlan,
	runtime *sessionStartRuntime,
) (*AgentProcess, error) {
	startOpts, err := m.prepareSessionLaunch(ctx, &plan.spec, session, runtime, nil)
	if err != nil {
		return nil, err
	}
	candidate, err := m.startAgentProcess(ctx, &plan.spec, startOpts)
	if err == nil {
		return candidate, nil
	}
	if !acp.LoadSessionRequiresFreshThread(err) {
		return nil, err
	}

	plan.spec.acpSessionID = ""
	startOpts, prepareErr := m.prepareSessionLaunch(ctx, &plan.spec, session, runtime, nil)
	if prepareErr != nil {
		return nil, errors.Join(err, prepareErr)
	}
	candidate, fallbackErr := m.startAgentProcess(ctx, &plan.spec, startOpts)
	if fallbackErr != nil {
		return nil, errors.Join(err, fallbackErr)
	}
	return candidate, nil
}

func recoveryNeedsSignIn(err error) bool {
	diagnostic, ok := acp.ProviderFailureDiagnosticFromError(err)
	return ok && diagnostic.Action == acp.ProviderFailureActionLogin
}

func (m *Manager) recoveryFailureEvent(session *Session, state *promptTurnDispatchState, err error) acp.AgentEvent {
	now := m.now()
	event := acp.AgentEvent{
		Type: acp.EventTypeError, TurnID: state.turnID, Timestamp: now,
		Error: err.Error(), Failure: sessionFailureFromError(err, store.FailureProtocol),
	}
	if diagnostic, ok := acp.ProviderFailureDiagnosticFromError(
		err,
	); ok &&
		diagnostic.Action == acp.ProviderFailureActionLogin {
		event.Failure.Kind = store.FailurePrompt
		event.ProviderError = &acp.ProviderErrorDiagnostic{
			Code: acp.ProviderErrorAuthRequired, Provider: session.Info().Provider,
			NextAction: diagnostic.Action, Guidance: diagnostic.Guidance,
			OccurrenceCount: 1, FirstSeenAt: now, LastSeenAt: now,
		}
	}
	return event
}

func (m *Manager) waitForPromptRecovery(session *Session, state *promptTurnDispatchState, delay time.Duration) bool {
	return !session.promptCancellationRequested(state.turnID) &&
		waitForPromptRecoverySignal(state.recovery.executionCtx, delay, session.authRecoverySignal()) == nil &&
		!m.daemonShuttingDown()
}
