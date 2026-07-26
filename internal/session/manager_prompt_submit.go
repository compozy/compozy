package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/acp"
)

func (m *Manager) submitPromptRequest(ctx context.Context, req promptRequest) (<-chan acp.AgentEvent, error) {
	session, err := m.lookupPromptSession(ctx, req.target)
	if err != nil {
		return nil, err
	}
	message, err := m.dispatchInputPreSubmit(ctx, session, req.turnID, req.turnSource, req.message)
	if err != nil {
		return nil, err
	}
	turnState := newPromptTurnDispatchState(session, req.turnID, req.turnSource, message)
	if err := m.dispatchTurnStart(ctx, turnState); err != nil {
		return nil, err
	}
	proc, err := session.beginExclusivePromptSetup()
	if err != nil {
		return nil, err
	}
	defer session.finishPromptSetup()
	return m.submitPromptInReservedSlot(ctx, session, proc, req, message, turnState)
}

func (m *Manager) submitPromptInReservedSlot(
	ctx context.Context,
	session *Session,
	proc *AgentProcess,
	req promptRequest,
	message string,
	turnState *promptTurnDispatchState,
) (<-chan acp.AgentEvent, error) {
	session.setCurrentTurnID(req.turnID)
	session.setCurrentTurnSource(turnState.turnSource)
	session.setCurrentPromptMessage(req.authoredMessage)
	session.setCurrentPromptMeta(req.meta)
	promptExecutionCtx, cancelPromptExecution := m.promptExecutionContext(ctx, turnState.managed != nil)
	session.setCurrentPromptCancel(cancelPromptExecution)
	clearTurnSource := true
	defer func() {
		if clearTurnSource {
			cancelPromptExecution()
			session.clearCurrentTurnID()
			session.clearCurrentTurnSource()
			session.clearCurrentPromptMessage()
			session.clearCurrentPromptMeta()
			session.clearCurrentPromptCancel()
		}
	}()

	req.message = message
	if err := m.recordPromptInputEvent(ctx, session, &req); err != nil {
		return nil, err
	}
	dispatchMessage, err := m.promptDispatchMessage(ctx, session, message)
	if err != nil {
		return nil, err
	}
	replayBlock := m.pendingResumeReplay(session.ID)
	dispatchMessage = promptWithResumeReplay(replayBlock, dispatchMessage)
	if _, err := m.persistSessionPromptActivity(ctx, session, m.now()); err != nil {
		return nil, err
	}
	activity := newPromptActivitySupervisor(promptExecutionCtx, m, session, turnState, m.supervision)
	activity.start()
	source, err := m.driver.Prompt(promptExecutionCtx, proc, acp.PromptRequest{
		TurnID:                    req.turnID,
		Message:                   dispatchMessage,
		Meta:                      req.meta,
		ActivityReporter:          activity.report,
		ActivityHeartbeatInterval: m.supervision.ActivityHeartbeatInterval,
	})
	if err != nil {
		cancelPromptExecution()
		activity.stop()
		activity.finish(m.now())
		return nil, fmt.Errorf("session: prompt session %q: %w", req.target, err)
	}
	if turnState.managed != nil {
		if err := m.recordManagedDriverAttached(ctx, turnState.managed, req.turnID); err != nil {
			m.abortPromptBeforePump(cancelPromptExecution, activity, source)
			return nil, err
		}
	}
	if err := m.preparePromptDelivery(ctx, session, req, cancelPromptExecution, activity, source); err != nil {
		return nil, err
	}
	m.consumeResumeReplay(session.ID, replayBlock)

	clearTurnSource = false
	lifecycleCtx := m.fallbackLifecycleContext()
	deliveryCtx := req.deliveryCtx
	if deliveryCtx == nil {
		deliveryCtx = ctx
	}
	return m.startPromptPump(
		lifecycleCtx,
		deliveryCtx,
		session,
		turnState,
		source,
		activity,
		cancelPromptExecution,
	), nil
}

func (m *Manager) preparePromptDelivery(
	ctx context.Context,
	session *Session,
	req promptRequest,
	cancelPromptExecution context.CancelFunc,
	activity *promptActivitySupervisor,
	source <-chan acp.AgentEvent,
) error {
	if req.prepareDelivery == nil {
		return nil
	}
	if err := req.prepareDelivery(ctx, PromptDelivery{SessionID: session.ID, TurnID: req.turnID}); err != nil {
		m.abortPromptBeforePump(cancelPromptExecution, activity, source)
		return fmt.Errorf("session: prepare prompt delivery for %q: %w", req.target, err)
	}
	return nil
}

func (m *Manager) abortPromptBeforePump(
	cancelPromptExecution context.CancelFunc,
	activity *promptActivitySupervisor,
	source <-chan acp.AgentEvent,
) {
	cancelPromptExecution()
	activity.stop()
	activity.finish(m.now())
	go drainPromptSource(source)
}

func (m *Manager) startPromptPump(
	lifecycleCtx context.Context,
	callerCtx context.Context,
	session *Session,
	turnState *promptTurnDispatchState,
	source <-chan acp.AgentEvent,
	activity *promptActivitySupervisor,
	cancelPromptExecution context.CancelFunc,
) <-chan acp.AgentEvent {
	out := make(chan acp.AgentEvent, m.promptBufSize)
	finishDrain := m.trackPromptDrain()
	go func() {
		defer finishDrain()
		m.pumpPrompt(
			lifecycleCtx,
			callerCtx,
			session,
			turnState,
			source,
			activity.eventsChannel(),
			out,
			activity,
			cancelPromptExecution,
		)
	}()
	return out
}

func (m *Manager) promptDispatchMessage(ctx context.Context, session *Session, message string) (string, error) {
	if m.inputAugmenter == nil {
		return message, nil
	}
	augmented, err := m.inputAugmenter(ctx, session, message)
	if err != nil {
		return "", fmt.Errorf("session: augment prompt input: %w", err)
	}
	if strings.TrimSpace(augmented) == "" {
		return message, nil
	}
	return augmented, nil
}

func drainPromptSource(source <-chan acp.AgentEvent) {
	for range source {
		continue
	}
}
