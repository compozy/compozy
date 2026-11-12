package daemon

import (
	"context"

	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/modelcatalog"
)

func bindCheckConfigRecovery(state *bootState, checks *heartbeat.CheckService) {
	catalog := state.modelCatalog
	if catalog == nil {
		return
	}
	catalog.afterRefreshMu.Lock()
	defer catalog.afterRefreshMu.Unlock()
	catalog.afterRefresh = func(ctx context.Context, opts modelcatalog.RefreshOptions) {
		// The refresh already holds the generation gate; read its service directly.
		models, err := catalog.service.ListModels(ctx, modelcatalog.ListOptions{
			ProviderID: opts.ProviderID, ExecutionContext: opts.ExecutionContext,
			SkipRefreshIfEmpty: true,
		})
		if err == nil {
			err = checks.RecoverConfiguration(ctx, func(target heartbeat.CheckTarget) bool {
				scope := opts.ExecutionContext
				if scope.Scope != modelcatalog.ExecutionScopeGlobal &&
					(scope.ProfileID != target.Instance.ProfileID || scope.WorkspaceID != target.Instance.WorkspaceID) {
					return false
				}
				for _, model := range models {
					if model.ProviderID == target.TriageProvider && model.ModelID == target.TriageModel &&
						model.AvailabilityState == modelcatalog.AvailabilityStateAvailableLive && !model.Stale {
						return true
					}
				}
				return false
			})
		}
		if err != nil {
			state.logger.ErrorContext(ctx, "daemon: recover checks after model refresh", "error", err)
		}
	}
}
