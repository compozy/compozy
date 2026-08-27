package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	compozyconfig "github.com/compozy/compozy/internal/config"
	compozyupdate "github.com/compozy/compozy/internal/update"
	compozyversion "github.com/compozy/compozy/internal/version"
)

func (e *bootstrapExecution) replaceOutdatedDesktopRuntime(status DaemonStatus) (bool, error) {
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
	if err := stopBootstrapDaemon(e.cmd.Context(), e.deps, e.homePaths); err != nil {
		return false, err
	}
	return true, nil
}

func (e *bootstrapExecution) provenance() compozyupdate.DesktopProvenanceMetadata {
	return compozyupdate.DesktopProvenanceMetadata{
		AppVersion:     e.options.appVersion,
		Channel:        e.options.channel,
		RuntimeVersion: compozyversion.Current().Version,
	}
}

func normalizedBootstrapVersion(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "v")
}

func stopBootstrapDaemon(
	ctx context.Context,
	deps commandDeps,
	homePaths compozyconfig.HomePaths,
) error {
	info, running, err := daemonInfo(homePaths, deps)
	if err != nil {
		return fmt.Errorf("cli: inspect outdated desktop runtime: %w", err)
	}
	if !running {
		return nil
	}
	if err := deps.signalProcess(info.PID, syscall.SIGTERM); err != nil &&
		!errors.Is(err, os.ErrProcessDone) && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("cli: stop outdated desktop runtime: %w", err)
	}
	runtimeCtx := &runtimeContext{HomePaths: homePaths}
	if _, err := waitForDaemonStop(ctx, deps, runtimeCtx, info); err != nil {
		return fmt.Errorf("cli: wait for outdated desktop runtime: %w", err)
	}
	return nil
}
