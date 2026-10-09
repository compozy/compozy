package daemon

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) subagentService() session.SubagentService {
	if n == nil || n.deps == nil || n.deps.Subagents == nil {
		return nil
	}
	return n.deps.Subagents()
}

func (n *daemonNativeTools) subagentToolBindings() map[toolspkg.ToolID]nativeToolBinding {
	available := n.dependencyAvailability(func() bool {
		return n.subagentService() != nil && n.deps.Sessions != nil && n.deps.Workspaces != nil
	})
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDSubagentCapabilities: {call: n.subagentCapabilities, availability: available},
		toolspkg.ToolIDSubagentDelegate:     {call: n.subagentDelegate, availability: available},
		toolspkg.ToolIDSubagentStatus:       {call: n.subagentStatus, availability: available},
		toolspkg.ToolIDSubagentCancel:       {call: n.subagentCancel, availability: available},
	}
}

func (n *daemonNativeTools) subagentCaller(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (session.SubagentCaller, error) {
	if scope.Operator || strings.TrimSpace(scope.SessionID) == "" || strings.TrimSpace(req.TurnID) == "" {
		return session.SubagentCaller{}, session.ErrSubagentParentNotActive
	}
	id, parent, err := n.nativeOrchestrationTarget(ctx, scope, req.ToolID, scope.SessionID, false)
	if err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			return session.SubagentCaller{}, session.ErrSubagentParentNotActive
		}
		return session.SubagentCaller{}, err
	}
	if parent.State == session.StateStopping || parent.State == session.StateStopped {
		return session.SubagentCaller{}, session.ErrSubagentParentNotActive
	}
	if _, err := n.deps.Sessions.ActivePromptRun(ctx, id); err != nil {
		if errors.Is(err, session.ErrPromptNotActive) {
			return session.SubagentCaller{}, session.ErrSubagentParentNotActive
		}
		return session.SubagentCaller{}, err
	}
	return session.SubagentCaller{
		WorkspaceID: parent.WorkspaceID,
		SessionID:   id,
		TurnID: strings.TrimSpace(
			req.TurnID,
		),
		ToolCallID: strings.TrimSpace(req.ToolCallID),
		AgentName:  parent.AgentName,
	}, nil
}

func (n *daemonNativeTools) subagentCapabilities(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input struct{}
	if err := decodeNativeInput(req, &input); err != nil {
		return subagentInputFailure(err)
	}
	caller, err := n.subagentCaller(ctx, scope, req)
	if err != nil {
		return subagentFailure(err, "")
	}
	svc := n.subagentService()
	if svc == nil {
		return toolspkg.ToolResult{}, nativeUnavailableError(req.ToolID, "subagent service is unavailable")
	}
	result, err := svc.Capabilities(ctx, caller)
	if err != nil {
		return subagentFailure(err, "")
	}
	return structuredResult(subagentCapabilitiesPayload(result), "Subagent capabilities")
}

func (n *daemonNativeTools) subagentDelegate(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeSubagentDelegateInput
	if err := decodeNativeInput(req, &input); err != nil {
		return subagentInputFailure(err)
	}
	caller, err := n.subagentCaller(ctx, scope, req)
	if err != nil {
		return subagentFailure(err, "")
	}
	request, err := input.request(caller)
	if err != nil {
		return subagentFailure(err, "")
	}
	svc := n.subagentService()
	if svc == nil {
		return toolspkg.ToolResult{}, nativeUnavailableError(req.ToolID, "subagent service is unavailable")
	}
	if err := n.validateSubagentPermissions(ctx, request); err != nil {
		return subagentFailure(err, "")
	}
	result, err := svc.Delegate(ctx, request)
	if err != nil {
		return subagentFailure(err, "")
	}
	return structuredResult(subagentPayload(&result), result.ID)
}

func (n *daemonNativeTools) subagentStatus(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeSubagentIDInput
	if err := decodeNativeInput(req, &input); err != nil {
		return subagentInputFailure(err)
	}
	caller, err := n.subagentCaller(ctx, scope, req)
	if err != nil {
		return subagentFailure(err, input.SubagentID)
	}
	if strings.TrimSpace(input.SubagentID) == "" {
		return subagentInvalidResult("subagent_id is required.")
	}
	svc := n.subagentService()
	if svc == nil {
		return toolspkg.ToolResult{}, nativeUnavailableError(req.ToolID, "subagent service is unavailable")
	}
	result, err := svc.Status(ctx, caller, input.SubagentID)
	if err != nil {
		return subagentFailure(err, input.SubagentID)
	}
	return structuredResult(subagentPayload(&result), result.ID)
}

func (n *daemonNativeTools) subagentCancel(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeSubagentCancelInput
	if err := decodeNativeInput(req, &input); err != nil {
		return subagentInputFailure(err)
	}
	caller, err := n.subagentCaller(ctx, scope, req)
	if err != nil {
		return subagentFailure(err, input.SubagentID)
	}
	if strings.TrimSpace(input.SubagentID) == "" {
		return subagentInvalidResult("subagent_id is required.")
	}
	svc := n.subagentService()
	if svc == nil {
		return toolspkg.ToolResult{}, nativeUnavailableError(req.ToolID, "subagent service is unavailable")
	}
	result, err := svc.Cancel(
		ctx,
		session.SubagentActor{Kind: daemonAgentField, ID: caller.SessionID, Caller: &caller},
		input.SubagentID,
		input.Reason,
	)
	if err != nil {
		return subagentFailure(err, input.SubagentID)
	}
	return structuredResult(map[string]any{"subagent_id": result.ID, "status": result.Status}, result.Status)
}
