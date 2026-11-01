package daemon

import (
	"context"
	"github.com/compozy/compozy/internal/acp"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"time"
)

func instancePermissionWaitResolver(state *bootState) acp.PermissionWaitResolver {
	provider, ok := state.registry.(instanceRepositoryProvider)
	if !ok {
		return nil
	}
	repo := provider.Instances()
	wait := state.cfg.Agents.StaysOn.ApprovalTimeout
	return func(sessionID string) time.Duration {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		caller, err := repo.InstanceCaller(ctx, sessionID)
		if err != nil || caller.InstanceID == "" {
			return 0
		}
		instance, err := repo.Get(ctx, caller.InstanceID)
		if err != nil || instance.ArchivedAt != nil || instance.State == "archived" {
			return 0
		}
		return wait
	}
}

func (b *toolApprovalBridge) sessionRequestTimeout(sessionID string, toolID toolspkg.ToolID) time.Duration {
	if b.permissionWaitResolver != nil {
		if wait := b.permissionWaitResolver(sessionID); wait > 0 {
			return wait
		}
	}
	return b.requestTimeout(toolID)
}
