package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	commandpkg "github.com/compozy/compozy/internal/command"
)

func (m *Manager) reservePromptSlot(
	ctx context.Context,
	session *Session,
	runtime *RuntimeSelection,
) (*AgentProcess, error) {
	proc, err := session.beginExclusivePromptSetup()
	if err != nil {
		return nil, err
	}
	proc, err = m.prepareReservedPromptSlot(ctx, session, runtime, proc)
	if err != nil {
		session.finishPromptSetup()
		return nil, err
	}
	return proc, nil
}

func (m *Manager) prepareReservedPromptSlot(
	ctx context.Context,
	session *Session,
	runtime *RuntimeSelection,
	proc *AgentProcess,
) (*AgentProcess, error) {
	return m.ensurePromptRuntime(ctx, session, runtime, proc)
}

func commitPromptDispatch(ctx context.Context, commit func(context.Context) error) error {
	if commit == nil {
		return nil
	}
	return commit(ctx)
}

func (m *Manager) submitPromptRequest(ctx context.Context, req promptRequest) (<-chan acp.AgentEvent, error) {
	session, err := m.lookupPromptRequestSession(ctx, req)
	if err != nil {
		return nil, err
	}
	if req.deliveryCtx == nil {
		req.deliveryCtx = ctx
	}
	promptExecutionCtx, cancelPromptExecution := m.promptExecutionContext(ctx, false)
	proc, err := session.beginExclusivePromptSetupForRequest(req, cancelPromptExecution)
	if err != nil {
		cancelPromptExecution()
		return nil, err
	}
	slotReserved := true
	stateOwned := true
	defer func() {
		if slotReserved {
			session.finishPromptSetup()
		}
		if stateOwned {
			cancelPromptExecution()
			clearPromptState(session, req.turnID)
		}
	}()
	proc, err = m.prepareReservedPromptSlot(promptExecutionCtx, session, req.runtime, proc)
	if err != nil {
		return nil, err
	}
	resolvedAttachments, err := m.preparePromptRequestAttachments(promptExecutionCtx, session, proc, req)
	if err != nil {
		return nil, err
	}
	if req.releaseSlotBeforeHooks {
		session.finishPromptSetup()
		slotReserved = false
		clearPromptState(session, req.turnID)
		stateOwned = false
	}
	message, err := m.preparePromptRequestMessage(promptExecutionCtx, session, &req)
	if err != nil {
		return nil, err
	}
	if !slotReserved {
		proc, err = session.beginExclusivePromptSetupForRequest(req, cancelPromptExecution)
		if err != nil {
			return nil, err
		}
		slotReserved = true
		stateOwned = true
		proc, err = m.prepareReservedPromptSlot(promptExecutionCtx, session, req.runtime, proc)
		if err != nil {
			return nil, err
		}
	} else {
		session.setCurrentSkillInvocations(req.skillInvocations)
	}
	dispatchMessage, err := m.promptDispatchMessage(promptExecutionCtx, session, message)
	if err != nil {
		return nil, err
	}
	turnState := newPromptRequestDispatchState(session, req, message)
	if err := m.dispatchTurnStart(promptExecutionCtx, turnState); err != nil {
		return nil, err
	}
	events, err := m.submitPromptInReservedSlot(
		promptExecutionCtx,
		session,
		proc,
		req,
		message,
		dispatchMessage,
		resolvedAttachments,
		turnState,
		cancelPromptExecution,
	)
	if err != nil {
		return nil, err
	}
	stateOwned = false
	return events, nil
}

func (m *Manager) preparePromptRequestAttachments(
	ctx context.Context,
	session *Session,
	proc *AgentProcess,
	req promptRequest,
) ([]acp.PromptAttachment, error) {
	if err := validatePromptAttachmentCaps(req.attachments, proc.CapsSnapshot()); err != nil {
		return nil, err
	}
	attachments, err := m.resolvePromptAttachments(
		ctx,
		session.WorkspaceID,
		session.ID,
		req.attachments,
		proc.CapsSnapshot(),
	)
	if err != nil {
		return nil, err
	}
	if err := commitPromptDispatch(ctx, req.commitDispatch); err != nil {
		return nil, err
	}
	return attachments, nil
}

