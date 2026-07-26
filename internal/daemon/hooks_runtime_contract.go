package daemon

import (
	"context"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

const (
	hooksBridgeWorkspaceIDKey = daemonWorkspaceIDKey
)

type hookRuntime interface {
	Close()
	Version() int64
	DispatchSessionPreCreate(
		context.Context,
		hookspkg.SessionPreCreatePayload,
	) (hookspkg.SessionPreCreatePayload, error)
	DispatchSessionPostCreate(
		context.Context,
		hookspkg.SessionPostCreatePayload,
	) (hookspkg.SessionPostCreatePayload, error)
	DispatchSessionPreResume(
		context.Context,
		hookspkg.SessionPreResumePayload,
	) (hookspkg.SessionPreResumePayload, error)
	DispatchSessionPostResume(
		context.Context,
		hookspkg.SessionPostResumePayload,
	) (hookspkg.SessionPostResumePayload, error)
	DispatchSessionPreStop(context.Context, hookspkg.SessionPreStopPayload) (hookspkg.SessionPreStopPayload, error)
	DispatchSessionPostStop(context.Context, hookspkg.SessionPostStopPayload) (hookspkg.SessionPostStopPayload, error)
	DispatchSandboxPrepare(
		context.Context,
		hookspkg.SandboxPreparePayload,
	) (hookspkg.SandboxPreparePayload, error)
	DispatchSandboxReady(
		context.Context,
		hookspkg.SandboxReadyPayload,
	) (hookspkg.SandboxReadyPayload, error)
	DispatchSandboxSyncBefore(
		context.Context,
		hookspkg.SandboxSyncBeforePayload,
	) (hookspkg.SandboxSyncBeforePayload, error)
	DispatchSandboxSyncAfter(
		context.Context,
		hookspkg.SandboxSyncAfterPayload,
	) (hookspkg.SandboxSyncAfterPayload, error)
	DispatchSandboxStop(context.Context, hookspkg.SandboxStopPayload) (hookspkg.SandboxStopPayload, error)
	DispatchInputPreSubmit(context.Context, hookspkg.InputPreSubmitPayload) (hookspkg.InputPreSubmitPayload, error)
	DispatchPromptPostAssemble(context.Context, hookspkg.PromptPayload) (hookspkg.PromptPayload, error)
	DispatchEventPreRecord(context.Context, hookspkg.EventPreRecordPayload) (hookspkg.EventPreRecordPayload, error)
	DispatchEventPostRecord(context.Context, hookspkg.EventPostRecordPayload) (hookspkg.EventPostRecordPayload, error)
	DispatchAutomationJobPreFire(
		context.Context,
		hookspkg.AutomationJobPreFirePayload,
	) (hookspkg.AutomationJobPreFirePayload, error)
	DispatchAutomationJobPostFire(
		context.Context,
		hookspkg.AutomationJobPostFirePayload,
	) (hookspkg.AutomationJobPostFirePayload, error)
	DispatchAutomationTriggerPreFire(
		context.Context,
		hookspkg.AutomationTriggerPreFirePayload,
	) (hookspkg.AutomationTriggerPreFirePayload, error)
	DispatchAutomationTriggerPostFire(
		context.Context,
		hookspkg.AutomationTriggerPostFirePayload,
	) (hookspkg.AutomationTriggerPostFirePayload, error)
	DispatchAutomationRunCompleted(
		context.Context,
		hookspkg.AutomationRunCompletedPayload,
	) (hookspkg.AutomationRunCompletedPayload, error)
	DispatchAutomationRunFailed(
		context.Context,
		hookspkg.AutomationRunFailedPayload,
	) (hookspkg.AutomationRunFailedPayload, error)
	DispatchAgentPreStart(context.Context, hookspkg.AgentPreStartPayload) (hookspkg.AgentPreStartPayload, error)
	DispatchAgentSpawned(context.Context, hookspkg.AgentSpawnedPayload) (hookspkg.AgentSpawnedPayload, error)
	DispatchAgentCrashed(context.Context, hookspkg.AgentCrashedPayload) (hookspkg.AgentCrashedPayload, error)
	DispatchAgentStopped(context.Context, hookspkg.AgentStoppedPayload) (hookspkg.AgentStoppedPayload, error)
	DispatchTurnStart(context.Context, hookspkg.TurnStartPayload) (hookspkg.TurnStartPayload, error)
	DispatchTurnEnd(context.Context, hookspkg.TurnEndPayload) (hookspkg.TurnEndPayload, error)
	DispatchMessageStart(context.Context, hookspkg.MessageStartPayload) (hookspkg.MessageStartPayload, error)
	DispatchMessageDelta(context.Context, hookspkg.MessageDeltaPayload) (hookspkg.MessageDeltaPayload, error)
	DispatchMessageEnd(context.Context, hookspkg.MessageEndPayload) (hookspkg.MessageEndPayload, error)
	DispatchSessionMessagePersisted(
		context.Context,
		hookspkg.SessionMessagePersistedPayload,
	) (hookspkg.SessionMessagePersistedPayload, error)
	DispatchToolPreCall(context.Context, hookspkg.ToolPreCallPayload) (hookspkg.ToolPreCallPayload, error)
	DispatchToolPostCall(context.Context, hookspkg.ToolPostCallPayload) (hookspkg.ToolPostCallPayload, error)
	DispatchToolPostError(context.Context, hookspkg.ToolPostErrorPayload) (hookspkg.ToolPostErrorPayload, error)
	DispatchPermissionRequest(
		context.Context,
		hookspkg.PermissionRequestPayload,
	) (hookspkg.PermissionRequestPayload, error)
	DispatchPermissionResolved(
		context.Context,
		hookspkg.PermissionResolvedPayload,
	) (hookspkg.PermissionResolvedPayload, error)
	DispatchPermissionDenied(
		context.Context,
		hookspkg.PermissionDeniedPayload,
	) (hookspkg.PermissionDeniedPayload, error)
	DispatchContextPreCompact(
		context.Context,
		hookspkg.ContextPreCompactPayload,
	) (hookspkg.ContextPreCompactPayload, error)
	DispatchContextPostCompact(
		context.Context,
		hookspkg.ContextPostCompactPayload,
	) (hookspkg.ContextPostCompactPayload, error)
	DispatchCoordinatorPreSpawn(
		context.Context,
		hookspkg.CoordinatorPreSpawnPayload,
	) (hookspkg.CoordinatorPreSpawnPayload, error)
	DispatchCoordinatorSpawned(
		context.Context,
		hookspkg.CoordinatorSpawnedPayload,
	) (hookspkg.CoordinatorSpawnedPayload, error)
	DispatchCoordinatorDecision(
		context.Context,
		hookspkg.CoordinatorDecisionPayload,
	) (hookspkg.CoordinatorDecisionPayload, error)
	DispatchCoordinatorStopped(
		context.Context,
		hookspkg.CoordinatorStoppedPayload,
	) (hookspkg.CoordinatorStoppedPayload, error)
	DispatchCoordinatorFailed(
		context.Context,
		hookspkg.CoordinatorFailedPayload,
	) (hookspkg.CoordinatorFailedPayload, error)
	DispatchTaskBlocked(context.Context, hookspkg.TaskBlockedPayload) (hookspkg.TaskBlockedPayload, error)
	DispatchTaskUnblocked(context.Context, hookspkg.TaskUnblockedPayload) (hookspkg.TaskUnblockedPayload, error)
	DispatchTaskNeedsAttention(
		context.Context,
		hookspkg.TaskNeedsAttentionPayload,
	) (hookspkg.TaskNeedsAttentionPayload, error)
	DispatchTaskRecovered(context.Context, hookspkg.TaskRecoveredPayload) (hookspkg.TaskRecoveredPayload, error)
	DispatchTaskStatusChanged(
		context.Context,
		hookspkg.TaskStatusChangedPayload,
	) (hookspkg.TaskStatusChangedPayload, error)
	DispatchTaskRunEnqueued(
		context.Context,
		hookspkg.TaskRunEnqueuedPayload,
	) (hookspkg.TaskRunEnqueuedPayload, error)
	DispatchTaskRunPreClaim(
		context.Context,
		hookspkg.TaskRunPreClaimPayload,
	) (hookspkg.TaskRunPreClaimPayload, error)
	DispatchTaskRunPostClaim(
		context.Context,
		hookspkg.TaskRunPostClaimPayload,
	) (hookspkg.TaskRunPostClaimPayload, error)
	DispatchTaskRunLeaseExtended(
		context.Context,
		hookspkg.TaskRunLeaseExtendedPayload,
	) (hookspkg.TaskRunLeaseExtendedPayload, error)
	DispatchTaskRunLeaseExpired(
		context.Context,
		hookspkg.TaskRunLeaseExpiredPayload,
	) (hookspkg.TaskRunLeaseExpiredPayload, error)
	DispatchTaskRunLeaseRecovered(
		context.Context,
		hookspkg.TaskRunLeaseRecoveredPayload,
	) (hookspkg.TaskRunLeaseRecoveredPayload, error)
	DispatchTaskRunReleased(
		context.Context,
		hookspkg.TaskRunReleasedPayload,
	) (hookspkg.TaskRunReleasedPayload, error)
	DispatchTaskRunCompleted(
		context.Context,
		hookspkg.TaskRunCompletedPayload,
	) (hookspkg.TaskRunCompletedPayload, error)
	DispatchTaskRunFailed(
		context.Context,
		hookspkg.TaskRunFailedPayload,
	) (hookspkg.TaskRunFailedPayload, error)
	DispatchLoopStarted(
		context.Context,
		hookspkg.LoopStartedPayload,
	) (hookspkg.LoopStartedPayload, error)
	DispatchLoopGenerationPre(
		context.Context,
		hookspkg.LoopGenerationPrePayload,
	) (hookspkg.LoopGenerationPrePayload, error)
	DispatchLoopGenerationPost(
		context.Context,
		hookspkg.LoopGenerationPostPayload,
	) (hookspkg.LoopGenerationPostPayload, error)
	DispatchLoopGatePre(
		context.Context,
		hookspkg.LoopGatePrePayload,
	) (hookspkg.LoopGatePrePayload, error)
	DispatchLoopGatePost(
		context.Context,
		hookspkg.LoopGatePostPayload,
	) (hookspkg.LoopGatePostPayload, error)
	DispatchLoopNodeTerminal(
		context.Context,
		hookspkg.LoopNodeTerminalPayload,
	) (hookspkg.LoopNodeTerminalPayload, error)
	DispatchLoopTerminal(
		context.Context,
		hookspkg.LoopTerminalPayload,
	) (hookspkg.LoopTerminalPayload, error)
	DispatchSpawnPreCreate(
		context.Context,
		hookspkg.SpawnPreCreatePayload,
	) (hookspkg.SpawnPreCreatePayload, error)
	DispatchSpawnCreated(
		context.Context,
		hookspkg.SpawnCreatedPayload,
	) (hookspkg.SpawnCreatedPayload, error)
	DispatchSpawnParentStopped(
		context.Context,
		hookspkg.SpawnParentStoppedPayload,
	) (hookspkg.SpawnParentStoppedPayload, error)
	DispatchSpawnTTLExpired(
		context.Context,
		hookspkg.SpawnTTLExpiredPayload,
	) (hookspkg.SpawnTTLExpiredPayload, error)
	DispatchSpawnReaped(
		context.Context,
		hookspkg.SpawnReapedPayload,
	) (hookspkg.SpawnReapedPayload, error)
	DispatchAgentSoulSnapshotResolved(
		context.Context,
		hookspkg.AgentSoulSnapshotResolvedPayload,
	) (hookspkg.AgentSoulSnapshotResolvedPayload, error)
	DispatchAgentSoulMutationAfter(
		context.Context,
		hookspkg.AgentSoulMutationAfterPayload,
	) (hookspkg.AgentSoulMutationAfterPayload, error)
	DispatchAgentHeartbeatPolicyResolved(
		context.Context,
		hookspkg.AgentHeartbeatPolicyResolvedPayload,
	) (hookspkg.AgentHeartbeatPolicyResolvedPayload, error)
	DispatchAgentHeartbeatWakeBefore(
		context.Context,
		hookspkg.AgentHeartbeatWakeBeforePayload,
	) (hookspkg.AgentHeartbeatWakeBeforePayload, error)
	DispatchAgentHeartbeatWakeAfter(
		context.Context,
		hookspkg.AgentHeartbeatWakeAfterPayload,
	) (hookspkg.AgentHeartbeatWakeAfterPayload, error)
	DispatchSessionHealthUpdateAfter(
		context.Context,
		hookspkg.SessionHealthUpdateAfterPayload,
	) (hookspkg.SessionHealthUpdateAfterPayload, error)
	DispatchNetworkPeerJoined(
		context.Context,
		hookspkg.NetworkPeerJoinedPayload,
	) (hookspkg.NetworkPeerJoinedPayload, error)
	DispatchNetworkPeerLeft(
		context.Context,
		hookspkg.NetworkPeerLeftPayload,
	) (hookspkg.NetworkPeerLeftPayload, error)
	DispatchNetworkThreadOpened(
		context.Context,
		hookspkg.NetworkThreadOpenedPayload,
	) (hookspkg.NetworkThreadOpenedPayload, error)
	DispatchNetworkDirectRoomOpened(
		context.Context,
		hookspkg.NetworkDirectRoomOpenedPayload,
	) (hookspkg.NetworkDirectRoomOpenedPayload, error)
	DispatchNetworkMessagePersisted(
		context.Context,
		hookspkg.NetworkMessagePersistedPayload,
	) (hookspkg.NetworkMessagePersistedPayload, error)
	DispatchNetworkWorkOpened(
		context.Context,
		hookspkg.NetworkWorkOpenedPayload,
	) (hookspkg.NetworkWorkOpenedPayload, error)
	DispatchNetworkWorkTransitioned(
		context.Context,
		hookspkg.NetworkWorkTransitionedPayload,
	) (hookspkg.NetworkWorkTransitionedPayload, error)
	DispatchNetworkWorkClosed(
		context.Context,
		hookspkg.NetworkWorkClosedPayload,
	) (hookspkg.NetworkWorkClosedPayload, error)
}
