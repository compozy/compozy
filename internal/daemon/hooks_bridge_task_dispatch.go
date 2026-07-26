package daemon

import (
	"context"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func (n *hooksNotifier) DispatchCoordinatorPreSpawn(
	ctx context.Context,
	payload hookspkg.CoordinatorPreSpawnPayload,
) (hookspkg.CoordinatorPreSpawnPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookCoordinatorPreSpawn,
		payload,
		hookRuntime.DispatchCoordinatorPreSpawn,
	)
}

func (n *hooksNotifier) DispatchCoordinatorSpawned(
	ctx context.Context,
	payload hookspkg.CoordinatorSpawnedPayload,
) (hookspkg.CoordinatorSpawnedPayload, error) {
	return dispatchCoordinatorSpawnedWithWatchObservers(ctx, n, payload)
}

func (n *hooksNotifier) DispatchCoordinatorDecision(
	ctx context.Context,
	payload hookspkg.CoordinatorDecisionPayload,
) (hookspkg.CoordinatorDecisionPayload, error) {
	return dispatchCoordinatorDecisionWithWatchObservers(ctx, n, payload)
}

func (n *hooksNotifier) DispatchCoordinatorStopped(
	ctx context.Context,
	payload hookspkg.CoordinatorStoppedPayload,
) (hookspkg.CoordinatorStoppedPayload, error) {
	return dispatchCoordinatorStoppedWithWatchObservers(ctx, n, payload)
}

func (n *hooksNotifier) DispatchCoordinatorFailed(
	ctx context.Context,
	payload hookspkg.CoordinatorFailedPayload,
) (hookspkg.CoordinatorFailedPayload, error) {
	return dispatchCoordinatorFailedWithWatchObservers(ctx, n, payload)
}

func (n *hooksNotifier) DispatchTaskBlocked(
	ctx context.Context,
	payload hookspkg.TaskBlockedPayload,
) (hookspkg.TaskBlockedPayload, error) {
	return dispatchTaskBlockedWithWatchObservers(ctx, n, payload)
}

func (n *hooksNotifier) DispatchTaskUnblocked(
	ctx context.Context,
	payload hookspkg.TaskUnblockedPayload,
) (hookspkg.TaskUnblockedPayload, error) {
	return dispatchTaskUnblockedWithWatchObservers(ctx, n, payload)
}

func (n *hooksNotifier) DispatchTaskNeedsAttention(
	ctx context.Context,
	payload hookspkg.TaskNeedsAttentionPayload,
) (hookspkg.TaskNeedsAttentionPayload, error) {
	return dispatchTaskNeedsAttentionWithWatchObservers(ctx, n, payload)
}

func (n *hooksNotifier) DispatchTaskRecovered(
	ctx context.Context,
	payload hookspkg.TaskRecoveredPayload,
) (hookspkg.TaskRecoveredPayload, error) {
	return dispatchTaskRecoveredWithWatchObservers(ctx, n, payload)
}

func (n *hooksNotifier) DispatchTaskRunEnqueued(
	ctx context.Context,
	payload hookspkg.TaskRunEnqueuedPayload,
) (hookspkg.TaskRunEnqueuedPayload, error) {
	result, err := dispatchRuntime(
		ctx,
		n,
		hookspkg.HookTaskRunEnqueued,
		payload,
		hookRuntime.DispatchTaskRunEnqueued,
	)
	if err != nil {
		return result, err
	}
	for _, observer := range n.taskRunEnqueuedObservers() {
		observer.OnTaskRunEnqueued(ctx, result)
	}
	return result, nil
}

func (n *hooksNotifier) DispatchTaskRunPreClaim(
	ctx context.Context,
	payload hookspkg.TaskRunPreClaimPayload,
) (hookspkg.TaskRunPreClaimPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookTaskRunPreClaim,
		payload,
		hookRuntime.DispatchTaskRunPreClaim,
	)
}

func (n *hooksNotifier) DispatchTaskRunPostClaim(
	ctx context.Context,
	payload hookspkg.TaskRunPostClaimPayload,
) (hookspkg.TaskRunPostClaimPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookTaskRunPostClaim,
		payload,
		hookRuntime.DispatchTaskRunPostClaim,
	)
}

func (n *hooksNotifier) DispatchTaskRunLeaseRecovered(
	ctx context.Context,
	payload hookspkg.TaskRunLeaseRecoveredPayload,
) (hookspkg.TaskRunLeaseRecoveredPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookTaskRunLeaseRecovered,
		payload,
		hookRuntime.DispatchTaskRunLeaseRecovered,
	)
}