func (m *Manager) preparePromptRequestMessage(
	ctx context.Context,
	session *Session,
	req *promptRequest,
) (string, error) {
	message, err := m.dispatchInputPreSubmit(
		ctx,
		session,
		req.turnID,
		req.turnSource,
		req.message,
		req.attachments,
	)
	if err != nil {
		return "", err
	}
	req.skillInvocations = commandpkg.ReconcileInvocations(message, req.skillInvocations)
	return message, nil
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
	cancelPromptExecution context.CancelFunc,
) (<-chan acp.AgentEvent, error) {
	req.message = message
	if err := m.recordPromptInputEvent(ctx, session, &req); err != nil {
		return nil, err
	}
	replayBlock := m.pendingResumeReplay(session.ID)
	dispatchMessage = promptWithResumeReplay(replayBlock, dispatchMessage)
	if _, err := m.persistSessionPromptActivity(ctx, session, m.now()); err != nil {
		return nil, err
	}
	delivery, err := m.openDurablePromptDelivery(req.deliveryCtx, session, req.turnID)
	if err != nil {
		return nil, fmt.Errorf("session: open durable prompt delivery for %q: %w", req.target, err)
	}
	supervision := supervisionForSession(session, m.supervision)
	activity := newPromptActivitySupervisor(ctx, m, session, turnState, supervision)
	activity.start()
	recoveryRequest := acp.PromptRequest{
		TurnID:                    req.turnID,
		RunID:                     req.runID,
		Generation:                session.Info().RuntimeGeneration,
		Message:                   dispatchMessage,
		Attachments:               attachments,
		Meta:                      req.meta,
		ActivityReporter:          activity.report,
		ActivityHeartbeatInterval: supervision.ActivityHeartbeatInterval,
	}
	source, err := m.startHostedPromptRun(ctx, session, proc, turnState, recoveryRequest)
	if err != nil {
		delivery.cancel()
		cancelPromptExecution()
		activity.stop()
		activity.finish(m.now())
		return nil, fmt.Errorf("session: prompt session %q: %w", req.target, err)
	}
	if turnState.managed != nil {
		if err := m.recordManagedDriverAttached(ctx, turnState.managed, req.turnID); err != nil {
			m.releaseHostedPromptRun(session, turnState)
			delivery.cancel()
			m.abortPromptBeforePump(cancelPromptExecution, activity, source)
			return nil, err
		}
	}
	if err := m.preparePromptDelivery(ctx, session, req, cancelPromptExecution, activity, source); err != nil {
		m.releaseHostedPromptRun(session, turnState)
		delivery.cancel()
		return nil, err
	}
	m.consumeResumeReplay(session.ID, replayBlock)

	lifecycleCtx := m.fallbackLifecycleContext()
	m.startPromptPersistencePump(
		lifecycleCtx,
		session,
		turnState,
		source,
		activity,
		cancelPromptExecution,
		delivery.persistenceDone,
	)
	return delivery.events, nil
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

func (m *Manager) startPromptPersistencePump(
	lifecycleCtx context.Context,
	session *Session,
	turnState *promptTurnDispatchState,
	source <-chan acp.AgentEvent,
	activity *promptActivitySupervisor,
	cancelPromptExecution context.CancelFunc,
	persistenceDone chan<- struct{},
) {
	m.startTrackedPromptTask(func() {
		defer closePromptPersistenceDone(persistenceDone)
		m.pumpPrompt(
			lifecycleCtx,
			nil,
			session,
			turnState,
			source,
			activity.eventsChannel(),
			nil,
			activity,
			cancelPromptExecution,
		)
	})
}

func closePromptPersistenceDone(done chan<- struct{}) {
	if done != nil {
		close(done)
	}
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
