package tools

import (
	"context"

	"fmt"

	"strings"
	"time"
)

type dispatchTarget struct {
	descriptor Descriptor
	handle     Handle
	view       ToolView
}

func (r *RuntimeRegistry) dispatch(ctx context.Context, scope Scope, req CallRequest) (ToolResult, error) {
	started := time.Now().UTC()
	if err := contextErr(ctx, req.ToolID); err != nil {
		return ToolResult{}, err
	}
	req, err := r.normalizeDispatchCallRequest(ctx, scope, req)
	if err != nil {
		return ToolResult{}, err
	}
	if err := req.ToolID.Validate(); err != nil {
		return ToolResult{}, invalidInputError(req.ToolID, "tool id is invalid", err)
	}
	target, err := r.resolveDispatchTarget(ctx, scope, req.ToolID)
	if contextError := contextErr(ctx, req.ToolID); contextError != nil {
		err = contextError
	}
	if err != nil {
		if target.descriptor.ID == "" {
			return ToolResult{}, normalizeToolError(req.ToolID, err)
		}
		return ToolResult{}, r.failDispatch(ctx, &target, req, started, err, ToolCallFailed)
	}
	return r.executeDispatchTarget(ctx, scope, req, &target, started)
}

func (r *RuntimeRegistry) executeDispatchTarget(
	ctx context.Context,
	scope Scope,
	req CallRequest,
	target *dispatchTarget,
	started time.Time,
) (ToolResult, error) {
	req.Input = normalizeCallInput(req.Input)
	if err := r.ensureDispatchTargetCallable(ctx, target, req, started); err != nil {
		return ToolResult{}, err
	}
	req, err := r.bindCallInput(ctx, scope, target.descriptor, req)
	if err != nil {
		if emitErr := r.emit(ctx, target, req, ToolCallStarted, ToolEventData{
			StartedAt: started,
			Input:     req.Input,
		}); emitErr != nil {
			return ToolResult{}, emitErr
		}
		return ToolResult{}, r.failDispatch(ctx, target, req, started, err, callInputFailureEvent(err))
	}
	if err := r.emit(ctx, target, req, ToolCallStarted, ToolEventData{
		StartedAt: started,
		Input:     req.Input,
	}); err != nil {
		return ToolResult{}, err
	}
	if err := validateCallInput(target.descriptor, req.Input); err != nil {
		return ToolResult{}, r.failDispatch(ctx, target, req, started, err, ToolCallFailed)
	}
	patchedReq, err := r.runPreCallHook(ctx, target, req)
	if err != nil {
		return ToolResult{}, r.failDispatch(ctx, target, req, started, err, ToolCallDenied)
	}
	patchedReq, err = r.bindCallInput(ctx, scope, target.descriptor, patchedReq)
	if err != nil {
		return ToolResult{}, r.failDispatch(
			ctx,
			target,
			patchedReq,
			started,
			err,
			callInputFailureEvent(err),
		)
	}
	if err := validateCallInput(target.descriptor, patchedReq.Input); err != nil {
		return ToolResult{}, r.failDispatch(ctx, target, patchedReq, started, err, ToolCallFailed)
	}
	if err := r.authorizeBoundCallInput(ctx, scope, target.descriptor, patchedReq); err != nil {
		return ToolResult{}, r.failDispatch(ctx, target, patchedReq, started, err, ToolCallDenied)
	}
	if err := contextErr(ctx, target.descriptor.ID); err != nil {
		return ToolResult{}, r.failDispatch(ctx, target, patchedReq, started, err, ToolCallFailed)
	}
	if err := r.requestApproval(ctx, scope, target, patchedReq); err != nil {
		return ToolResult{}, r.failDispatch(ctx, target, patchedReq, started, err, ToolCallDenied)
	}
	providerResult, err := target.handle.Call(ctx, patchedReq)
	completionCtx, cancelCompletion := r.postDispatchContext(ctx)
	defer cancelCompletion()
	if err != nil {
		if contextError := contextErr(ctx, target.descriptor.ID); contextError != nil {
			err = contextError
		}
		normalized := normalizeBackendError(target.descriptor.ID, err)
		if hookErr := r.runPostErrorHook(completionCtx, target, patchedReq, normalized); hookErr != nil {
			normalized = hookErr
		}
		return ToolResult{}, r.failDispatch(completionCtx, target, patchedReq, started, normalized, ToolCallFailed)
	}
	return r.completeDispatch(completionCtx, scope, target, patchedReq, started, providerResult)
}

