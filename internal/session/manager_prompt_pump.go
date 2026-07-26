package session

import (
	"context"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
)

type promptPumpFatal struct {
	failure   *store.SessionFailure
	errorText string
}

func (f *promptPumpFatal) capture(failure *store.SessionFailure, errorText string) {
	if f == nil || failure == nil {
		return
	}
	f.failure = failure
	f.errorText = errorText
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
			m.flushPromptPumpRun(run, &loop, coalescer)
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
	fatalPromptFailure *store.SessionFailure,
	fatalPromptError string,
) {
	if releaseExecution != nil {
		releaseExecution()
	}
	if activity != nil {
		activity.stop()
		activity.finish(m.now())
	}
	if fatalPromptFailure == nil {
		m.finishPromptMessage(lifecycleCtx, turnState, time.Time{})
		m.dispatchTurnEnd(lifecycleCtx, turnState, time.Time{})
	}
	m.finishManagedInputPrompt(lifecycleCtx, session, turnState)
	if session != nil {
		session.clearCurrentTurnID()
		session.clearCurrentTurnSource()
		session.clearCurrentPromptMessage()
		session.clearCurrentPromptMeta()
		session.clearCurrentPromptCancel()
	}
	if fatalPromptFailure == nil {
		notifier := m.currentTurnEndNotifier()
		if notifier != nil && session != nil {
			notifier(session.ID)
		}
	}
	close(out)
	if session == nil {
		return
	}
	if fatalPromptFailure != nil {
		m.stopSessionAfterFatalPromptFailure(lifecycleCtx, session, fatalPromptFailure, fatalPromptError)
		return
	}
	m.startNextQueuedInputPrompt(session.ID)
	m.startNextQueuedSyntheticPrompt(session.ID)
}
