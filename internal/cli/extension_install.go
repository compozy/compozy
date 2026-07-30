package cli

import (
	"context"
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
)

func installExtension(
	ctx context.Context,
	deps commandDeps,
	request InstallExtensionRequest,
) (ExtensionRecord, error) {
	client, running, err := daemonClientIfRunning(ctx, deps)
	if err != nil {
		return ExtensionRecord{}, err
	}
	if running {
		return client.InstallExtension(ctx, request)
	}
	if request.Source != contract.InstallExtensionSourceLocalPath {
		return ExtensionRecord{}, errors.New("cli: extension install from a published source requires a running daemon")
	}
	prepared, err := prepareExtensionInstall(request.Ref)
	if err != nil {
		return ExtensionRecord{}, err
	}

	return withLocalExtensionRegistry(
		ctx,
		deps,
		func(runtime *runtimeContext, registry localExtensionRegistry) (ExtensionRecord, error) {
			if err := extensionpkg.ValidateUnverifiedSideLoad(
				prepared.Manifest.Name,
				prepared.Path,
				runtime.Config.Extensions.Trust.AllowUnverified,
				request.AllowUnverified,
			); err != nil {
				return ExtensionRecord{}, err
			}
			if err := installPreparedExtension(
				runtime.HomePaths,
				registry,
				prepared,
				deps.now(),
				request.AllowUnverified,
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

func executeExtensionInstallPlan(
	ctx context.Context,
	deps commandDeps,
	plan extensionInstallPlan,
) (ExtensionRecord, error) {
	if len(plan.Attempts) == 0 {
		return ExtensionRecord{}, errors.New("cli: extension install plan has no attempts")
	}
	for index, request := range plan.Attempts {
		item, err := installExtension(ctx, deps, request)
		if err == nil {
			return item, nil
		}
		if index == len(plan.Attempts)-1 || !extensionInstallFallbackAllowed(err) {
			return ExtensionRecord{}, err
		}
	}
	return ExtensionRecord{}, errors.New("cli: extension install plan exhausted")
}

func extensionInstallFallbackAllowed(err error) bool {
	var apiErr *daemonAPIError
	return errors.As(err, &apiErr) && apiErr.statusCode == http.StatusNotFound
}
