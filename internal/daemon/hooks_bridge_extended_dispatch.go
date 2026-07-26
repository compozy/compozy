package daemon

import (
	"context"

	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/session"
)

func (n *hooksNotifier) DispatchSpawnPreCreate(
	ctx context.Context,
	payload hookspkg.SpawnPreCreatePayload,
) (hookspkg.SpawnPreCreatePayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookSpawnPreCreate,
		payload,
		hookRuntime.DispatchSpawnPreCreate,
	)
}

func (n *hooksNotifier) DispatchSpawnCreated(
	ctx context.Context,
	payload hookspkg.SpawnCreatedPayload,
) (hookspkg.SpawnCreatedPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookSpawnCreated,
		payload,
		hookRuntime.DispatchSpawnCreated,
	)
}

func (n *hooksNotifier) DispatchSpawnParentStopped(
	ctx context.Context,
	payload hookspkg.SpawnParentStoppedPayload,
) (hookspkg.SpawnParentStoppedPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookSpawnParentStopped,
		payload,
		hookRuntime.DispatchSpawnParentStopped,
	)
}

func (n *hooksNotifier) DispatchSpawnTTLExpired(
	ctx context.Context,
	payload hookspkg.SpawnTTLExpiredPayload,
) (hookspkg.SpawnTTLExpiredPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookSpawnTTLExpired,
		payload,
		hookRuntime.DispatchSpawnTTLExpired,
	)
}

func (n *hooksNotifier) DispatchSpawnReaped(
	ctx context.Context,
	payload hookspkg.SpawnReapedPayload,
) (hookspkg.SpawnReapedPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookSpawnReaped,
		payload,
		hookRuntime.DispatchSpawnReaped,
	)
}

func (n *hooksNotifier) DispatchAgentSoulSnapshotResolved(
	ctx context.Context,
	payload hookspkg.AgentSoulSnapshotResolvedPayload,
) (hookspkg.AgentSoulSnapshotResolvedPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookAgentSoulSnapshotResolved,
		payload,
		hookRuntime.DispatchAgentSoulSnapshotResolved,
	)
}

func (n *hooksNotifier) DispatchAgentSoulMutationAfter(
	ctx context.Context,
	payload hookspkg.AgentSoulMutationAfterPayload,
) (hookspkg.AgentSoulMutationAfterPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookAgentSoulMutationAfter,
		payload,
		hookRuntime.DispatchAgentSoulMutationAfter,
	)
}

func (n *hooksNotifier) DispatchAgentHeartbeatPolicyResolved(
	ctx context.Context,
	payload hookspkg.AgentHeartbeatPolicyResolvedPayload,
) (hookspkg.AgentHeartbeatPolicyResolvedPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookAgentHeartbeatPolicyResolved,
		payload,
		hookRuntime.DispatchAgentHeartbeatPolicyResolved,
	)
}

func (n *hooksNotifier) DispatchAgentHeartbeatWakeBefore(
	ctx context.Context,
	payload hookspkg.AgentHeartbeatWakeBeforePayload,
) (hookspkg.AgentHeartbeatWakeBeforePayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookAgentHeartbeatWakeBefore,
		payload,
		hookRuntime.DispatchAgentHeartbeatWakeBefore,
	)
}

func (n *hooksNotifier) DispatchAgentHeartbeatWakeAfter(
	ctx context.Context,
	payload hookspkg.AgentHeartbeatWakeAfterPayload,
) (hookspkg.AgentHeartbeatWakeAfterPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookAgentHeartbeatWakeAfter,
		payload,
		hookRuntime.DispatchAgentHeartbeatWakeAfter,
	)
}

func (n *hooksNotifier) DispatchSessionHealthUpdateAfter(
	ctx context.Context,
	payload hookspkg.SessionHealthUpdateAfterPayload,
) (hookspkg.SessionHealthUpdateAfterPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookSessionHealthUpdateAfter,
		payload,
		hookRuntime.DispatchSessionHealthUpdateAfter,
	)
}

func (n *hooksNotifier) DispatchNetworkPeerJoined(
	ctx context.Context,
	payload hookspkg.NetworkPeerJoinedPayload,
) (hookspkg.NetworkPeerJoinedPayload, error) {
	return dispatchRuntime(ctx, n, hookspkg.HookNetworkPeerJoined, payload, hookRuntime.DispatchNetworkPeerJoined)
}

func (n *hooksNotifier) DispatchNetworkPeerLeft(
	ctx context.Context,
	payload hookspkg.NetworkPeerLeftPayload,
) (hookspkg.NetworkPeerLeftPayload, error) {
	return dispatchRuntime(ctx, n, hookspkg.HookNetworkPeerLeft, payload, hookRuntime.DispatchNetworkPeerLeft)
}

func (n *hooksNotifier) DispatchNetworkThreadOpened(
	ctx context.Context,
	payload hookspkg.NetworkThreadOpenedPayload,
) (hookspkg.NetworkThreadOpenedPayload, error) {
	return dispatchNetworkThreadOpenedWithWatchObservers(ctx, n, payload)
}

func (n *hooksNotifier) DispatchNetworkDirectRoomOpened(
	ctx context.Context,
	payload hookspkg.NetworkDirectRoomOpenedPayload,
) (hookspkg.NetworkDirectRoomOpenedPayload, error) {
	return dispatchNetworkDirectRoomOpenedWithWatchObservers(ctx, n, payload)
}

func (n *hooksNotifier) DispatchNetworkMessagePersisted(
	ctx context.Context,
	payload hookspkg.NetworkMessagePersistedPayload,
) (hookspkg.NetworkMessagePersistedPayload, error) {
	return dispatchNetworkMessagePersistedWithWatchObservers(ctx, n, payload)
}

func (n *hooksNotifier) DispatchNetworkWorkOpened(
	ctx context.Context,
	payload hookspkg.NetworkWorkOpenedPayload,
) (hookspkg.NetworkWorkOpenedPayload, error) {
	return dispatchNetworkWorkOpenedWithWatchObservers(ctx, n, payload)
}

func (n *hooksNotifier) DispatchNetworkWorkTransitioned(
	ctx context.Context,
	payload hookspkg.NetworkWorkTransitionedPayload,
) (hookspkg.NetworkWorkTransitionedPayload, error) {
	return dispatchNetworkWorkTransitionedWithWatchObservers(ctx, n, payload)
}

func (n *hooksNotifier) DispatchNetworkWorkClosed(
	ctx context.Context,
	payload hookspkg.NetworkWorkClosedPayload,
) (hookspkg.NetworkWorkClosedPayload, error) {
	return dispatchNetworkWorkClosedWithWatchObservers(ctx, n, payload)
}

func (n *hooksNotifier) OnAgentEvent(ctx context.Context, sessionID string, event any) {
	n.dispatchAgentEvent(ctx, hookspkg.SessionContext{SessionID: strings.TrimSpace(sessionID)}, event)
}

func (n *hooksNotifier) OnAgentEventForSession(ctx context.Context, sess *session.Session, event any) {
	n.dispatchAgentEvent(ctx, hookSessionContext(sess), event)
}

func (n *hooksNotifier) OnSandboxLifecycleEvent(ctx context.Context, event session.SandboxLifecycleEvent) {
	_, agentEventNotify := n.runtime()
	if notifier, ok := agentEventNotify.(session.SandboxLifecycleNotifier); ok {
		notifier.OnSandboxLifecycleEvent(ctx, event)
	}
}
