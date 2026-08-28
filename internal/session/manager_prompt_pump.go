package session

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

type promptPumpFatal struct {
	failure           *store.SessionFailure
	errorText         string
	finalizationOwned bool
	waitErr           error
}

func (f *promptPumpFatal) capture(failure *store.SessionFailure, errorText string) {
	if f == nil || failure == nil {
		return
	}
	f.failure = failure
	f.errorText = errorText
}

func (f *promptPumpFatal) ownFinalization(waitErr error) {
	if f == nil {
		return
	}
	f.finalizationOwned = true
	f.waitErr = waitErr
}

type promptPumpRun struct {
	lifecycleCtx context.Context
	deliveryCtx  context.Context
	session      *Session
	turnState    *promptTurnDispatchState
	source       <-chan acp.AgentEvent
	runtime      <-chan acp.AgentEvent
	out          chan<- acp.AgentEvent
	activity     *promptActivitySupervisor
	fatal        *promptPumpFatal
}

func (m *Manager) runPromptPumpLoop(run *promptPumpRun) {
	loop := promptPumpLoopState{source: run.source, runtime: run.runtime, activity: run.activity}
	coalescer := &promptChunkCoalescer{}
	for {
		for loop.active() {
			event, runtimeEvent, ok, flushOnly := m.nextPromptPumpEventWithPendingChunk(
				run.lifecycleCtx,
				&loop,
				coalescer,
			)
			if flushOnly {
				if m.flushPromptPumpRun(run, &loop, coalescer) {
					return
				}
				continue
			}
			if !ok {
				if m.flushPromptPumpRun(run, &loop, coalescer) {
					return
				}
				break
			}
			if coalescer.append(event, runtimeEvent) {
				if !coalescer.shouldFlush() {
					continue
				}
				if m.flushPromptPumpRun(run, &loop, coalescer) {
					return
				}
				continue
			}
			if m.flushPromptPumpRun(run, &loop, coalescer) {
				return
			}
			if coalescer.append(event, runtimeEvent) {
				if !coalescer.shouldFlush() {
					continue
				}
				if m.flushPromptPumpRun(run, &loop, coalescer) {
					return
				}
				continue
			}
			if m.handlePromptPumpRun(run, &loop, event, runtimeEvent) {
				return
			}
		}
		terminal, ok := promptStreamFailureWithoutTerminal(run, &loop)
		if !ok {
			return
		}
		if m.handlePromptPumpRun(run, &loop, terminal, false) {
			return
		}
		if !loop.active() {
			return
		}
	}
}

func promptStreamFailureWithoutTerminal(
	run *promptPumpRun,
	loop *promptPumpLoopState,
) (acp.AgentEvent, bool) {
	if run == nil || loop == nil || run.lifecycleCtx.Err() != nil || loop.turnEnded {
		return acp.AgentEvent{}, false
	}
	if run.session == nil {
		return acp.PromptStreamIncompleteEvent(), true
	}
	if run.turnState != nil && run.session.promptCancellationRequested(run.turnState.turnID) {
		return promptCancellationTerminalEvent(), true
	}
	info := run.session.Info()
	if info == nil || info.State != StateActive {
		return acp.AgentEvent{}, false
	}
	proc := run.session.processHandle()
	if isProcessDone(proc) {
		return promptProcessExitEvent(proc), true
	}
	return acp.PromptStreamIncompleteEvent(), true
}

func promptCancellationTerminalEvent() acp.AgentEvent {
	return acp.AgentEvent{
		Type:             acp.EventTypeDone,
		StopReason:       string(acp.PromptStopReasonCancelled),
		PromptStopReason: acp.PromptStopReasonCancelled,
	}
}

func (m *Manager) flushPromptPumpRun(
	run *promptPumpRun,
	loop *promptPumpLoopState,
	coalescer *promptChunkCoalescer,
) bool {
	failure, errorText, stop := m.flushPromptChunkCoalescer(
		run.lifecycleCtx,
		run.deliveryCtx,
		run.session,
		run.turnState,
		run.out,
		loop,
		coalescer,
		run.fatal,
	)
	run.fatal.capture(failure, errorText)
	return stop
}

func (m *Manager) handlePromptPumpRun(
	run *promptPumpRun,
	loop *promptPumpLoopState,
	event acp.AgentEvent,
	runtimeEvent bool,
) bool {
	failure, errorText, stop := m.handlePromptPumpEvent(
		run.lifecycleCtx,
		run.deliveryCtx,
		run.session,
		run.turnState,
		run.out,
		loop,
		event,
		runtimeEvent,
		run.fatal,
	)
	run.fatal.capture(failure, errorText)
	return stop
}

func (m *Manager) finishPromptPump(
	lifecycleCtx context.Context,
	session *Session,
	turnState *promptTurnDispatchState,
	activity *promptActivitySupervisor,
	releaseExecution context.CancelFunc,
	out chan<- acp.AgentEvent,
	fatal *promptPumpFatal,
) {
	identity, identityErr := promptRunIdentity(session, turnState)
	m.releaseHostedPromptRun(session, turnState)
	var fatalPromptFailure *store.SessionFailure
	if fatal != nil {
		fatalPromptFailure = fatal.failure
	}
	if releaseExecution != nil {
		releaseExecution()
	}
	if activity != nil {
		activity.stop()
		activity.finish(m.now())
	}
	if session != nil {
		if err := m.settleSessionAttention(lifecycleCtx, session.ID, m.now().UTC()); err != nil {
			m.sessionLogger(session).Error(
				"session: persist terminal attention failed",
				"error", err,
			)
		}
	}
	if fatalPromptFailure == nil {
		m.finishPromptMessage(lifecycleCtx, turnState, time.Time{})
		m.dispatchTurnEnd(lifecycleCtx, turnState, time.Time{})
	}
	m.finishManagedInputPrompt(lifecycleCtx, session, turnState)
	if session != nil {
		if turnState != nil {
			session.clearPromptCancellation(turnState.turnID)
		}
		session.clearCurrentTurnID()
		session.clearCurrentTurnSource()
		session.clearCurrentPromptMessage()
		session.clearCurrentPromptMeta()
		session.clearCurrentSkillInvocations()
		session.clearCurrentPromptCancel()
		session.finishCurrentPromptCompletion()
	}
	if identityErr == nil {
		m.clearActivePromptRun(identity)
		notifier := m.currentTurnEndNotifier()
		if notifier != nil {
			notifier(lifecycleCtx, identity)
		}
	}
	if session == nil {
		closePromptOutput(out)
		return
	}
	if fatalPromptFailure != nil {
		if fatal.finalizationOwned {
			m.finalizeSessionAfterFatalPromptFailure(lifecycleCtx, session, fatal)
			closePromptOutput(out)
			return
		}
		m.stopSessionAfterFatalPromptFailure(lifecycleCtx, session, fatal.failure, fatal.errorText)
		closePromptOutput(out)
		return
	}
	closePromptOutput(out)
	m.startNextQueuedInputPrompt(session.ID)
	m.startNextQueuedSyntheticPrompt(session.ID)
}

func closePromptOutput(out chan<- acp.AgentEvent) {
	if out != nil {
		close(out)
	}
}