func (r *RuntimeRegistry) postDispatchContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil || ctx.Err() == nil || r.completionTimeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(context.WithoutCancel(ctx), r.completionTimeout)
}

func normalizeCallRequest(scope Scope, req CallRequest) (CallRequest, error) {
	if err := bindCallScopeField(req.ToolID, "profile_id", scope.ProfileID, &req.ProfileID); err != nil {
		return CallRequest{}, err
	}
	if err := bindCallScopeField(req.ToolID, "session_id", scope.SessionID, &req.SessionID); err != nil {
		return CallRequest{}, err
	}
	if err := bindCallScopeField(req.ToolID, "workspace_id", scope.WorkspaceID, &req.WorkspaceID); err != nil {
		return CallRequest{}, err
	}
	if err := bindCallScopeField(req.ToolID, "agent_name", scope.AgentName, &req.AgentName); err != nil {
		return CallRequest{}, err
	}
	if err := bindCallScopeField(req.ToolID, "actor_kind", scope.ActorKind, &req.ActorKind); err != nil {
		return CallRequest{}, err
	}
	return req, nil
}

func bindCallScopeField(id ToolID, field string, trusted string, supplied *string) error {
	if supplied == nil {
		return nil
	}
	trusted = strings.TrimSpace(trusted)
	current := strings.TrimSpace(*supplied)
	if current == "" {
		*supplied = trusted
		return nil
	}
	if trusted != "" && current != trusted {
		return callScopeMismatchError(id, field)
	}
	*supplied = current
	return nil
}

func callScopeMismatchError(id ToolID, field string) error {
	return NewToolError(
		ErrorCodeDenied,
		id,
		fmt.Sprintf("tool %q call %s conflicts with caller scope", id, field),
		ErrToolDenied,
		ReasonScopeMismatch,
	)
}

func validateHookCallRequestIdentity(original CallRequest, patched CallRequest) error {
	checks := []struct {
		field string
		left  string
		right string
	}{
		{field: "tool_call_id", left: original.ToolCallID, right: patched.ToolCallID},
		{field: "turn_id", left: original.TurnID, right: patched.TurnID},
		{field: "profile_id", left: original.ProfileID, right: patched.ProfileID},
		{field: "session_id", left: original.SessionID, right: patched.SessionID},
		{field: "workspace_id", left: original.WorkspaceID, right: patched.WorkspaceID},
		{field: "agent_name", left: original.AgentName, right: patched.AgentName},
		{field: "actor_kind", left: original.ActorKind, right: patched.ActorKind},
		{field: "approval_token", left: original.ApprovalToken, right: patched.ApprovalToken},
	}
	for _, check := range checks {
		if strings.TrimSpace(check.left) != strings.TrimSpace(check.right) {
			return callScopeMismatchError(original.ToolID, check.field)
		}
	}
	return nil
}

