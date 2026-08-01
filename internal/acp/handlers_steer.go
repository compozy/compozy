package acp

import (
	"context"
	"errors"

	"fmt"
	"strconv"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/store"
)

func (p *AgentProcess) injectSteerAfterToolResult(ctx context.Context, boundary AgentEvent) {
	if p == nil || p.steerSource == nil || p.conn == nil || boundary.Type != EventTypeToolResult {
		return
	}
	sessionID := strings.TrimSpace(p.SessionID)
	if sessionID == "" {
		return
	}
	consumeCtx, cancel := steerConsumeContext(ctx, p)
	defer cancel()

	input, ok, err := p.steerSource.ConsumeSteer(consumeCtx, sessionID)
	if err != nil {
		p.emitPromptEvent(AgentEvent{
			Type:      EventTypeError,
			SessionID: sessionID,
			TurnID:    firstNonEmptyString(p.activeTurnID(), boundary.TurnID),
			Timestamp: timeNowUTC(),
			Error:     fmt.Sprintf("consume staged steer input: %v", err),
		})
		return
	}
	if !ok {
		return
	}

	turnID := firstNonEmptyString(input.TurnID, p.activeTurnID(), boundary.TurnID)
	p.emitPromptEvent(steeredUserEvent(sessionID, turnID, input))

	go p.dispatchSteerPrompt(steerDispatchContext(p), input, turnID)
}

func steeredUserEvent(sessionID string, turnID string, input SteerInput) AgentEvent {
	localEvent := AgentEvent{
		Type:      EventTypeUserMessage,
		SessionID: strings.TrimSpace(sessionID),
		TurnID:    strings.TrimSpace(turnID),
		RequestID: strings.TrimSpace(input.QueueEntryID),
		Timestamp: timeNowUTC(),
		Text:      strings.TrimSpace(input.Text),
		Action:    PromptActionSteered,
		Resource:  "session_input_queue",
		Decision:  strconv.FormatInt(input.QueueGeneration, 10),
	}
	return localEvent.WithMessageID(input.MessageID).WithEventID(input.EventID)
}

func steerConsumeContext(ctx context.Context, process *AgentProcess) (context.Context, context.CancelFunc) {
	if ctx != nil {
		detached, cancel := withoutCancelPreservingDeadline(ctx)
		if _, ok := detached.Deadline(); ok {
			return detached, cancel
		}
		timeoutCtx, timeoutCancel := context.WithTimeout(detached, steerDispatchTimeout)
		return timeoutCtx, func() {
			timeoutCancel()
			cancel()
		}
	}
	return context.WithTimeout(steerDispatchContext(process), steerDispatchTimeout)
}

func steerDispatchContext(process *AgentProcess) context.Context {
	if process != nil && process.processCtx != nil {
		return process.processCtx
	}
	return context.Background()
}

func (p *AgentProcess) dispatchSteerPrompt(ctx context.Context, input SteerInput, turnID string) {
	if p == nil || p.conn == nil {
		return
	}
	dispatchCtx, cancel := steerTimeoutContext(ctx)
	defer cancel()

	request := acpsdk.PromptRequest{
		SessionId: acpsdk.SessionId(p.SessionID),
		Prompt:    []acpsdk.ContentBlock{acpsdk.TextBlock(strings.TrimSpace(input.Text))},
	}
	if meta, err := steerPromptMeta(input); err == nil {
		request.Meta = meta
	} else {
		finalizeErr := p.failSteerDispatch(ctx, input, err)
		p.emitPromptEvent(AgentEvent{
			Type:      EventTypeError,
			SessionID: p.SessionID,
			TurnID:    turnID,
			Timestamp: timeNowUTC(),
			Error:     fmt.Sprintf("build staged steer metadata: %v", errors.Join(err, finalizeErr)),
		})
		return
	}
	if _, err := acpsdk.SendRequest[wirePromptResponse](
		p.conn,
		dispatchCtx,
		acpsdk.AgentMethodSessionPrompt,
		request,
	); err != nil {
		finalizeErr := p.failSteerDispatch(ctx, input, err)
		failure, _ := FailureFromError(err, store.FailurePrompt)
		p.emitPromptEvent(AgentEvent{
			Type:      EventTypeError,
			SessionID: p.SessionID,
			TurnID:    turnID,
			Timestamp: timeNowUTC(),
			Error:     fmt.Sprintf("dispatch staged steer input: %v", errors.Join(err, finalizeErr)),
			Failure:   failure,
			Raw:       requestErrorRaw(err),
		})
		return
	}
	if err := p.completeSteerDispatch(ctx, input); err != nil {
		p.emitPromptEvent(AgentEvent{
			Type:      EventTypeError,
			SessionID: p.SessionID,
			TurnID:    turnID,
			Timestamp: timeNowUTC(),
			Error:     fmt.Sprintf("complete staged steer dispatch: %v", err),
		})
	}
}

func (p *AgentProcess) completeSteerDispatch(ctx context.Context, input SteerInput) error {
	if p == nil || p.steerSource == nil {
		return nil
	}
	finalizeCtx, cancel := steerFinalizeContext(ctx, p)
	defer cancel()
	return p.steerSource.CompleteSteer(
		finalizeCtx,
		strings.TrimSpace(p.SessionID),
		strings.TrimSpace(input.QueueEntryID),
	)
}

func (p *AgentProcess) failSteerDispatch(ctx context.Context, input SteerInput, cause error) error {
	if p == nil || p.steerSource == nil {
		return nil
	}
	finalizeCtx, cancel := steerFinalizeContext(ctx, p)
	defer cancel()
	summary := "staged steer dispatch failed"
	if cause != nil {
		summary = cause.Error()
	}
	return p.steerSource.FailSteer(
		finalizeCtx,
		strings.TrimSpace(p.SessionID),
		strings.TrimSpace(input.QueueEntryID),
		summary,
	)
}

func steerFinalizeContext(ctx context.Context, process *AgentProcess) (context.Context, context.CancelFunc) {
	base := context.Background()
	if process != nil && process.processCtx != nil {
		base = context.WithoutCancel(process.processCtx)
	} else if ctx != nil {
		base = context.WithoutCancel(ctx)
	}
	return context.WithTimeout(base, steerDispatchTimeout)
}

func steerTimeoutContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.WithTimeout(context.Background(), steerDispatchTimeout)
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, steerDispatchTimeout)
}

func steerPromptMeta(input SteerInput) (map[string]any, error) {
	meta, err := PromptMeta{TurnSource: PromptTurnSourceUser}.ToMap()
	if err != nil {
		return nil, err
	}
	if meta == nil {
		meta = make(map[string]any)
	}
	meta["busy_input"] = map[string]any{
		sessionConfigModeKey: "steer",
		"queue_entry_id":     strings.TrimSpace(input.QueueEntryID),
		"queue_generation":   input.QueueGeneration,
	}
	return meta, nil
}
