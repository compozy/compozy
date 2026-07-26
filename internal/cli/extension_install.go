package cli

import (
	"context"

	extensionpkg "github.com/compozy/agh/internal/extension"
)

func installExtension(
	ctx context.Context,
	deps commandDeps,
	prepared preparedExtensionInstall,
	allowUnverified bool,
) (ExtensionRecord, error) {
	client, running, err := daemonClientIfRunning(ctx, deps)
	if err != nil {
		return ExtensionRecord{}, err
	}
	if running {
		return client.InstallExtension(ctx, InstallExtensionRequest{
			Path:            prepared.Path,
			Checksum:        prepared.Checksum,
			AllowUnverified: allowUnverified,
		})
	}

	return withLocalExtensionRegistry(
		ctx,
		deps,
		func(runtime *runtimeContext, registry localExtensionRegistry) (ExtensionRecord, error) {
			if err := extensionpkg.ValidateUnverifiedSideLoad(
				prepared.Manifest.Name,
				prepared.Path,
				runtime.Config.Extensions.Marketplace.AllowUnverified,
				allowUnverified,
			); err != nil {
				return ExtensionRecord{}, err
			}
			if err := installPreparedExtension(
				runtime.HomePaths,
				registry,
				prepared,
				deps.now(),
				allowUnverified,
			); err != nil {
				return ExtensionRecord{}, err
			}
			info, err := registry.Get(prepared.Manifest.Name)
			if err != nil {
				return ExtensionRecord{}, err
			}
			return localExtensionRecord(*info, deps.now, deps.getenv), nil
		},
	)
}
