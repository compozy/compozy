package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	commandpkg "github.com/compozy/compozy/internal/command"
)

func (m *Manager) submitPromptRequest(ctx context.Context, req promptRequest) (<-chan acp.AgentEvent, error) {
	session, err := m.lookupPromptRequestSession(ctx, req)
	if err != nil {
		return nil, err
	}
	proc, err := session.beginExclusivePromptSetup()
	if err != nil {
		return nil, err
	}
	slotReserved := true
	defer func() {
		if slotReserved {
			session.finishPromptSetup()
		}
	}()
	if err := m.validateReservedRuntimeModel(session, proc, req.runtime); err != nil {
		return nil, err
	}
	if req.commitDispatch != nil {
		if err := req.commitDispatch(ctx); err != nil {
			return nil, err
		}
		req.commitDispatch = nil
	}
	proc, err = m.ensurePromptRuntime(ctx, session, req.runtime, proc)
	if err != nil {
		return nil, err
	}
	if req.releaseSlotBeforeHooks {
		session.finishPromptSetup()
		slotReserved = false
	}
	message, err := m.dispatchInputPreSubmit(
		ctx,
		session,
		req.turnID,
		req.turnSource,
		req.message,
		req.attachments,
	)
	if err != nil {
		return nil, err
	}
	req.skillInvocations = commandpkg.ReconcileInvocations(message, req.skillInvocations)
	if !slotReserved {
		proc, err = session.beginExclusivePromptSetup()
		if err != nil {
			return nil, err
		}
		slotReserved = true
		if err := m.validateReservedRuntimeModel(session, proc, req.runtime); err != nil {
			return nil, err
		}
		proc, err = m.ensurePromptRuntime(ctx, session, req.runtime, proc)
		if err != nil {
			return nil, err
		}
	}
	resolvedAttachments, err := m.resolvePromptAttachments(
		ctx,
		session.WorkspaceID,
		session.ID,
		req.attachments,
		proc.CapsSnapshot(),
	)
	if err != nil {
		return nil, err
	}
	session.setCurrentTurnID(req.turnID)
	session.setCurrentTurnSource(req.turnSource)
	session.setCurrentPromptMessage(req.authoredMessage)
	session.setCurrentPromptMeta(req.meta)
	session.setCurrentSkillInvocations(req.skillInvocations)
	stateOwned := true
	defer func() {
		if stateOwned {
			session.clearPromptCancellation(req.turnID)
			session.clearCurrentTurnID()
			session.clearCurrentTurnSource()
			session.clearCurrentPromptMessage()
			session.clearCurrentPromptMeta()
			session.clearCurrentSkillInvocations()
			session.finishCurrentPromptCompletion()
		}
	}()
	dispatchMessage, err := m.promptDispatchMessage(ctx, session, message)
	if err != nil {
		return nil, err
	}
	turnState := newPromptTurnDispatchState(session, req.turnID, req.turnSource, message)
	if err := m.dispatchTurnStart(ctx, turnState); err != nil {
		return nil, err
	}
	stateOwned = false
	return m.submitPromptInReservedSlot(
		ctx,
		session,
		proc,
		req,
		message,
		dispatchMessage,
		resolvedAttachments,
		turnState,
	)
}

func (m *Manager) submitPromptInReservedSlot(
	ctx context.Context,
	session *Session,
	proc *AgentProcess,
	req promptRequest,
	message string,
	dispatchMessage string,
	attachments []acp.PromptAttachment,
	turnState *promptTurnDispatchState,
) (<-chan acp.AgentEvent, error) {
	promptExecutionCtx, cancelPromptExecution := m.promptExecutionContext(ctx, turnState.managed != nil)
	session.setCurrentPromptCancel(cancelPromptExecution)
	clearTurnSource := true
	defer func() {
		if clearTurnSource {
			cancelPromptExecution()
			session.clearPromptCancellation(req.turnID)
			session.clearCurrentTurnID()
			session.clearCurrentTurnSource()
			session.clearCurrentPromptMessage()
			session.clearCurrentPromptMeta()
			session.clearCurrentSkillInvocations()
			session.clearCurrentPromptCancel()
			session.finishCurrentPromptCompletion()
		}
	}()

	req.message = message
	if err := m.recordPromptInputEvent(ctx, session, &req); err != nil {
		return nil, err
	}
	replayBlock := m.pendingResumeReplay(session.ID)
	dispatchMessage = promptWithResumeReplay(replayBlock, dispatchMessage)
	if _, err := m.persistSessionPromptActivity(ctx, session, m.now()); err != nil {
		return nil, err
	}
	supervision := supervisionForSession(session, m.supervision)
	activity := newPromptActivitySupervisor(promptExecutionCtx, m, session, turnState, supervision)
	activity.start()
	source, err := m.driver.Prompt(promptExecutionCtx, proc, acp.PromptRequest{
		TurnID:                    req.turnID,
		Message:                   dispatchMessage,
		Attachments:               attachments,
		Meta:                      req.meta,
		ActivityReporter:          activity.report,
		ActivityHeartbeatInterval: supervision.ActivityHeartbeatInterval,
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
	m.startTrackedPromptTask(func() {
		drainPromptSource(source)
	})
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
	m.startTrackedPromptTask(func() {
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
	})
	return out
}

func (m *Manager) promptDispatchMessage(ctx context.Context, session *Session, message string) (string, error) {
	expanded, err := m.expandPromptSkillInvocations(ctx, session, message)
	if err != nil {
		return "", err
	}
	if m.inputAugmenter == nil {
		return expanded, nil
	}
	augmented, err := m.inputAugmenter(ctx, session, expanded)
	if err != nil {
		return "", fmt.Errorf("session: augment prompt input: %w", err)
	}
	if strings.TrimSpace(augmented) == "" {
		return expanded, nil
	}
	return augmented, nil
}

func drainPromptSource(source <-chan acp.AgentEvent) {
	for range source {
		continue
	}
}
