package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

type promptRecoveryEventData struct {
	Attempt       int    `json:"attempt"`
	MaxAttempts   int    `json:"max_attempts"`
	Generation    int64  `json:"generation"`
	FailureKind   string `json:"failure_kind,omitempty"`
	FailureDetail string `json:"failure_detail,omitempty"`
}

func clonePromptRecoveryRequest(request acp.PromptRequest) acp.PromptRequest {
	cloned := request
	cloned.Attachments = make([]acp.PromptAttachment, len(request.Attachments))
	for index, attachment := range request.Attachments {
		cloned.Attachments[index] = attachment
		cloned.Attachments[index].Data = append([]byte(nil), attachment.Data...)
	}
	return cloned
}

func promptEventIsRecoverable(event acp.AgentEvent) bool {
	if event.Type != acp.EventTypeError || event.Failure == nil {
		return false
	}
	switch event.Failure.Kind {
	case store.FailureProcess, store.FailureTransport:
		return true
	default:
		return false
	}
}

func (m *Manager) attemptPromptRecovery(
	ctx context.Context,
	session *Session,
	turnState *promptTurnDispatchState,
	loop *promptPumpLoopState,
	trigger acp.AgentEvent,
) (bool, error) {
	if m == nil || session == nil || turnState == nil || turnState.recovery == nil {
		return false, nil
	}
	recovery := turnState.recovery
	if recovery.executionCtx == nil || recovery.executionCtx.Err() != nil ||
		session.promptCancellationRequested(turnState.turnID) {
		return false, nil
	}

	maxAttempts := len(m.promptRecoveryDelays)
	if maxAttempts == 0 || recovery.exhaustedRecorded {
		return false, nil
	}
	for recovery.attempts < maxAttempts {
		attempt := recovery.attempts + 1
		delay := m.promptRecoveryDelays[recovery.attempts]
		recovery.attempts = attempt
		now := m.now()
		nextAttemptAt := now.Add(delay)
		if err := session.beginAutomaticRecovery(attempt, maxAttempts, nextAttemptAt, now); err != nil {
			return false, err
		}
		if err := m.persistSessionLifecycleState(ctx, session, false); err != nil {
			return false, err
		}
		if err := m.recordPromptRecoveryEvent(
			ctx, session, turnState, loop, acp.EventTypeRuntimeRecoveryStarted, trigger, attempt, maxAttempts,
		); err != nil {
			return false, err
		}
		if err := waitForPromptRecovery(recovery.executionCtx, delay); err != nil {
			return false, nil
		}

		source, replayBlock, err := m.startRecoveredPrompt(recovery.executionCtx, session, recovery.request)
		if err != nil {
			if persistErr := m.persistPromptRecoveryFailure(ctx, session, err); persistErr != nil {
				return false, persistErr
			}
			continue
		}
		m.consumeResumeReplay(session.ID, replayBlock)
		loop.source = source
		loop.turnEnded = false
		loop.sourceProbeRequired = false
		if err := m.recordPromptRecoveryEvent(
			ctx, session, turnState, loop, acp.EventTypeRuntimeRecoverySucceeded, trigger, attempt, maxAttempts,
		); err != nil {
			return false, err
		}
		return true, nil
	}

	if err := m.recordPromptRecoveryEvent(
		ctx,
		session,
		turnState,
		loop,
		acp.EventTypeRuntimeRecoveryExhausted,
		trigger,
		recovery.attempts,
		maxAttempts,
	); err != nil {
		return false, err
	}
	recovery.exhaustedRecorded = true
	return false, nil
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
	request acp.PromptRequest,
) (<-chan acp.AgentEvent, string, error) {
	_, replayBlock, err := m.recoverPromptRuntime(ctx, session)
	if err != nil {
		return nil, "", err
	}
	replayRequest := clonePromptRecoveryRequest(request)
	replayRequest.Message = promptWithResumeReplay(replayBlock, replayRequest.Message)
	source, err := m.driver.Prompt(ctx, session.processHandle(), replayRequest)
	if err != nil {
		return nil, "", err
	}
	return source, replayBlock, nil
}

func waitForPromptRecovery(ctx context.Context, delay time.Duration) error {
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
	}
}

func (m *Manager) recordPromptRecoveryEvent(
	ctx context.Context,
	session *Session,
	turnState *promptTurnDispatchState,
	loop *promptPumpLoopState,
	eventType string,
	trigger acp.AgentEvent,
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
		Attempt: attempt, MaxAttempts: maxAttempts, Generation: generation,
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
		return err
	}
	m.dispatchRuntimeRecoveryObservation(
		ctx,
		session,
		turnState.turnID,
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
	snapshot := session.runtimeBindingSnapshot()
	plan, err := m.preparePromptRuntimePlan(ctx, session, snapshot.selection)
	if err != nil {
		return nil, "", err
	}
	runtime := plan.runtime
	runtime.mcpServers, err = m.sessionMCPServers(ctx, &plan.spec, runtime.agent, runtime.agentDef)
	if err != nil {
		return nil, "", fmt.Errorf("session: resolve recovery MCP servers: %w", err)
	}
	plan.spec.resumeReplay = true
	plan.spec.startAction = "automatic recovery"

	candidate, err := m.startPromptRecoveryCandidate(ctx, session, plan, &runtime)
	if err != nil {
		return nil, "", err
	}
	previous := session.completeRuntimeTransition(
		candidate,
		plan.selection,
		RuntimeTransitionAutomaticRecovery,
		m.now(),
	)
	session.setAgentDefinition(runtime.agentDef)
	if err := m.persistSessionLifecycleState(ctx, session, false); err != nil {
		session.restoreRuntimeBinding(&snapshot, err.Error(), m.now())
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
	candidate, err := m.startAgentProcess(ctx, &plan.spec, session, startOpts)
	if err == nil {
		return candidate, nil
	}
	if !acp.IsLoadSessionResourceMissing(err) && !errors.Is(err, acp.ErrAgentDoesNotSupportSession) {
		return nil, err
	}

	plan.spec.acpSessionID = ""
	startOpts, prepareErr := m.prepareSessionLaunch(ctx, &plan.spec, session, runtime, nil)
	if prepareErr != nil {
		return nil, errors.Join(err, prepareErr)
	}
	candidate, fallbackErr := m.startAgentProcess(ctx, &plan.spec, session, startOpts)
	if fallbackErr != nil {
		return nil, errors.Join(err, fallbackErr)
	}
	return candidate, nil
}
