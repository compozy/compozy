package daemon

import (
	"context"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/modelcatalog"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type loopRuntimeCatalogFactory struct {
	homePaths         compozyconfig.HomePaths
	workspaceResolver workspacepkg.RuntimeResolver
	models            modelcatalog.Service
}

func (f loopRuntimeCatalogFactory) ForWorkspace(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
) (looppkg.RuntimeCatalog, error) {
	cfg, err := resolveLoopServiceConfig(ctx, f.homePaths, f.workspaceResolver, workspaceID)
	if err != nil {
		return nil, err
	}
	return &loopRuntimeCatalog{config: &cfg, models: f.models}, nil
}

type loopRuntimeCatalog struct {
	config *compozyconfig.Config
	models modelcatalog.Service
}

func (c *loopRuntimeCatalog) CanonicalProvider(provider string) string {
	return compozyconfig.CanonicalProviderName(provider)
}

func (c *loopRuntimeCatalog) ValidateRuntime(ctx context.Context, runtime looppkg.RuntimeSpec) error {
	provider := compozyconfig.CanonicalProviderName(runtime.Provider)
	if provider != "" {
		if c == nil || c.config == nil {
			return fmt.Errorf("%w: runtime config is unavailable", looppkg.ErrActionDependencyMissing)
		}
		if _, err := c.config.ResolveProvider(provider); err != nil {
			return looppkg.NewRuntimeValidationError(looppkg.RuntimeValidationItem{
				Field: "provider", Value: runtime.Provider, Reason: "unknown_provider",
			})
		}
	}
	model := strings.TrimSpace(runtime.Model)
	if provider == "" || model == "" || !modelcatalog.HasAuthoritativeProviderCatalog(provider) {
		return nil
	}
	if c.models == nil {
		return fmt.Errorf("%w: model catalog is unavailable", looppkg.ErrActionDependencyMissing)
	}
	models, err := c.models.ListModels(ctx, modelcatalog.ListOptions{
		ProviderID: provider, View: modelcatalog.CatalogViewAll,
		SkipRefreshIfEmpty: true, IncludeAll: true, IncludeStale: true,
	})
	if err != nil {
		return fmt.Errorf("validate runtime model catalog: %w", err)
	}
	for _, candidate := range models {
		if candidate.Available != nil && !*candidate.Available {
			continue
		}
		if strings.TrimSpace(candidate.ModelID) == model {
			return nil
		}
	}
	return looppkg.NewRuntimeValidationError(looppkg.RuntimeValidationItem{
		Field: "model", Value: model, Reason: "unknown_model",
	})
}
