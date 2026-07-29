package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/workspaceaccess"
)

func (r *RuntimeRegistry) normalizeDispatchCallRequest(
	ctx context.Context,
	scope Scope,
	req CallRequest,
) (CallRequest, error) {
	for _, binding := range []struct {
		field    string
		trusted  string
		supplied *string
	}{
		{field: "session_id", trusted: scope.SessionID, supplied: &req.SessionID},
		{field: "agent_name", trusted: scope.AgentName, supplied: &req.AgentName},
		{field: "actor_kind", trusted: scope.ActorKind, supplied: &req.ActorKind},
	} {
		if err := bindCallScopeField(req.ToolID, binding.field, binding.trusted, binding.supplied); err != nil {
			return CallRequest{}, err
		}
	}
	trustedWorkspaceID := strings.TrimSpace(scope.WorkspaceID)
	requestedWorkspaceID := strings.TrimSpace(req.WorkspaceID)
	if requestedWorkspaceID == "" {
		req.WorkspaceID = trustedWorkspaceID
		return req, nil
	}
	if trustedWorkspaceID == "" || trustedWorkspaceID == requestedWorkspaceID {
		req.WorkspaceID = requestedWorkspaceID
		return req, nil
	}
	if r == nil || r.workspaceAccess == nil {
		return CallRequest{}, workspaceAccessDeniedError(req.ToolID)
	}
	decision, err := r.workspaceAccess.Authorize(ctx, workspaceaccess.Request{
		Actor:             workspaceAccessActorFromScope(scope),
		TargetWorkspaceID: requestedWorkspaceID,
		Seam:              workspaceaccess.SeamTool,
	})
	if err != nil || !decision.Allowed {
		return CallRequest{}, workspaceAccessDeniedError(req.ToolID)
	}
	req.WorkspaceID = requestedWorkspaceID
	return req, nil
}

func workspaceAccessActorFromScope(scope Scope) workspaceaccess.ActorRef {
	kind := workspaceaccess.ActorKind(strings.TrimSpace(scope.ActorKind))
	if kind == "" && strings.TrimSpace(scope.SessionID) != "" {
		kind = workspaceaccess.ActorAgentSession
	}
	if scope.Operator && kind == "" {
		kind = workspaceaccess.ActorHuman
	}
	return workspaceaccess.ActorRef{
		Kind:        kind,
		SessionID:   strings.TrimSpace(scope.SessionID),
		WorkspaceID: strings.TrimSpace(scope.WorkspaceID),
		AgentName:   strings.TrimSpace(scope.AgentName),
		Operator:    scope.Operator,
	}
}

func workspaceAccessDeniedError(id ToolID) error {
	return NewToolError(
		ErrorCodeDenied,
		id,
		fmt.Sprintf("tool %q denied: %s", id, workspaceaccess.DenialHint),
		ErrToolDenied,
		ReasonWorkspaceAccessDenied,
	)
}
