package cli

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"

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

func extensionInstallValidationReport(
	deps commandDeps,
	plan extensionInstallPlan,
	item ExtensionRecord,
) *extensionpkg.ValidationReport {
	for _, request := range plan.Attempts {
		if request.Source != contract.InstallExtensionSourceLocalPath {
			continue
		}
		path := strings.TrimSpace(request.Ref)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			continue
		}
		report, err := extensionpkg.ValidateBundleReport(path)
		if err != nil {
			continue
		}
		if report.DualManifest || report.Format == item.Format {
			return report
		}
	}
	if strings.TrimSpace(item.Format) != string(extensionpkg.FormatAgentPlugin) {
		return nil
	}
	homePaths, err := deps.resolveHome()
	if err != nil {
		return nil
	}
	path := extensionpkg.ManagedInstallPath(homePaths, item.Name)
	report, err := extensionpkg.ValidateBundleReport(path)
	if err != nil {
		return nil
	}
	return report
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
	apiErr, ok := errors.AsType[*daemonAPIError](err)
	return ok && apiErr.statusCode == http.StatusNotFound
}
