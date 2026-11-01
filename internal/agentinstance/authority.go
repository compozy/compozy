package agentinstance

import (
	"context"
	"slices"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

type Caller struct {
	SessionID, AgentName, ProfileID, InstanceID string
	InstanceDepth                               int
	PermissionPolicy                            store.SessionPermissionPolicy
	Grant                                       string
}

type CallerReader interface {
	InstanceCaller(context.Context, string) (Caller, error)
}

type Authority struct{ Callers CallerReader }

func (a Authority) Authorize(ctx context.Context, actor workspaceaccess.ActorRef, instance Instance, verb string) error {
	if actor.Operator {
		return nil
	}
	if actor.Kind != workspaceaccess.ActorAgentSession || actor.SessionID == "" || a.Callers == nil {
		return ErrForbidden
	}
	caller, err := a.Callers.InstanceCaller(ctx, actor.SessionID)
	if err != nil {
		return err
	}
	if caller.InstanceID != instance.ID || caller.AgentName != instance.AgentName || caller.ProfileID != instance.ProfileID {
		return ErrForbidden
	}
	switch verb {
	case "list", "reports", "pause", "archive", "ensure_main", "check", "checks_off":
		return nil
	case "resume":
		if instance.PauseReason != "manual" && instance.PauseReason != "fatigue" {
			return nil
		}
	}
	return ErrForbidden
}

func IntersectPermissions(a, b store.SessionPermissionPolicy) store.SessionPermissionPolicy {
	a = store.NormalizeSessionPermissionPolicy(a)
	b = store.NormalizeSessionPermissionPolicy(b)
	return store.SessionPermissionPolicy{Tools: intersectAtoms(a.Tools, b.Tools), Skills: intersectAtoms(a.Skills, b.Skills), MCPServers: intersectAtoms(a.MCPServers, b.MCPServers), WorkspacePaths: intersectAtoms(a.WorkspacePaths, b.WorkspacePaths)}
}
func intersectAtoms(a, b []string) []string {
	result := make([]string, 0)
	for _, atom := range a {
		if slices.Contains(b, atom) {
			result = append(result, atom)
		}
	}
	return result
}
