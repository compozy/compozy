package tools

import (
	"context"

	"errors"
	"fmt"
)

func (r *RuntimeRegistry) runPreCallHook(
	ctx context.Context,
	target *dispatchTarget,
	req CallRequest,
) (CallRequest, error) {
	if r.hooks == nil {
		return req, nil
	}
	patched, decision, err := r.hooks.PreCall(ctx, req)
	if err != nil {
		return req, normalizeHookError(target.descriptor.ID, err)
	}
	patched = mergeHookCallRequest(req, patched)
	if patched.ToolID != req.ToolID {
		return req, NewToolError(
			ErrorCodeDenied,
			req.ToolID,
			fmt.Sprintf("tool %q pre-call hook attempted to change tool_id", req.ToolID),
			ErrToolDenied,
			ReasonHookDenied,
		)
	}
	if !decision.Callable {
		reasons := appendUniqueReasons(append([]ReasonCode(nil), decision.ReasonCodes...), ReasonHookDenied)
		return req, NewToolError(
			ErrorCodeDenied,
			req.ToolID,
			fmt.Sprintf("tool %q denied by pre-call hook", req.ToolID),
			ErrToolDenied,
			reasons...,
		)
	}
	if err := validateCallInput(target.descriptor, normalizeCallInput(patched.Input)); err != nil {
		return req, err
	}
	if err := validateHookCallRequestIdentity(req, patched); err != nil {
		return req, err
	}
	patched.Input = normalizeCallInput(patched.Input)
	return patched, nil
}

func (r *RuntimeRegistry) requestApproval(
	ctx context.Context,
	scope Scope,
	target *dispatchTarget,
	req CallRequest,
) error {
	if target == nil || !target.view.Decision.ApprovalRequired {
		return nil
	}
	if r.approvalBridge == nil {
		return approvalBridgeError(
			target.descriptor.ID,
			"tool approval channel is unavailable",
			ErrToolApprovalRequired,
			ReasonApprovalUnreachable,
		)
	}
	view := cloneToolView(&target.view)
	if err := r.approvalBridge.RequestToolApproval(ctx, scope, req, &view); err != nil {
		return normalizeApprovalBridgeError(target.descriptor.ID, err)
	}
	return nil
}

func normalizeApprovalBridgeError(id ToolID, err error) error {
	if err == nil {
		return nil
	}
	if toolErr, ok := errors.AsType[*ToolError](err); ok {
		return normalizeToolError(id, toolErr)
	}
	switch {
	case errors.Is(err, context.Canceled):
		return approvalBridgeError(id, "tool approval was canceled", ErrToolApprovalRequired, ReasonApprovalCanceled)
	case errors.Is(err, context.DeadlineExceeded):
		return approvalBridgeError(id, "tool approval timed out", ErrToolApprovalRequired, ReasonApprovalTimedOut)
	case errors.Is(err, ErrToolApprovalRequired):
		return approvalBridgeError(id, "tool approval was not granted", err)
	default:
		return approvalBridgeError(id, "tool approval failed", fmt.Errorf("%w: %w", ErrToolApprovalRequired, err))
	}
}

func approvalBridgeError(id ToolID, message string, err error, reasons ...ReasonCode) *ToolError {
	allReasons := appendUniqueReasons([]ReasonCode{ReasonApprovalRequired}, reasons...)
	return NewToolError(
		ErrorCodeApprovalRequired,
		id,
		fmt.Sprintf("%s for %q", message, id),
		err,
		allReasons...,
	)
}

func mergeHookCallRequest(original CallRequest, patched CallRequest) CallRequest {
	if patched.ToolID == "" {
		patched.ToolID = original.ToolID
	}
	if patched.ToolCallID == "" {
		patched.ToolCallID = original.ToolCallID
	}
	if patched.TurnID == "" {
		patched.TurnID = original.TurnID
	}
	if patched.SessionID == "" {
		patched.SessionID = original.SessionID
	}
	if patched.WorkspaceID == "" {
		patched.WorkspaceID = original.WorkspaceID
	}
	if patched.AgentName == "" {
		patched.AgentName = original.AgentName
	}
	if patched.ActorKind == "" {
		patched.ActorKind = original.ActorKind
	}
	if patched.CorrelationID == "" {
		patched.CorrelationID = original.CorrelationID
	}
	if len(patched.Input) == 0 {
		patched.Input = original.Input
	}
	if len(patched.SensitiveInputFields) == 0 {
		patched.SensitiveInputFields = append([]string(nil), original.SensitiveInputFields...)
	}
	if patched.ApprovalToken == "" {
		patched.ApprovalToken = original.ApprovalToken
	}
	return patched
}
