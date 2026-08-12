package hooks

import "context"

func (h *Hooks) DispatchWorktreePreCreate(
	ctx context.Context,
	payload WorktreePreCreatePayload,
) (WorktreePreCreatePayload, error) {
	return executeDispatch(ctx, h, HookWorktreePreCreate, payload,
		dispatchConfig[WorktreePreCreatePayload, WorktreeControlPatch]{
			match: matchWorktreePreCreate, apply: applyWorktreePreCreatePatch,
			denied: worktreeControlPatchDenied,
			denyErr: func(_ WorktreePreCreatePayload, report dispatchReport) error {
				return hookDeniedByError(HookWorktreePreCreate, report.DenySource, report.DenyReason)
			},
		})
}

func (h *Hooks) DispatchWorktreePreRemove(
	ctx context.Context,
	payload WorktreePreRemovePayload,
) (WorktreePreRemovePayload, error) {
	return executeDispatch(ctx, h, HookWorktreePreRemove, payload,
		dispatchConfig[WorktreePreRemovePayload, WorktreeControlPatch]{
			match: matchWorktreePreRemove, apply: applyWorktreePreRemovePatch,
			denied: worktreeControlPatchDenied,
			denyErr: func(_ WorktreePreRemovePayload, report dispatchReport) error {
				return hookDeniedByError(HookWorktreePreRemove, report.DenySource, report.DenyReason)
			},
		})
}

func (h *Hooks) DispatchWorktreeCreated(
	ctx context.Context,
	payload WorktreeObservationPayload,
) (WorktreeObservationPayload, error) {
	return executeDispatch(ctx, h, HookWorktreeCreated, payload,
		dispatchConfig[WorktreeObservationPayload, WorktreeObservationPatch]{
			match: matchWorktreeObservation, apply: applyNoop[WorktreeObservationPayload, WorktreeObservationPatch],
		})
}

func (h *Hooks) DispatchWorktreeAdopted(
	ctx context.Context,
	payload WorktreeObservationPayload,
) (WorktreeObservationPayload, error) {
	return executeDispatch(ctx, h, HookWorktreeAdopted, payload,
		dispatchConfig[WorktreeObservationPayload, WorktreeObservationPatch]{
			match: matchWorktreeObservation, apply: applyNoop[WorktreeObservationPayload, WorktreeObservationPatch],
		})
}

func (h *Hooks) DispatchWorktreeRemoved(
	ctx context.Context,
	payload WorktreeObservationPayload,
) (WorktreeObservationPayload, error) {
	return executeDispatch(ctx, h, HookWorktreeRemoved, payload,
		dispatchConfig[WorktreeObservationPayload, WorktreeObservationPatch]{
			match: matchWorktreeObservation, apply: applyNoop[WorktreeObservationPayload, WorktreeObservationPatch],
		})
}

func matchWorktreePreCreate(matcher HookMatcher, payload WorktreePreCreatePayload) bool {
	return matcher.MatchesWorktree(payload.WorktreeContext)
}

func matchWorktreePreRemove(matcher HookMatcher, payload WorktreePreRemovePayload) bool {
	return matcher.MatchesWorktree(payload.Worktree)
}

func matchWorktreeObservation(matcher HookMatcher, payload WorktreeObservationPayload) bool {
	return matcher.MatchesWorktree(payload.WorktreeContext)
}

func applyWorktreePreCreatePatch(
	payload WorktreePreCreatePayload,
	patch WorktreeControlPatch,
) WorktreePreCreatePayload {
	payload.Denied, payload.DenyReason = patch.Deny, patch.DenyReason
	return payload
}

func applyWorktreePreRemovePatch(
	payload WorktreePreRemovePayload,
	patch WorktreeControlPatch,
) WorktreePreRemovePayload {
	payload.Denied, payload.DenyReason = patch.Deny, patch.DenyReason
	return payload
}

func worktreeControlPatchDenied(patch WorktreeControlPatch) bool {
	return patch.Deny
}
