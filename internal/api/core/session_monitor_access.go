package core

import (
	"context"
	"slices"

	monitorpkg "github.com/compozy/compozy/internal/monitor"
	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

type monitorSessionReader interface {
	Status(context.Context, string) (*session.Info, error)
}

// VisibleMonitorTargets rechecks access before retained states leave native, HTTP or wake boundaries.
func VisibleMonitorTargets(
	ctx context.Context, sessions monitorSessionReader, policy workspaceaccess.Policy,
	viewer *session.Info, targets []monitorpkg.TargetStatus, seam workspaceaccess.Seam,
) ([]monitorpkg.TargetStatus, bool) {
	visible := slices.Clone(targets)
	denied := false
	for i, target := range visible {
		info, err := sessions.Status(ctx, target.SessionID)
		if err == nil && info != nil && info.WorkspaceID == viewer.WorkspaceID && info.ProfileID == viewer.ProfileID {
			continue
		}
		allowed := false
		if policy != nil {
			ref := workspaceaccess.TargetRef{Kind: workspaceaccess.TargetUnresolved}
			if err == nil && info != nil {
				ref = workspaceaccess.ScopeTarget(info.WorkspaceID, info.ProfileID)
			}
			decision, accessErr := policy.Authorize(ctx, workspaceaccess.Request{
				Actor: workspaceaccess.ActorRef{
					Kind: workspaceaccess.ActorAgentSession, SessionID: viewer.ID,
					WorkspaceID: viewer.WorkspaceID, AgentName: viewer.AgentName,
				},
				Target: ref, ReadOnly: true, Seam: seam,
			})
			allowed = accessErr == nil && decision.Allowed
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
