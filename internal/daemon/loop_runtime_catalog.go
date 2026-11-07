package daemon

import (
	"context"
	"fmt"
	"log/slog"

	compozyconfig "github.com/compozy/compozy/internal/config"
	diagcontract "github.com/compozy/compozy/internal/diagnosticcontract"
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
	profileID string,
) (looppkg.RuntimeCatalog, error) {
	cfg, err := resolveLoopProfileConfig(ctx, f.homePaths, f.workspaceResolver, workspaceID, profileID)
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
		if err := requireLoopRuntimeProviderEnabled(c.config, provider, "loop_validate"); err != nil {
			return err
		}
	}
	return nil
}

// requireLoopRuntimeProviderEnabled refuses a disabled provider as a runtime validation failure
// whose cause still matches config.ErrProviderDisabled.
func requireLoopRuntimeProviderEnabled(cfg *compozyconfig.Config, provider string, surface string) error {
	err := cfg.RequireEnabledProvider(provider)
	if err == nil {
		return nil
	}
	slog.Info("provider.disabled_refused", "provider", provider, "surface", surface)
	return looppkg.NewRuntimeValidationCauseError(err, looppkg.RuntimeValidationItem{
		Field: watchEventsPayloadProviderKey, Value: provider, Reason: diagcontract.CodeProviderDisabled,
	})
}
