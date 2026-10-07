package daemon

import (
	"context"

	"strings"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/session"
)

func (n *hooksNotifier) DispatchSpawnPreCreate(
	ctx context.Context,
	payload hookspkg.SpawnPreCreatePayload,
) (hookspkg.SpawnPreCreatePayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookSpawnPreCreate,
		payload,
		hookRuntime.DispatchSpawnPreCreate,
	)
}

func (n *hooksNotifier) DispatchSpawnCreated(
	ctx context.Context,
	payload hookspkg.SpawnCreatedPayload,
) (hookspkg.SpawnCreatedPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookSpawnCreated,
		payload,
		hookRuntime.DispatchSpawnCreated,
	)
}

func (n *hooksNotifier) DispatchSpawnParentStopped(
	ctx context.Context,
	payload hookspkg.SpawnParentStoppedPayload,
) (hookspkg.SpawnParentStoppedPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookSpawnParentStopped,
		payload,
		hookRuntime.DispatchSpawnParentStopped,
	)
}

func (n *hooksNotifier) DispatchSpawnTTLExpired(
	ctx context.Context,
	payload hookspkg.SpawnTTLExpiredPayload,
) (hookspkg.SpawnTTLExpiredPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookSpawnTTLExpired,
		payload,
		hookRuntime.DispatchSpawnTTLExpired,
	)
}

func (n *hooksNotifier) DispatchSpawnReaped(
	ctx context.Context,
	payload hookspkg.SpawnReapedPayload,
) (hookspkg.SpawnReapedPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookSpawnReaped,
		payload,
		hookRuntime.DispatchSpawnReaped,
	)
}

func (n *hooksNotifier) DispatchAgentSoulSnapshotResolved(
	ctx context.Context,
	payload hookspkg.AgentSoulSnapshotResolvedPayload,
) (hookspkg.AgentSoulSnapshotResolvedPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookAgentSoulSnapshotResolved,
		payload,
		hookRuntime.DispatchAgentSoulSnapshotResolved,
	)
}

func (n *hooksNotifier) DispatchAgentSoulMutationAfter(
	ctx context.Context,
	payload hookspkg.AgentSoulMutationAfterPayload,
) (hookspkg.AgentSoulMutationAfterPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookAgentSoulMutationAfter,
		payload,
		hookRuntime.DispatchAgentSoulMutationAfter,
	)
}

func (n *hooksNotifier) DispatchAgentHeartbeatPolicyResolved(
	ctx context.Context,
	payload hookspkg.AgentHeartbeatPolicyResolvedPayload,
) (hookspkg.AgentHeartbeatPolicyResolvedPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookAgentHeartbeatPolicyResolved,
		payload,
		hookRuntime.DispatchAgentHeartbeatPolicyResolved,
	)
}

func (n *hooksNotifier) DispatchAgentHeartbeatWakeBefore(
	ctx context.Context,
	payload hookspkg.AgentHeartbeatWakeBeforePayload,
) (hookspkg.AgentHeartbeatWakeBeforePayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookAgentHeartbeatWakeBefore,
		payload,
		hookRuntime.DispatchAgentHeartbeatWakeBefore,
	)
}

func (n *hooksNotifier) DispatchAgentHeartbeatWakeAfter(
	ctx context.Context,
	payload hookspkg.AgentHeartbeatWakeAfterPayload,
) (hookspkg.AgentHeartbeatWakeAfterPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookAgentHeartbeatWakeAfter,
		payload,
		hookRuntime.DispatchAgentHeartbeatWakeAfter,
	)
}

func (n *hooksNotifier) DispatchSessionHealthUpdateAfter(
	ctx context.Context,
	payload hookspkg.SessionHealthUpdateAfterPayload,
) (hookspkg.SessionHealthUpdateAfterPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookSessionHealthUpdateAfter,
		payload,
		hookRuntime.DispatchSessionHealthUpdateAfter,
	)
}

func (n *hooksNotifier) DispatchSessionAttentionChanged(
	ctx context.Context,
	payload hookspkg.SessionAttentionChangedPayload,
) (hookspkg.SessionAttentionChangedPayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookSessionAttentionChanged,
		payload,
		hookRuntime.DispatchSessionAttentionChanged,
	)
}

func (n *hooksNotifier) OnAgentEvent(ctx context.Context, sessionID string, event any) {
	target := strings.TrimSpace(sessionID)
	n.logger.ErrorContext(
		ctx,
		"daemon: refusing hook dispatch without session workspace identity",
		"session_id",
		target,
	)
	_, agentEventNotify := n.runtime()
	if agentEventNotify != nil {
		agentEventNotify.OnAgentEvent(ctx, target, event)
	}
}

func (n *hooksNotifier) OnAgentEventForSession(ctx context.Context, sess *session.Session, event any) {
	n.dispatchAgentEvent(ctx, sess, event)
}
