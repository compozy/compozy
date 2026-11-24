package cli

import (
	"context"
	"errors"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/spf13/cobra"
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
	if request.Enable {
		return ExtensionRecord{}, errors.New("cli: extension install --enable requires a running daemon")
	}
	if request.Profile != "" || request.WorkspaceID != "" || request.Scope == workspaceFlagName {
		return ExtensionRecord{}, errors.New("cli: scoped extension installation requires a running daemon")
	}
	if request.Source != contract.InstallExtensionSourceLocalPath {
		return ExtensionRecord{}, errors.New("cli: extension install from a published source requires a running daemon")
	}
	prepared, err := prepareExtensionInstall(request.Ref)
	if err != nil {
		return ExtensionRecord{}, err
	}
	if err := extensionpkg.CheckExpectedDigest(request.ExpectedDigest, prepared.Checksum); err != nil {
		return ExtensionRecord{}, err
	}
	if len(request.Inputs) > 0 || len(prepared.Manifest.Inputs) > 0 {
		return ExtensionRecord{}, errors.New("cli: extension install with manifest inputs requires a running daemon")
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
				request.Replace,
			); err != nil {
				return ExtensionRecord{}, err
			}
			info, err := registry.Get(prepared.Manifest.Name)
			if err != nil {
				return ExtensionRecord{}, err
			}
			return localExtensionRecord(ctx, info, deps.now, deps.getenv)
		},
	)
}

func extensionInstallValidationReport(
	deps commandDeps,
	plan extensionInstallPlan,
	item *ExtensionRecord,
) *extensionpkg.ValidationReport {
	var homePaths compozyconfig.HomePaths
	dataDir := ""
	if strings.TrimSpace(item.Format) == string(extensionpkg.FormatAgentPlugin) {
		var err error
		homePaths, err = deps.resolveHome()
		if err != nil {
			return nil
		}
		dataDir, err = homePaths.ExtensionDataPath(item.Name, "", "")
		if err != nil {
			return nil
		}
	}
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
		report, err := extensionpkg.ValidateBundleReportWithAgentPluginDataDir(path, dataDir, item.Name)
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
	path := extensionpkg.ManagedInstallPath(homePaths, item.Name)
	report, err := extensionpkg.ValidateBundleReportWithAgentPluginDataDir(path, dataDir, item.Name)
	if err != nil {
		return nil
	}
	return report
}

// executeExtensionInstallPlanWithPrompts installs, answering a dependency refusal and then an inputs refusal
// on an interactive terminal; the dependency gate runs before the input gate, so at most two retries follow.
func executeExtensionInstallPlanWithPrompts(
	cmd *cobra.Command, deps commandDeps, plan extensionInstallPlan,
) (ExtensionRecord, error) {
	var installed []string
	for attempt := 0; ; attempt++ {
		item, err := executeExtensionInstallPlan(cmd.Context(), deps, plan)
		if err == nil {
			if len(installed) > 0 {
				item.InstallDependenciesInstalled = appendInstalledDependencies(
					installed,
					item.InstallDependenciesInstalled,
				)
			}
			return item, nil
		}
		if refusal, ok := errors.AsType[*extensionOperationAPIError](err); ok {
			installed = appendInstalledDependencies(installed, refusal.payload.DependenciesInstalled)
		}
		if refusal, ok := errors.AsType[*extensionValidationAPIError](err); ok {
			installed = appendInstalledDependencies(installed, refusal.payload.DependenciesInstalled)
		}
		item.InstallDependenciesInstalled = installed
		if attempt == 2 {
			return item, err
		}
		retry, prompted, promptErr := promptExtensionInstallDependencies(cmd, deps, plan, err)
		if promptErr == nil && !prompted {
			retry, prompted, promptErr = promptExtensionInstallInputs(cmd, deps, plan, err)
		}
		if promptErr != nil {
			return item, promptErr
		}
		if !prompted {
			return item, err
		}
		plan = retry
	}
}

func appendInstalledDependencies(installed, completed []string) []string {
	for _, name := range completed {
		if !slices.Contains(installed, name) {
			installed = append(installed, name)
		}
	}
	return installed
}

func executeExtensionInstallPlan(
	ctx context.Context,
	deps commandDeps,
	plan extensionInstallPlan,
) (ExtensionRecord, error) {
	return runExtensionInstallAttempts(plan.Attempts, func(request InstallExtensionRequest) (ExtensionRecord, error) {
		return installExtension(ctx, deps, request)
	})
}

func runExtensionInstallAttempts[T any](
	attempts []InstallExtensionRequest,
	run func(InstallExtensionRequest) (T, error),
) (T, error) {
	var zero T
	if len(attempts) == 0 {
		return zero, errors.New("cli: extension install plan has no attempts")
	}
	for index, request := range attempts {
		value, err := run(request)
		if err == nil {
			return value, nil
		}
		if index == len(attempts)-1 || !extensionInstallFallbackAllowed(err) {
			return zero, err
		}
	}
	return zero, errors.New("cli: extension install plan exhausted")
}

func extensionInstallFallbackAllowed(err error) bool {
	apiErr, ok := errors.AsType[*daemonAPIError](err)
	return ok && apiErr.statusCode == http.StatusNotFound
}
