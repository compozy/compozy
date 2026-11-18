package daemon

import (
	"context"

	"github.com/compozy/compozy/internal/api/core"
	monitorpkg "github.com/compozy/compozy/internal/monitor"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

func (n *daemonNativeTools) visibleMonitor(
	ctx context.Context,
	ownerID string,
	mon monitorpkg.Monitor,
) (monitorpkg.Monitor, error) {
	if mon.Kind != monitorpkg.KindSession || len(mon.Targets) == 0 {
		return mon, nil
	}
	owner, err := n.deps.Sessions.Status(ctx, ownerID)
	if err != nil {
		return monitorpkg.Monitor{}, err
	}
	if owner == nil {
		return monitorpkg.Monitor{}, session.ErrSessionNotFound
	}
	var denied bool
	mon.Targets, denied = core.VisibleMonitorTargets(
		ctx, n.deps.Sessions, n.deps.WorkspaceAccess, n.deps.ApprovalGrants, owner, &mon, workspaceaccess.SeamTool,
	)
	if denied {
		mon.Reason = "access_denied"
	}
	return mon, nil
}

func (w *monitorSessionWaker) accessPolicy() workspaceaccess.Policy {
	if w.workspaceAccess == nil {
		return nil
	}
	return w.workspaceAccess()
}

func (w *monitorSessionWaker) grantStore() toolspkg.ApprovalGrantStore {
	if w.approvalGrants == nil {
		return nil
	}
	return w.approvalGrants()
}
