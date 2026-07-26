package hooks

import "context"

// DispatchTaskBlocked runs the task.blocked hook dispatch.
func (h *Hooks) DispatchTaskBlocked(
	ctx context.Context,
	payload TaskBlockedPayload,
) (TaskBlockedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookTaskBlocked,
		payload,
		dispatchConfig[TaskBlockedPayload, TaskObservationPatch]{
			match: matchTaskBlock,
			apply: applyNoop[TaskBlockedPayload, TaskObservationPatch],
		},
	)
}

// DispatchTaskUnblocked runs the task.unblocked hook dispatch.
func (h *Hooks) DispatchTaskUnblocked(
	ctx context.Context,
	payload TaskUnblockedPayload,
) (TaskUnblockedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookTaskUnblocked,
		payload,
		dispatchConfig[TaskUnblockedPayload, TaskObservationPatch]{
			match: matchTaskBlock,
			apply: applyNoop[TaskUnblockedPayload, TaskObservationPatch],
		},
	)
}

// DispatchTaskNeedsAttention runs the task.needs_attention hook dispatch.
func (h *Hooks) DispatchTaskNeedsAttention(
	ctx context.Context,
	payload TaskNeedsAttentionPayload,
) (TaskNeedsAttentionPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookTaskNeedsAttention,
		payload,
		dispatchConfig[TaskNeedsAttentionPayload, TaskObservationPatch]{
			match: matchTaskAttention,
			apply: applyNoop[TaskNeedsAttentionPayload, TaskObservationPatch],
		},
	)
}

// DispatchTaskRecovered runs the task.recovered hook dispatch.
func (h *Hooks) DispatchTaskRecovered(
	ctx context.Context,
	payload TaskRecoveredPayload,
) (TaskRecoveredPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookTaskRecovered,
		payload,
		dispatchConfig[TaskRecoveredPayload, TaskObservationPatch]{
			match: matchTaskAttention,
			apply: applyNoop[TaskRecoveredPayload, TaskObservationPatch],
		},
	)
}

// DispatchTaskRunEnqueued runs the task.run.enqueued hook dispatch.
func (h *Hooks) DispatchTaskRunEnqueued(
	ctx context.Context,
	payload TaskRunEnqueuedPayload,
) (TaskRunEnqueuedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookTaskRunEnqueued,
		payload,
		dispatchConfig[TaskRunEnqueuedPayload, TaskRunObservationPatch]{
			match: matchTaskRunEnqueued,
			apply: applyNoop[TaskRunEnqueuedPayload, TaskRunObservationPatch],
		},
	)
}

// DispatchTaskRunPreClaim runs the task.run.pre_claim hook pipeline.
func (h *Hooks) DispatchTaskRunPreClaim(
	ctx context.Context,
	payload TaskRunPreClaimPayload,
) (TaskRunPreClaimPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookTaskRunPreClaim,
		payload,
		dispatchConfig[TaskRunPreClaimPayload, TaskRunPreClaimPatch]{
			match:  matchTaskRunPreClaim,
			apply:  applyTaskRunPreClaimPatch,
			denied: taskRunPreClaimPatchDenied,
			denyErr: func(_ TaskRunPreClaimPayload, report dispatchReport) error {
				return hookDeniedError(HookTaskRunPreClaim, report.DenyReason)
			},
			guard: guardTaskRunPreClaimPatch,
		},
	)
}

// DispatchTaskRunPostClaim runs the task.run.post_claim hook dispatch.
func (h *Hooks) DispatchTaskRunPostClaim(
	ctx context.Context,
	payload TaskRunPostClaimPayload,
) (TaskRunPostClaimPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookTaskRunPostClaim,
		payload,
		dispatchConfig[TaskRunPostClaimPayload, TaskRunObservationPatch]{
			match: matchTaskRunPostClaim,
			apply: applyNoop[TaskRunPostClaimPayload, TaskRunObservationPatch],
		},
	)
}

// DispatchTaskRunLeaseExtended runs the task.run.lease_extended hook dispatch.
func (h *Hooks) DispatchTaskRunLeaseExtended(
	ctx context.Context,
	payload TaskRunLeaseExtendedPayload,
) (TaskRunLeaseExtendedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookTaskRunLeaseExtended,
		payload,
		dispatchConfig[TaskRunLeaseExtendedPayload, TaskRunObservationPatch]{
			match: matchTaskRunLease,
			apply: applyNoop[TaskRunLeaseExtendedPayload, TaskRunObservationPatch],
		},
	)
}

// DispatchTaskRunLeaseExpired runs the task.run.lease_expired hook dispatch.
func (h *Hooks) DispatchTaskRunLeaseExpired(
	ctx context.Context,
	payload TaskRunLeaseExpiredPayload,
) (TaskRunLeaseExpiredPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookTaskRunLeaseExpired,
		payload,
		dispatchConfig[TaskRunLeaseExpiredPayload, TaskRunObservationPatch]{
			match: matchTaskRunLease,
			apply: applyNoop[TaskRunLeaseExpiredPayload, TaskRunObservationPatch],
		},
	)
}

// DispatchTaskRunLeaseRecovered runs the task.run.lease_recovered hook dispatch.
func (h *Hooks) DispatchTaskRunLeaseRecovered(
	ctx context.Context,
	payload TaskRunLeaseRecoveredPayload,
) (TaskRunLeaseRecoveredPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookTaskRunLeaseRecovered,
		payload,
		dispatchConfig[TaskRunLeaseRecoveredPayload, TaskRunObservationPatch]{
			match: matchTaskRunLease,
			apply: applyNoop[TaskRunLeaseRecoveredPayload, TaskRunObservationPatch],
		},
	)
}

// DispatchTaskRunReleased runs the task.run.released hook dispatch.
func (h *Hooks) DispatchTaskRunReleased(
	ctx context.Context,
	payload TaskRunReleasedPayload,
) (TaskRunReleasedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookTaskRunReleased,
		payload,
		dispatchConfig[TaskRunReleasedPayload, TaskRunObservationPatch]{
			match: matchTaskRunLease,
			apply: applyNoop[TaskRunReleasedPayload, TaskRunObservationPatch],
		},
	)
}

// DispatchTaskRunCompleted runs the task.run.completed hook dispatch.
func (h *Hooks) DispatchTaskRunCompleted(
	ctx context.Context,
	payload TaskRunCompletedPayload,
) (TaskRunCompletedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookTaskRunCompleted,
		payload,
		dispatchConfig[TaskRunCompletedPayload, TaskRunObservationPatch]{
			match: matchTaskRunLease,
			apply: applyNoop[TaskRunCompletedPayload, TaskRunObservationPatch],
		},
	)
}

// DispatchTaskRunFailed runs the task.run.failed hook dispatch.
func (h *Hooks) DispatchTaskRunFailed(
	ctx context.Context,
	payload TaskRunFailedPayload,
) (TaskRunFailedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookTaskRunFailed,
		payload,
		dispatchConfig[TaskRunFailedPayload, TaskRunObservationPatch]{
			match: matchTaskRunLease,
			apply: applyNoop[TaskRunFailedPayload, TaskRunObservationPatch],
		},
	)
}
