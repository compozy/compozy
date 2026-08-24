package daemon

import (
	bridgepkg "github.com/compozy/compozy/internal/bridges"
	"github.com/compozy/compozy/internal/store"
)

func (r *bridgeRuntime) forgetRemovedBridgeEntities(plan *bridgepkg.ResourceProjectionPlan) {
	if r == nil || r.deadEntities == nil || plan == nil {
		return
	}
	nextIDs := make(map[string]struct{})
	for _, instance := range plan.NextInstances() {
		nextIDs[instance.ID] = struct{}{}
	}
	for _, instance := range plan.PreviousInstances() {
		if instance.Scope != bridgepkg.ScopeWorkspace {
			continue
		}
		if _, retained := nextIDs[instance.ID]; retained {
			continue
		}
		r.deadEntities.ForgetEntity(store.DeadEntityKey{
			ProfileID:   instance.ProfileID,
			WorkspaceID: instance.WorkspaceID,
			Kind:        store.DeadEntityKindBridge,
			EntityID:    instance.ID,
		})
	}
}
