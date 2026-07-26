package session

import (
	"context"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

type noopConversationHooks struct{}

func (noopConversationHooks) DispatchTurnStart(
	_ context.Context,
	payload hookspkg.TurnStartPayload,
) (hookspkg.TurnStartPayload, error) {
	return payload, nil
}

func (noopConversationHooks) DispatchTurnEnd(
	_ context.Context,
	payload hookspkg.TurnEndPayload,
) (hookspkg.TurnEndPayload, error) {
	return payload, nil
}

func (noopConversationHooks) DispatchMessageStart(
	_ context.Context,
	payload hookspkg.MessageStartPayload,
) (hookspkg.MessageStartPayload, error) {
	return payload, nil
}

func (noopConversationHooks) DispatchMessageDelta(
	_ context.Context,
	payload hookspkg.MessageDeltaPayload,
) (hookspkg.MessageDeltaPayload, error) {
	return payload, nil
}

func (noopConversationHooks) DispatchMessageEnd(
	_ context.Context,
	payload hookspkg.MessageEndPayload,
) (hookspkg.MessageEndPayload, error) {
	return payload, nil
}

func (noopConversationHooks) DispatchSessionMessagePersisted(
	_ context.Context,
	payload hookspkg.SessionMessagePersistedPayload,
) (hookspkg.SessionMessagePersistedPayload, error) {
	return payload, nil
}

type noopToolHooks struct{}

func (noopToolHooks) DispatchToolPreCall(
	_ context.Context,
	payload hookspkg.ToolPreCallPayload,
) (hookspkg.ToolPreCallPayload, error) {
	return payload, nil
}

func (noopToolHooks) DispatchToolPostCall(
	_ context.Context,
	payload hookspkg.ToolPostCallPayload,
) (hookspkg.ToolPostCallPayload, error) {
	return payload, nil
}

func (noopToolHooks) DispatchToolPostError(
	_ context.Context,
	payload hookspkg.ToolPostErrorPayload,
) (hookspkg.ToolPostErrorPayload, error) {
	return payload, nil
}

type noopCompactionHooks struct{}

func (noopCompactionHooks) DispatchContextPreCompact(
	_ context.Context,
	payload hookspkg.ContextPreCompactPayload,
) (hookspkg.ContextPreCompactPayload, error) {
	return payload, nil
}

func (noopCompactionHooks) DispatchContextPostCompact(
	_ context.Context,
	payload hookspkg.ContextPostCompactPayload,
) (hookspkg.ContextPostCompactPayload, error) {
	return payload, nil
}
