package session

import (
	"context"

	"fmt"

	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func (m *Manager) dispatchInputPreSubmit(
	ctx context.Context,
	session *Session,
	turnID string,
	turnSource TurnSource,
	message string,
) (string, error) {
	if m == nil {
		return message, nil
	}
	ctx = hookDispatchContext(ctx, m, session)

	payload, err := m.hooks.prompt().DispatchInputPreSubmit(ctx, hookspkg.InputPreSubmitPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookInputPreSubmit,
			Timestamp: m.now(),
		},
		SessionContext: hookSessionContext(session),
		TurnContext:    hookspkg.TurnContext{TurnID: strings.TrimSpace(turnID)},
		InputClass:     inputClassForTurnSource(turnSource),
		Message:        message,
	})
	if err != nil {
		return "", fmt.Errorf("session: dispatch input.pre_submit: %w", err)
	}

	return payload.Message, nil
}

func (m *Manager) dispatchPromptPostAssemble(
	ctx context.Context,
	sessionCtx hookspkg.SessionContext,
	prompt string,
) (string, error) {
	if m == nil {
		return prompt, nil
	}

	payload, err := m.hooks.prompt().DispatchPromptPostAssemble(ctx, hookspkg.PromptPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookPromptPostAssemble,
			Timestamp: m.now(),
		},
		SessionContext: sessionCtx,
		InputClass:     hookInputClassStartup,
		Prompt:         prompt,
	})
	if err != nil {
		return "", fmt.Errorf("session: dispatch prompt.post_assemble: %w", err)
	}

	return strings.TrimSpace(payload.Prompt), nil
}

func (m *Manager) dispatchTurnStart(ctx context.Context, state *promptTurnDispatchState) error {
	if m == nil || state == nil {
		return nil
	}
	ctx = hookDispatchContext(ctx, m, state.session)

	_, err := m.hooks.conversation().DispatchTurnStart(ctx, hookspkg.TurnStartPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookTurnStart,
			Timestamp: m.now(),
		},
		SessionContext: hookSessionContext(state.session),
		TurnContext:    hookspkg.TurnContext{TurnID: state.turnID},
		InputClass:     state.inputClass,
		UserMessage:    state.userMessage,
	})
	if err != nil {
		return fmt.Errorf("session: dispatch turn.start: %w", err)
	}

	return nil
}

func (m *Manager) dispatchTurnEnd(ctx context.Context, state *promptTurnDispatchState, eventTime time.Time) {
	if state == nil || state.turnEnded {
		return
	}
	state.turnEnded = true
	if m == nil {
		return
	}
	ctx = hookDispatchContext(ctx, m, state.session)

	_, err := m.hooks.conversation().DispatchTurnEnd(ctx, hookspkg.TurnEndPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookTurnEnd,
			Timestamp: hookTimestamp(m.now(), eventTime),
		},
		SessionContext: hookSessionContext(state.session),
		TurnContext:    hookspkg.TurnContext{TurnID: state.turnID},
		InputClass:     state.inputClass,
		UserMessage:    state.userMessage,
	})
	if err != nil {
		m.warnHookDispatch(ctx, state.session, hookspkg.HookTurnEnd, err)
	}
}

func (m *Manager) preparePromptEvent(
	ctx context.Context,
	state *promptTurnDispatchState,
	event acp.AgentEvent,
) acp.AgentEvent {
	if state == nil {
		return event
	}

	role, deltaType, isMessage := hookMessageDetails(event.Type)
	if !isMessage {
		m.finishPromptMessage(ctx, state, event.Timestamp)
		return event
	}

	if state.openMessage == nil {
		event = m.dispatchMessageStart(ctx, state, event, role)
	}

	if state.openMessage == nil {
		return event
	}

	state.openMessage.text.WriteString(event.Text)
	state.openMessage.lastRaw = cloneSessionRawMessage(event.Raw)
	m.dispatchMessageDelta(ctx, state, event, deltaType)
	return event
}

func (m *Manager) dispatchMessageStart(
	ctx context.Context,
	state *promptTurnDispatchState,
	event acp.AgentEvent,
	role string,
) acp.AgentEvent {
	if state == nil {
		return event
	}

	state.messageSeq++
	message := &promptMessageDispatchState{
		id:   nextPromptMessageID(state.turnID, state.messageSeq),
		role: strings.TrimSpace(role),
	}
	state.openMessage = message
	if m == nil {
		return event
	}
	ctx = hookDispatchContext(ctx, m, state.session)

	payload, err := m.hooks.conversation().DispatchMessageStart(ctx, hookspkg.MessageStartPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookMessageStart,
			Timestamp: hookTimestamp(m.now(), event.Timestamp),
		},
		SessionContext: hookSessionContext(state.session),
		TurnContext:    hookspkg.TurnContext{TurnID: state.turnID},
		MessageID:      message.id,
		Role:           message.role,
		DeltaType:      hookMessageDeltaTypeFull,
		Text:           event.Text,
		Raw:            cloneSessionRawMessage(event.Raw),
	})
	if err != nil {
		m.warnHookDispatch(ctx, state.session, hookspkg.HookMessageStart, err)
		return event
	}

	message.role = strings.TrimSpace(payload.Role)
	event.Text = payload.Text
	return event
}

func (m *Manager) dispatchMessageDelta(
	ctx context.Context,
	state *promptTurnDispatchState,
	event acp.AgentEvent,
	deltaType string,
) {
	if m == nil || state == nil || state.openMessage == nil {
		return
	}
	ctx = hookDispatchContext(ctx, m, state.session)

	_, err := m.hooks.conversation().DispatchMessageDelta(ctx, hookspkg.MessageDeltaPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookMessageDelta,
			Timestamp: hookTimestamp(m.now(), event.Timestamp),
		},
		SessionContext: hookSessionContext(state.session),
		TurnContext:    hookspkg.TurnContext{TurnID: state.turnID},
		MessageID:      state.openMessage.id,
		Role:           state.openMessage.role,
		DeltaType:      strings.TrimSpace(deltaType),
		Text:           event.Text,
		Raw:            cloneSessionRawMessage(event.Raw),
	})
	if err != nil {
		m.warnHookDispatch(ctx, state.session, hookspkg.HookMessageDelta, err)
	}
}

func (m *Manager) finishPromptMessage(ctx context.Context, state *promptTurnDispatchState, eventTime time.Time) {
	if state == nil || state.openMessage == nil {
		return
	}

	message := state.openMessage
	state.openMessage = nil
	if m == nil {
		return
	}
	ctx = hookDispatchContext(ctx, m, state.session)

	_, err := m.hooks.conversation().DispatchMessageEnd(ctx, hookspkg.MessageEndPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookMessageEnd,
			Timestamp: hookTimestamp(m.now(), eventTime),
		},
		SessionContext: hookSessionContext(state.session),
		TurnContext:    hookspkg.TurnContext{TurnID: state.turnID},
		MessageID:      message.id,
		Role:           message.role,
		DeltaType:      hookMessageDeltaTypeFull,
		Text:           message.text.String(),
		Raw:            cloneSessionRawMessage(message.lastRaw),
	})
	if err != nil {
		m.warnHookDispatch(ctx, state.session, hookspkg.HookMessageEnd, err)
	}
}
