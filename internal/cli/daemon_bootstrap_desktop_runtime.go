package cli

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"

	compozyupdate "github.com/compozy/compozy/internal/update"
)

// replaceOutdatedDesktopRuntime replaces an older desktop runtime while preserving compatible newer versions.
func (e *bootstrapExecution) replaceOutdatedDesktopRuntime(status DaemonStatus) (bool, error) {
	newer := bootstrapRuntimeIsNewer(status.Version, e.provenance().RuntimeVersion)
	acceptsApp := bootstrapRuntimeAcceptsApp(status.MinAppVersion, e.options.appVersion)
	if newer && acceptsApp {
		return false, nil
	}
	if !regularFileExists(e.installedPath) ||
		!compozyupdate.RuntimeOwnedByDesktopApp(e.homePaths, e.installedPath) {
		return false, nil
	}
	provenance := e.provenance()
	matches, err := compozyupdate.DesktopRuntimeMatchesBundle(
		e.homePaths,
		e.installedPath,
		e.bundlePath,
		provenance,
	)
	if err != nil {
		return false, err
	}
	wantVersion := normalizedBootstrapVersion(provenance.RuntimeVersion)
	runningVersion := normalizedBootstrapVersion(status.Version)
	if matches && runningVersion == wantVersion {
		return false, nil
	}
	if err := provisionBootstrapRuntime(e.cmd, &bootstrapProvisionRequest{
		resolution: bootstrapResolutionProvision,
		homePaths:  e.homePaths, bundlePath: e.bundlePath, installedPath: e.installedPath,
		provenance: provenance,
		restart: func() error {
			return newCLIUpdateRuntime(e.deps, e.homePaths).RestartDaemon(e.cmd.Context())
		},
	}); err != nil {
		return false, fmt.Errorf("cli: replace outdated desktop runtime: %w", err)
	}
	// A newer runtime that rejects this app would otherwise restart from the same binary and strand the
	// shell on version skew, so the bundled runtime must replace it.
	e.bundleRequired = newer
	return true, nil
}

// restartRejectingDesktopRuntime re-resolves once when a started desktop-owned runtime rejects this app.
func (e *bootstrapExecution) restartRejectingDesktopRuntime(status DaemonStatus, owned bool) bool {
	return owned && !e.bundleRequired &&
		bootstrapRuntimeIsNewer(status.Version, e.provenance().RuntimeVersion) &&
		!bootstrapRuntimeAcceptsApp(status.MinAppVersion, e.options.appVersion)
}

// bootstrapRuntimeAcceptsApp mirrors the attach compatibility check for the runtime's minimum app version.
func bootstrapRuntimeAcceptsApp(minAppVersion, appVersion string) bool {
	if strings.TrimSpace(appVersion) == "" || strings.TrimSpace(minAppVersion) == "" {
		return true
	}
	return compozyupdate.CheckRuntimeCompatibility(compozyupdate.Compatibility{
		MinAppVersion: minAppVersion,
	}, compozyupdate.InstalledApp{Present: true, Version: appVersion}) == nil
}

// bootstrapRuntimeIsNewer compares release versions without treating development builds as newer.
func bootstrapRuntimeIsNewer(runningVersion, bundledVersion string) bool {
	running, runningErr := semver.NewVersion(normalizedBootstrapVersion(runningVersion))
	bundled, bundledErr := semver.NewVersion(normalizedBootstrapVersion(bundledVersion))
	return runningErr == nil && bundledErr == nil && running.GreaterThan(bundled)
}

func (e *bootstrapExecution) provenance() compozyupdate.DesktopProvenanceMetadata {
	return compozyupdate.DesktopProvenanceMetadata{
		AppVersion:     e.options.appVersion,
		Channel:        e.options.channel,
		RuntimeVersion: e.runtimeVersion,
	}
}

func normalizedBootstrapVersion(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "v")
}
