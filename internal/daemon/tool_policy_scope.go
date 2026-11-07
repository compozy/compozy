package daemon

import (
	"strings"

	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func normalizeToolPolicyScope(scope toolspkg.Scope) toolspkg.Scope {
	return toolspkg.Scope{
		ProfileID:   strings.TrimSpace(scope.ProfileID),
		WorkspaceID: strings.TrimSpace(scope.WorkspaceID),
		SessionID:   strings.TrimSpace(scope.SessionID),
		AgentName:   strings.TrimSpace(scope.AgentName),
		ActorKind:   strings.TrimSpace(scope.ActorKind),
		Operator:    scope.Operator,
	}
}

func fillToolPolicyScopeFromSession(scope toolspkg.Scope, info *session.Info) toolspkg.Scope {
	if info == nil {
		return scope
	}
	if strings.TrimSpace(scope.SessionID) == "" {
		scope.SessionID = strings.TrimSpace(info.ID)
	}
	if strings.TrimSpace(scope.ProfileID) == "" {
		scope.ProfileID = strings.TrimSpace(info.ProfileID)
	}
	if strings.TrimSpace(scope.WorkspaceID) == "" {
		scope.WorkspaceID = strings.TrimSpace(info.WorkspaceID)
	}
	if strings.TrimSpace(scope.AgentName) == "" {
		scope.AgentName = strings.TrimSpace(info.AgentName)
	}
	return scope
}
