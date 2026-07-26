package task

import (
	"context"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

// RunHookDispatcher is the narrow hook bridge consumed by task-run transitions.
type RunHookDispatcher interface {
	DispatchTaskBlocked(
		context.Context,
		hookspkg.TaskBlockedPayload,
	) (hookspkg.TaskBlockedPayload, error)
	DispatchTaskUnblocked(
		context.Context,
		hookspkg.TaskUnblockedPayload,
	) (hookspkg.TaskUnblockedPayload, error)
	DispatchTaskNeedsAttention(
		context.Context,
		hookspkg.TaskNeedsAttentionPayload,
	) (hookspkg.TaskNeedsAttentionPayload, error)
	DispatchTaskRecovered(
		context.Context,
		hookspkg.TaskRecoveredPayload,
	) (hookspkg.TaskRecoveredPayload, error)
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
}

type terminalHookDispatcher interface {
	DispatchLoopTerminal(
		context.Context,
		hookspkg.LoopTerminalPayload,
	) (hookspkg.LoopTerminalPayload, error)
}

var _ RunHookDispatcher = noopTaskRunHooks{}

type noopTaskRunHooks struct{}

func (noopTaskRunHooks) DispatchTaskBlocked(
	_ context.Context,
	payload hookspkg.TaskBlockedPayload,
) (hookspkg.TaskBlockedPayload, error) {
	return payload, nil
}

func (noopTaskRunHooks) DispatchTaskUnblocked(
	_ context.Context,
	payload hookspkg.TaskUnblockedPayload,
) (hookspkg.TaskUnblockedPayload, error) {
	return payload, nil
}

func (noopTaskRunHooks) DispatchTaskNeedsAttention(
	_ context.Context,
	payload hookspkg.TaskNeedsAttentionPayload,
) (hookspkg.TaskNeedsAttentionPayload, error) {
	return payload, nil
}

func (noopTaskRunHooks) DispatchTaskRecovered(
	_ context.Context,
	payload hookspkg.TaskRecoveredPayload,
) (hookspkg.TaskRecoveredPayload, error) {
	return payload, nil
}

func (noopTaskRunHooks) DispatchTaskStatusChanged(
	_ context.Context,
	payload hookspkg.TaskStatusChangedPayload,
) (hookspkg.TaskStatusChangedPayload, error) {
	return payload, nil
}

func (noopTaskRunHooks) DispatchTaskRunEnqueued(
	_ context.Context,
	payload hookspkg.TaskRunEnqueuedPayload,
) (hookspkg.TaskRunEnqueuedPayload, error) {
	return payload, nil
}

func (noopTaskRunHooks) DispatchTaskRunPreClaim(
	_ context.Context,
	payload hookspkg.TaskRunPreClaimPayload,
) (hookspkg.TaskRunPreClaimPayload, error) {
	return payload, nil
}

func (noopTaskRunHooks) DispatchTaskRunPostClaim(
	_ context.Context,
	payload hookspkg.TaskRunPostClaimPayload,
) (hookspkg.TaskRunPostClaimPayload, error) {
	return payload, nil
}

func (noopTaskRunHooks) DispatchTaskRunLeaseExtended(
	_ context.Context,
	payload hookspkg.TaskRunLeaseExtendedPayload,
) (hookspkg.TaskRunLeaseExtendedPayload, error) {
	return payload, nil
}

func (noopTaskRunHooks) DispatchTaskRunLeaseExpired(
	_ context.Context,
	payload hookspkg.TaskRunLeaseExpiredPayload,
) (hookspkg.TaskRunLeaseExpiredPayload, error) {
	return payload, nil
}

func (noopTaskRunHooks) DispatchTaskRunLeaseRecovered(
	_ context.Context,
	payload hookspkg.TaskRunLeaseRecoveredPayload,
) (hookspkg.TaskRunLeaseRecoveredPayload, error) {
	return payload, nil
}

func (noopTaskRunHooks) DispatchTaskRunReleased(
	_ context.Context,
	payload hookspkg.TaskRunReleasedPayload,
) (hookspkg.TaskRunReleasedPayload, error) {
	return payload, nil
}

func (noopTaskRunHooks) DispatchTaskRunCompleted(
	_ context.Context,
	payload hookspkg.TaskRunCompletedPayload,
) (hookspkg.TaskRunCompletedPayload, error) {
	return payload, nil
}

func (noopTaskRunHooks) DispatchTaskRunFailed(
	_ context.Context,
	payload hookspkg.TaskRunFailedPayload,
) (hookspkg.TaskRunFailedPayload, error) {
	return payload, nil
}

func defaultTaskRunHooks(hooks RunHookDispatcher) RunHookDispatcher {
	if hooks != nil {
		return hooks
	}
	return noopTaskRunHooks{}
}

func taskRunObservationHookContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.TODO()
	}
	if isCompletedSettlementPublicationContext(ctx) {
		return ctx
	}
	return context.WithoutCancel(ctx)
}

func taskRunLifecycleContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.TODO()
	}
	return context.WithoutCancel(ctx)
}