func (r *RuntimeRegistry) resolveDispatchTarget(
	ctx context.Context,
	scope Scope,
	id ToolID,
) (dispatchTarget, error) {
	index, err := r.buildIndex(ctx, scope)
	if err != nil {
		return dispatchTarget{}, err
	}
	entry, ok := index.byID[id]
	if !ok {
		return dispatchTarget{}, NewToolError(
			ErrorCodeNotFound,
			id,
			fmt.Sprintf("tool %q not found", id),
			ErrToolNotFound,
			ReasonToolUnknown,
		)
	}
	evaluator, err := r.evaluatorFor(ctx, scope, index.ids())
	if err != nil {
		return dispatchTarget{}, err
	}
	handle, availability := r.resolveHandleForDispatch(ctx, scope, entry)
	decision, err := evaluateIndexedDescriptor(ctx, scope, evaluator, entry.descriptor)
	if err != nil {
		return dispatchTarget{}, err
	}
	decision = applyAvailabilityDecision(decision, availability)
	view := ToolView{
		Descriptor:   cloneDescriptor(entry.descriptor),
		Availability: availability,
		Decision:     decision,
	}
	target := dispatchTarget{descriptor: cloneDescriptor(entry.descriptor), handle: handle, view: view}
	if !decision.Callable || isNilInterface(handle) {
		return target, nil
	}
	if err := ensureHandleMatchesDescriptor(entry.descriptor, handle); err != nil {
		return target, err
	}
	return target, nil
}

func (r *RuntimeRegistry) ensureDispatchTargetCallable(
	ctx context.Context,
	target *dispatchTarget,
	req CallRequest,
	started time.Time,
) error {
	if !target.view.Decision.Callable {
		return r.failDispatch(ctx, target, req, started, denialErrorForView(&target.view), ToolCallDenied)
	}
	if isNilInterface(target.handle) {
		return r.failDispatch(ctx, target, req, started, NewToolError(
			ErrorCodeUnavailable,
			target.descriptor.ID,
			fmt.Sprintf("tool %q is unavailable", target.descriptor.ID),
			ErrToolUnavailable,
			ReasonBackendNotExecutable,
		), ToolCallDenied)
	}
	return nil
}

func (r *RuntimeRegistry) resolveHandleForDispatch(
	ctx context.Context,
	scope Scope,
	entry *registryEntry,
) (Handle, Availability) {
	if len(entry.conflicts) > 0 {
		return nil, Availability{
			Registered:  true,
			Enabled:     true,
			Conflicted:  true,
			ReasonCodes: append([]ReasonCode(nil), entry.conflicts...),
		}
	}
	handle, ok, err := entry.provider.Resolve(ctx, scope, entry.descriptor.ID)
	if err != nil {
		return nil, Availability{
			Registered:  true,
			Enabled:     true,
			ReasonCodes: []ReasonCode{ReasonBackendUnhealthy},
		}
	}
	if !ok || isNilInterface(handle) {
		return nil, Availability{
			Registered:  true,
			Enabled:     true,
			ReasonCodes: []ReasonCode{ReasonBackendNotExecutable},
		}
	}
	availability := handle.Availability(ctx, scope)
	availability.Registered = true
	if err := availability.Validate(); err != nil {
		reason, found := ReasonOf(err)
		if !found {
			reason = ReasonBackendUnhealthy
		}
		return handle, Availability{
			Registered:  true,
			Enabled:     availability.Enabled,
			Available:   availability.Available,
			Authorized:  availability.Authorized,
			Executable:  false,
			Conflicted:  availability.Conflicted,
			ReasonCodes: appendReason(availability.ReasonCodes, reason),
		}
	}
	return handle, availability
}

func ensureHandleMatchesDescriptor(descriptor Descriptor, handle Handle) error {
	if err := ValidateHandle(handle); err != nil {
		return NewToolError(
			ErrorCodeUnavailable,
			descriptor.ID,
			fmt.Sprintf("tool %q handle rejected", descriptor.ID),
			fmt.Errorf("%w: %w", ErrToolUnavailable, err),
			ReasonRuntimeDescriptorMismatch,
		)
	}
	handleDescriptor := handle.Descriptor()
	if handleDescriptor.ID != descriptor.ID || handleDescriptor.Backend.Kind != descriptor.Backend.Kind {
		return NewToolError(
			ErrorCodeUnavailable,
			descriptor.ID,
			fmt.Sprintf("tool %q handle descriptor mismatch", descriptor.ID),
			ErrToolUnavailable,
			ReasonRuntimeDescriptorMismatch,
		)
	}
	return nil
}
