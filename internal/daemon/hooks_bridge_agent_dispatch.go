package daemon

import (
	"context"

	hookspkg "github.com/compozy/compozy/internal/hooks"
)

func (n *hooksNotifier) DispatchAgentPreStart(
	ctx context.Context,
	payload hookspkg.AgentPreStartPayload,
) (hookspkg.AgentPreStartPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookAgentPreStart,
		payload,
		hookRuntime.DispatchAgentPreStart,
	)
}

func (n *hooksNotifier) DispatchAgentSpawned(
	ctx context.Context,
	payload hookspkg.AgentSpawnedPayload,
) (hookspkg.AgentSpawnedPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookAgentSpawned,
		payload,
		hookRuntime.DispatchAgentSpawned,
	)
}

func (n *hooksNotifier) DispatchAgentCrashed(
	ctx context.Context,
	payload hookspkg.AgentCrashedPayload,
) (hookspkg.AgentCrashedPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookAgentCrashed,
		payload,
		hookRuntime.DispatchAgentCrashed,
	)
}

func (n *hooksNotifier) DispatchAgentStopped(
	ctx context.Context,
	payload hookspkg.AgentStoppedPayload,
) (hookspkg.AgentStoppedPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookAgentStopped,
		payload,
		hookRuntime.DispatchAgentStopped,
	)
}

func (n *hooksNotifier) DispatchTurnStart(
	ctx context.Context,
	payload hookspkg.TurnStartPayload,
) (hookspkg.TurnStartPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookTurnStart,
		payload,
		hookRuntime.DispatchTurnStart,
	)
}

func (n *hooksNotifier) DispatchTurnEnd(
	ctx context.Context,
	payload hookspkg.TurnEndPayload,
) (hookspkg.TurnEndPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookTurnEnd,
		payload,
		hookRuntime.DispatchTurnEnd,
	)
}

func (n *hooksNotifier) DispatchMessageStart(
	ctx context.Context,
	payload hookspkg.MessageStartPayload,
) (hookspkg.MessageStartPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookMessageStart,
		payload,
		hookRuntime.DispatchMessageStart,
	)
}

func (n *hooksNotifier) DispatchMessageDelta(
	ctx context.Context,
	payload hookspkg.MessageDeltaPayload,
) (hookspkg.MessageDeltaPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookMessageDelta,
		payload,
		hookRuntime.DispatchMessageDelta,
	)
}

func (n *hooksNotifier) DispatchMessageEnd(
	ctx context.Context,
	payload hookspkg.MessageEndPayload,
) (hookspkg.MessageEndPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookMessageEnd,
		payload,
		hookRuntime.DispatchMessageEnd,
	)
}

func (n *hooksNotifier) DispatchSessionMessagePersisted(
	ctx context.Context,
	payload hookspkg.SessionMessagePersistedPayload,
) (hookspkg.SessionMessagePersistedPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookSessionMessagePersisted,
		payload,
		hookRuntime.DispatchSessionMessagePersisted,
	)
}

func (n *hooksNotifier) DispatchToolPreCall(
	ctx context.Context,
	payload hookspkg.ToolPreCallPayload,
) (hookspkg.ToolPreCallPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookToolPreCall,
		payload,
		hookRuntime.DispatchToolPreCall,
	)
}

func (n *hooksNotifier) DispatchToolPostCall(
	ctx context.Context,
	payload hookspkg.ToolPostCallPayload,
) (hookspkg.ToolPostCallPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookToolPostCall,
		payload,
		hookRuntime.DispatchToolPostCall,
	)
}

func (n *hooksNotifier) DispatchToolPostError(
	ctx context.Context,
	payload hookspkg.ToolPostErrorPayload,
) (hookspkg.ToolPostErrorPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookToolPostError,
		payload,
		hookRuntime.DispatchToolPostError,
	)
}

func (n *hooksNotifier) DispatchContextPreCompact(
	ctx context.Context,
	payload hookspkg.ContextPreCompactPayload,
) (hookspkg.ContextPreCompactPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookContextPreCompact,
		payload,
		hookRuntime.DispatchContextPreCompact,
	)
}

func (n *hooksNotifier) DispatchContextPostCompact(
	ctx context.Context,
	payload hookspkg.ContextPostCompactPayload,
) (hookspkg.ContextPostCompactPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookContextPostCompact,
		payload,
		hookRuntime.DispatchContextPostCompact,
	)
}
