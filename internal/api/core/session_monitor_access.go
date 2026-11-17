package core

import (
	"context"
	"slices"

	monitorpkg "github.com/compozy/compozy/internal/monitor"
	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

type monitorSessionReader interface {
	Status(context.Context, string) (*session.Info, error)
}

// VisibleMonitorTargets rechecks access before retained states leave native, HTTP or wake boundaries.
func VisibleMonitorTargets(
	ctx context.Context, sessions monitorSessionReader, policy workspaceaccess.Policy,
	grants toolspkg.ApprovalGrantStore, viewer *session.Info, mon *monitorpkg.Monitor, seam workspaceaccess.Seam,
) ([]monitorpkg.TargetStatus, bool) {
	visible := slices.Clone(mon.Targets)
	denied := false
	for i, target := range visible {
		ref := monitorTargetScope(ctx, sessions, mon, target.SessionID)
		if ref.Kind == workspaceaccess.TargetWorkspace &&
			ref.WorkspaceID == viewer.WorkspaceID && ref.ProfileID == viewer.ProfileID {
			continue
		}
		actor := workspaceaccess.ActorRef{
			Kind: workspaceaccess.ActorAgentSession, SessionID: viewer.ID,
			WorkspaceID: viewer.WorkspaceID, AgentName: viewer.AgentName,
		}
		allowed := false
		if policy != nil {
			decision, err := policy.Authorize(ctx, workspaceaccess.Request{
				Actor: actor, Target: ref, ReadOnly: true, Seam: seam,
			})
			if err == nil {
				allowed = decision.Allowed
				if !allowed && decision.PromptEligible {
					// Allow once belongs only to this dispatch; later reads need a current saved grant.
					allowed = toolspkg.SessionTargetAuthorized(ctx, target.SessionID) ||
						monitorReadGranted(ctx, grants, viewer, actor, ref)
				}
			}
		}
		if !allowed {
			visible[i] = monitorpkg.TargetStatus{SessionID: target.SessionID, State: monitorpkg.TargetStateUnknown}
			denied = true
			continue
		}
		visible[i].Name = redact.CrossScopeString(target.Name)
	}
	return visible, denied
}

func monitorTargetScope(
	ctx context.Context, sessions monitorSessionReader, mon *monitorpkg.Monitor, sessionID string,
) workspaceaccess.TargetRef {
	if info, err := sessions.Status(ctx, sessionID); err == nil && info != nil {
		return workspaceaccess.ScopeTarget(info.WorkspaceID, info.ProfileID)
	}
	for _, scope := range mon.TargetScopes {
		if scope.SessionID == sessionID {
			return workspaceaccess.ScopeTarget(scope.WorkspaceID, scope.ProfileID)
		}
	}
	return workspaceaccess.TargetRef{Kind: workspaceaccess.TargetUnresolved}
}

func monitorReadGranted(
	ctx context.Context, grants toolspkg.ApprovalGrantStore, viewer *session.Info,
	actor workspaceaccess.ActorRef, target workspaceaccess.TargetRef,
) bool {
	if grants == nil {
		return false
	}
	key := toolspkg.WorkspaceAccessReadGrantKey(
		toolspkg.Scope{ProfileID: viewer.ProfileID, WorkspaceID: viewer.WorkspaceID, AgentName: viewer.AgentName},
		toolspkg.CallRequest{}, toolspkg.Descriptor{ID: toolspkg.ToolIDMonitorCreate, ReadOnly: true}, actor, target,
	)
	if key == nil {
		return false
	}
	grant, found, err := grants.LookupApprovalGrant(ctx, *key)
	return err == nil && found && grant.ApprovalGrantKey == *key && grant.Decision == toolspkg.ApprovalGrantAllow
}