func (n *hooksNotifier) DispatchTaskRunLeaseExtended(
	ctx context.Context,
	payload hookspkg.TaskRunLeaseExtendedPayload,
) (hookspkg.TaskRunLeaseExtendedPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookTaskRunLeaseExtended,
		payload,
		hookRuntime.DispatchTaskRunLeaseExtended,
	)
}

func (n *hooksNotifier) DispatchTaskRunLeaseExpired(
	ctx context.Context,
	payload hookspkg.TaskRunLeaseExpiredPayload,
) (hookspkg.TaskRunLeaseExpiredPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookTaskRunLeaseExpired,
		payload,
		hookRuntime.DispatchTaskRunLeaseExpired,
	)
}

func (n *hooksNotifier) DispatchTaskRunReleased(
	ctx context.Context,
	payload hookspkg.TaskRunReleasedPayload,
) (hookspkg.TaskRunReleasedPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookTaskRunReleased,
		payload,
		hookRuntime.DispatchTaskRunReleased,
	)
}

func (n *hooksNotifier) DispatchTaskRunCompleted(
	ctx context.Context,
	payload hookspkg.TaskRunCompletedPayload,
) (hookspkg.TaskRunCompletedPayload, error) {
	result, err := dispatchRuntime(
		ctx,
		n,
		hookspkg.HookTaskRunCompleted,
		payload,
		hookRuntime.DispatchTaskRunCompleted,
	)
	n.notifyTaskRunTerminalObservers(ctx, result)
	return result, err
}

func (n *hooksNotifier) DispatchTaskRunFailed(
	ctx context.Context,
	payload hookspkg.TaskRunFailedPayload,
) (hookspkg.TaskRunFailedPayload, error) {
	result, err := dispatchRuntime(
		ctx,
		n,
		hookspkg.HookTaskRunFailed,
		payload,
		hookRuntime.DispatchTaskRunFailed,
	)
	n.notifyTaskRunTerminalObservers(ctx, result)
	return result, err
}

func (n *hooksNotifier) DispatchLoopStarted(
	ctx context.Context,
	payload hookspkg.LoopStartedPayload,
) (hookspkg.LoopStartedPayload, error) {
	result, err := dispatchRuntime(
		ctx,
		n,
		hookspkg.HookLoopStarted,
		payload,
		hookRuntime.DispatchLoopStarted,
	)
	if err != nil {
		return result, err
	}
	n.notifyLoopStartedObservers(ctx, result)
	return result, nil
}

func (n *hooksNotifier) DispatchLoopGenerationPre(
	ctx context.Context,
	payload hookspkg.LoopGenerationPrePayload,
) (hookspkg.LoopGenerationPrePayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookLoopGenerationPre,
		payload,
		hookRuntime.DispatchLoopGenerationPre,
	)
}

func (n *hooksNotifier) DispatchLoopGenerationPost(
	ctx context.Context,
	payload hookspkg.LoopGenerationPostPayload,
) (hookspkg.LoopGenerationPostPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookLoopGenerationPost,
		payload,
		hookRuntime.DispatchLoopGenerationPost,
	)
}

func (n *hooksNotifier) DispatchLoopGatePre(
	ctx context.Context,
	payload hookspkg.LoopGatePrePayload,
) (hookspkg.LoopGatePrePayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookLoopGatePre,
		payload,
		hookRuntime.DispatchLoopGatePre,
	)
}

func (n *hooksNotifier) DispatchLoopGatePost(
	ctx context.Context,
	payload hookspkg.LoopGatePostPayload,
) (hookspkg.LoopGatePostPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookLoopGatePost,
		payload,
		hookRuntime.DispatchLoopGatePost,
	)
}

func (n *hooksNotifier) DispatchLoopNodeTerminal(
	ctx context.Context,
	payload hookspkg.LoopNodeTerminalPayload,
) (hookspkg.LoopNodeTerminalPayload, error) {
	return dispatchLoopNodeTerminalWithWatchObservers(ctx, n, payload)
}

func (n *hooksNotifier) DispatchLoopTerminal(
	ctx context.Context,
	payload hookspkg.LoopTerminalPayload,
) (hookspkg.LoopTerminalPayload, error) {
	result, err := dispatchRuntime(
		ctx,
		n,
		hookspkg.HookLoopTerminal,
		payload,
		hookRuntime.DispatchLoopTerminal,
	)
	n.notifyLoopTerminalObservers(ctx, result)
	return result, err
}
