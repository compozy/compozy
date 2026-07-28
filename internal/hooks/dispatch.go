package hooks

import "context"

type dispatchConfig[P any, R any] struct {
	match   matcherFunc[P]
	apply   func(P, R) P
	denied  denyDetector[R]
	denyErr func(P, dispatchReport) error
	guard   patchGuard[P, R]
}

// DispatchSessionPreCreate runs the session.pre_create hook pipeline.
func (h *Hooks) DispatchSessionPreCreate(
	ctx context.Context,
	payload SessionPreCreatePayload,
) (SessionPreCreatePayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookSessionPreCreate,
		payload,
		dispatchConfig[SessionPreCreatePayload, SessionCreatePatch]{
			match:  matchSessionPreCreate,
			apply:  applySessionCreatePatch,
			denied: sessionCreatePatchDenied,
			denyErr: func(_ SessionPreCreatePayload, report dispatchReport) error {
				return hookDeniedError(HookSessionPreCreate, report.DenyReason)
			},
		},
	)
}

// DispatchSessionPostCreate runs the session.post_create hook pipeline.
func (h *Hooks) DispatchSessionPostCreate(
	ctx context.Context,
	payload SessionPostCreatePayload,
) (SessionPostCreatePayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookSessionPostCreate,
		payload,
		dispatchConfig[SessionPostCreatePayload, SessionPostCreatePatch]{
			match:  matchSessionLifecycle,
			apply:  applySessionLifecyclePatch,
			denied: sessionCreatePatchDenied,
		},
	)
}

// DispatchSessionPreResume runs the session.pre_resume hook pipeline.
func (h *Hooks) DispatchSessionPreResume(
	ctx context.Context,
	payload SessionPreResumePayload,
) (SessionPreResumePayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookSessionPreResume,
		payload,
		dispatchConfig[SessionPreResumePayload, SessionPreResumePatch]{
			match:  matchSessionLifecycle,
			apply:  applySessionLifecyclePatch,
			denied: sessionCreatePatchDenied,
			denyErr: func(_ SessionPreResumePayload, report dispatchReport) error {
				return hookDeniedError(HookSessionPreResume, report.DenyReason)
			},
		},
	)
}

// DispatchSessionPostResume runs the session.post_resume hook pipeline.
func (h *Hooks) DispatchSessionPostResume(
	ctx context.Context,
	payload SessionPostResumePayload,
) (SessionPostResumePayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookSessionPostResume,
		payload,
		dispatchConfig[SessionPostResumePayload, SessionPostResumePatch]{
			match:  matchSessionLifecycle,
			apply:  applySessionLifecyclePatch,
			denied: sessionCreatePatchDenied,
		},
	)
}

// DispatchSessionPreStop runs the session.pre_stop hook pipeline.
func (h *Hooks) DispatchSessionPreStop(
	ctx context.Context,
	payload SessionPreStopPayload,
) (SessionPreStopPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookSessionPreStop,
		payload,
		dispatchConfig[SessionPreStopPayload, SessionPreStopPatch]{
			match:  matchSessionLifecycle,
			apply:  applySessionLifecyclePatch,
			denied: sessionCreatePatchDenied,
			denyErr: func(_ SessionPreStopPayload, report dispatchReport) error {
				return hookDeniedError(HookSessionPreStop, report.DenyReason)
			},
		},
	)
}

// DispatchSessionPostStop runs the session.post_stop hook pipeline.
func (h *Hooks) DispatchSessionPostStop(
	ctx context.Context,
	payload SessionPostStopPayload,
) (SessionPostStopPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookSessionPostStop,
		payload,
		dispatchConfig[SessionPostStopPayload, SessionPostStopPatch]{
			match:  matchSessionLifecycle,
			apply:  applySessionLifecyclePatch,
			denied: sessionCreatePatchDenied,
		},
	)
}

// DispatchSandboxPrepare runs the sandbox.prepare hook pipeline.
func (h *Hooks) DispatchSandboxPrepare(
	ctx context.Context,
	payload SandboxPreparePayload,
) (SandboxPreparePayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookSandboxPrepare,
		payload,
		dispatchConfig[SandboxPreparePayload, SandboxPreparePatch]{
			match:  matchSandboxPrepare,
			apply:  applySandboxPreparePatch,
			denied: sandboxPreparePatchDenied,
			denyErr: func(_ SandboxPreparePayload, report dispatchReport) error {
				return hookDeniedError(HookSandboxPrepare, report.DenyReason)
			},
		},
	)
}

// DispatchSandboxReady runs the sandbox.ready hook dispatch.
func (h *Hooks) DispatchSandboxReady(
	ctx context.Context,
	payload SandboxReadyPayload,
) (SandboxReadyPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookSandboxReady,
		payload,
		dispatchConfig[SandboxReadyPayload, SandboxReadyPatch]{
			match: matchSandboxReady,
			apply: applyNoop[SandboxReadyPayload, SandboxReadyPatch],
		},
	)
}

// DispatchSandboxSyncBefore runs the sandbox.sync.before hook pipeline.
func (h *Hooks) DispatchSandboxSyncBefore(
	ctx context.Context,
	payload SandboxSyncBeforePayload,
) (SandboxSyncBeforePayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookSandboxSyncBefore,
		payload,
		dispatchConfig[SandboxSyncBeforePayload, SandboxSyncBeforePatch]{
			match:  matchSandboxSyncBefore,
			apply:  applySandboxSyncBeforePatch,
			denied: sandboxSyncBeforePatchDenied,
		},
	)
}

// DispatchSandboxSyncAfter runs the sandbox.sync.after hook dispatch.
func (h *Hooks) DispatchSandboxSyncAfter(
	ctx context.Context,
	payload SandboxSyncAfterPayload,
) (SandboxSyncAfterPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookSandboxSyncAfter,
		payload,
		dispatchConfig[SandboxSyncAfterPayload, SandboxSyncAfterPatch]{
			match: matchSandboxSyncAfter,
			apply: applyNoop[SandboxSyncAfterPayload, SandboxSyncAfterPatch],
		},
	)
}

// DispatchSandboxStop runs the sandbox.stop hook pipeline.
func (h *Hooks) DispatchSandboxStop(
	ctx context.Context,
	payload SandboxStopPayload,
) (SandboxStopPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookSandboxStop,
		payload,
		dispatchConfig[SandboxStopPayload, SandboxStopPatch]{
			match:  matchSandboxStop,
			apply:  applySandboxStopPatch,
			denied: sandboxStopPatchDenied,
		},
	)
}
