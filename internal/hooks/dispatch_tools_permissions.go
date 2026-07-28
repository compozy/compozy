package hooks

import "context"

// DispatchToolPreCall runs the tool.pre_call hook pipeline.
func (h *Hooks) DispatchToolPreCall(ctx context.Context, payload ToolPreCallPayload) (ToolPreCallPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookToolPreCall,
		payload,
		dispatchConfig[ToolPreCallPayload, ToolCallPatch]{
			match:  matchToolPreCall,
			apply:  applyToolCallPatch,
			denied: toolCallPatchDenied,
			denyErr: func(_ ToolPreCallPayload, report dispatchReport) error {
				return hookDeniedError(HookToolPreCall, report.DenyReason)
			},
		},
	)
}

// DispatchToolPostCall runs the tool.post_call hook pipeline.
func (h *Hooks) DispatchToolPostCall(ctx context.Context, payload ToolPostCallPayload) (ToolPostCallPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookToolPostCall,
		payload,
		dispatchConfig[ToolPostCallPayload, ToolResultPatch]{
			match:  matchToolPostCall,
			apply:  applyToolResultPatch,
			denied: toolResultPatchDenied,
		},
	)
}

// DispatchToolPostError runs the tool.post_error hook pipeline.
func (h *Hooks) DispatchToolPostError(ctx context.Context, payload ToolPostErrorPayload) (ToolPostErrorPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookToolPostError,
		payload,
		dispatchConfig[ToolPostErrorPayload, ToolPostErrorPatch]{
			match:  matchToolPostError,
			apply:  applyToolPostErrorPatch,
			denied: toolResultPatchDenied,
		},
	)
}

// DispatchPermissionRequest runs the permission.request hook pipeline.
func (h *Hooks) DispatchPermissionRequest(
	ctx context.Context,
	payload PermissionRequestPayload,
) (PermissionRequestPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookPermissionRequest,
		payload,
		dispatchConfig[PermissionRequestPayload, PermissionRequestPatch]{
			match:  matchPermissionRequest,
			apply:  mergePermissionRequestPatch,
			denied: permissionPatchDenies,
			guard:  newPermissionRequestGuard(h.logger, h.metrics),
		},
	)
}

// DispatchPermissionResolved runs the permission.resolved hook dispatch.
func (h *Hooks) DispatchPermissionResolved(
	ctx context.Context,
	payload PermissionResolvedPayload,
) (PermissionResolvedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookPermissionResolved,
		payload,
		dispatchConfig[PermissionResolvedPayload, PermissionResolvedPatch]{
			match: matchPermissionResolution,
			apply: applyNoop[PermissionResolvedPayload, PermissionResolvedPatch],
		},
	)
}

// DispatchPermissionDenied runs the permission.denied hook dispatch.
func (h *Hooks) DispatchPermissionDenied(
	ctx context.Context,
	payload PermissionDeniedPayload,
) (PermissionDeniedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookPermissionDenied,
		payload,
		dispatchConfig[PermissionDeniedPayload, PermissionDeniedPatch]{
			match: matchPermissionResolution,
			apply: applyNoop[PermissionDeniedPayload, PermissionDeniedPatch],
		},
	)
}
