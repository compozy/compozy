package daemon

import (
	"context"
	"fmt"

	compozyconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type loopRuntimeCatalogFactory struct {
	homePaths         compozyconfig.HomePaths
	workspaceResolver workspacepkg.RuntimeResolver
}

func (f loopRuntimeCatalogFactory) ForWorkspace(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
) (looppkg.RuntimeCatalog, error) {
	cfg, err := resolveLoopServiceConfig(ctx, f.homePaths, f.workspaceResolver, workspaceID)
	if err != nil {
		return nil, err
	}
	return &loopRuntimeCatalog{config: &cfg}, nil
}

type loopRuntimeCatalog struct {
	config *compozyconfig.Config
}

func (c *loopRuntimeCatalog) CanonicalProvider(provider string) string {
	return compozyconfig.CanonicalProviderName(provider)
}

func (c *loopRuntimeCatalog) ValidateRuntime(_ context.Context, runtime looppkg.RuntimeSpec) error {
	provider := compozyconfig.CanonicalProviderName(runtime.Provider)
	if provider != "" {
		if c == nil || c.config == nil {
			return fmt.Errorf("%w: runtime config is unavailable", looppkg.ErrActionDependencyMissing)
		}
		if _, err := c.config.ResolveProvider(provider); err != nil {
			return looppkg.NewRuntimeValidationError(looppkg.RuntimeValidationItem{
				Field: watchEventsPayloadProviderKey, Value: runtime.Provider, Reason: "unknown_provider",
			})
		}
	}
	return nil
}
