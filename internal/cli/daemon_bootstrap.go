package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	compozyconfig "github.com/compozy/compozy/internal/config"
	compozyupdate "github.com/compozy/compozy/internal/update"
	"github.com/spf13/cobra"
)

type bootstrapPhase string

const (
	bootstrapPhaseResolve   bootstrapPhase = "resolve"
	bootstrapPhaseAttach    bootstrapPhase = "attach"
	bootstrapPhaseProvision bootstrapPhase = "provision"
	bootstrapPhaseStart     bootstrapPhase = "start"
	bootstrapPhaseReady     bootstrapPhase = "ready"
	bootstrapPhaseFailed    bootstrapPhase = "failed"
)

type bootstrapDaemon struct {
	DaemonStatus
	Origin string `json:"origin"`
}

type bootstrapEvent struct {
	Type           string              `json:"type"`
	Phase          bootstrapPhase      `json:"phase"`
	Status         string              `json:"status"`
	Resolution     bootstrapResolution `json:"resolution,omitempty"`
	Attempt        int                 `json:"attempt,omitempty"`
	BackoffMS      int64               `json:"backoff_ms,omitempty"`
	Classification bootstrapProbeClass `json:"classification,omitempty"`
	Message        string              `json:"message"`
	Daemon         *bootstrapDaemon    `json:"daemon,omitempty"`
}

type bootstrapOptions struct {
	bundlePath     string
	minimumRuntime string
	appVersion     string
}

var daemonBootstrapAttemptGate bootstrapAttemptGate

func newDaemonBootstrapCommand(deps commandDeps) *cobra.Command {
	options := bootstrapOptions{}
	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Resolve, provision, and start the desktop runtime",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDaemonBootstrap(cmd, deps, options)
		},
	}
	cmd.Flags().StringVar(&options.bundlePath, "bundle-path", "", "Bundled runtime payload path")
	cmd.Flags().StringVar(&options.minimumRuntime, "minimum-runtime", ">=0.0.0", "Minimum compatible runtime version")
	cmd.Flags().StringVar(&options.appVersion, "app-version", "", "Desktop app version for compatibility checks")
	return cmd
}

func runDaemonBootstrap(cmd *cobra.Command, deps commandDeps, options bootstrapOptions) error {
	if cmd == nil {
		return errors.New("cli: bootstrap command is required")
	}
	finishAttempt, err := daemonBootstrapAttemptGate.Begin()
	if err != nil {
		return writeBootstrapFailure(cmd, bootstrapPhaseResolve, bootstrapProbeUnavailable, err)
	}
	defer finishAttempt()
	homePaths, err := deps.resolveHome()
	if err != nil {
		return err
	}
	if err := deps.ensureHome(homePaths); err != nil {
		return err
	}
	bundlePath, err := resolveBootstrapBundlePath(deps, options.bundlePath)
	if err != nil {
		return err
	}
	installedPath := bootstrapRuntimePath(homePaths)
	if err := writeBootstrapEvent(cmd, bootstrapEvent{
		Type: "bootstrap", Phase: bootstrapPhaseResolve, Status: "started", Message: "Resolving the CompozyOS runtime.",
	}); err != nil {
		return err
	}

	status, running, err := probeBootstrapDaemon(cmd.Context(), deps, homePaths)
	if err == nil && running {
		if err := validateBootstrapCompatibility(status, options.minimumRuntime, options.appVersion); err != nil {
			return writeBootstrapFailure(cmd, bootstrapPhaseAttach, bootstrapProbeListeningUnhealthy, err)
		}
		daemon, err := bootstrapDaemonFromStatus(status)
		if err != nil {
			return writeBootstrapFailure(cmd, bootstrapPhaseAttach, bootstrapProbeListeningUnhealthy, err)
		}
		return writeBootstrapEvent(cmd, bootstrapEvent{
			Type: "bootstrap", Phase: bootstrapPhaseReady, Status: "completed",
			Resolution: bootstrapResolutionAttach, Message: "Attached to the running CompozyOS runtime.",
			Daemon: daemon,
		})
	}
	if err := cleanupStaleBootstrapDaemonRecord(homePaths, deps); err != nil {
		return writeBootstrapFailure(cmd, bootstrapPhaseResolve, bootstrapProbeStaleRecord, err)
	}

	installed := regularFileExists(installedPath)
	resolution := resolveBootstrapAction(bootstrapProbe{Installed: installed})
	if resolution == bootstrapResolutionProvision {
		if err := writeBootstrapEvent(cmd, bootstrapEvent{
			Type: "bootstrap", Phase: bootstrapPhaseProvision, Status: "started",
			Resolution: resolution, Message: "Provisioning the bundled CompozyOS runtime.",
		}); err != nil {
			return err
		}
		if err := provisionBundledRuntime(bundlePath, installedPath); err != nil {
			return writeBootstrapFailure(cmd, bootstrapPhaseProvision, bootstrapProbeUnavailable, err)
		}
		if err := writeBootstrapEvent(cmd, bootstrapEvent{
			Type: "bootstrap", Phase: bootstrapPhaseProvision, Status: "completed",
			Resolution: resolution, Message: "Provisioned the bundled CompozyOS runtime.",
		}); err != nil {
			return err
		}
	}

	var lastErr error
	for attempt := 1; attempt <= bootstrapMaximumAttempts; attempt++ {
		if err := writeBootstrapEvent(cmd, bootstrapEvent{
			Type: "bootstrap", Phase: bootstrapPhaseStart, Status: "started",
			Resolution: resolution, Attempt: attempt, Message: "Starting the CompozyOS runtime.",
		}); err != nil {
			return err
		}
		status, lastErr = runBootstrapDaemonDetached(cmd.Context(), deps, homePaths, installedPath)
		if lastErr == nil {
			if compatibilityErr := validateBootstrapCompatibility(status, options.minimumRuntime, options.appVersion); compatibilityErr != nil {
				return writeBootstrapFailure(cmd, bootstrapPhaseReady, bootstrapProbeListeningUnhealthy, compatibilityErr)
			}
			daemon, daemonErr := bootstrapDaemonFromStatus(status)
			if daemonErr != nil {
				return writeBootstrapFailure(cmd, bootstrapPhaseReady, bootstrapProbeListeningUnhealthy, daemonErr)
			}
			return writeBootstrapEvent(cmd, bootstrapEvent{
				Type: "bootstrap", Phase: bootstrapPhaseReady, Status: "completed",
				Resolution: resolution, Attempt: attempt, Message: "The CompozyOS runtime is ready.",
				Daemon: daemon,
			})
		}
		if bootstrapShouldGiveUp(attempt) {
			break
		}
		delay := bootstrapBackoff(attempt)
		if err := writeBootstrapEvent(cmd, bootstrapEvent{
			Type: "bootstrap", Phase: bootstrapPhaseStart, Status: "retrying",
			Resolution: resolution, Attempt: attempt, BackoffMS: delay.Milliseconds(),
			Classification: bootstrapProbeListeningUnhealthy,
			Message:        "The runtime did not become ready; retrying with bounded backoff.",
		}); err != nil {
			return err
		}
		if err := waitBootstrapBackoff(cmd.Context(), delay); err != nil {
			return err
		}
	}
	return writeBootstrapFailure(
		cmd,
		bootstrapPhaseFailed,
		bootstrapProbeListeningUnhealthy,
		fmt.Errorf("cli: runtime did not become ready after %d attempts: %w", bootstrapMaximumAttempts, lastErr),
	)
}

func bootstrapDaemonFromStatus(status DaemonStatus) (*bootstrapDaemon, error) {
	host := strings.TrimSpace(status.HTTPHost)
	ip := net.ParseIP(host)
	if (ip == nil || !ip.IsLoopback()) && !strings.EqualFold(host, "localhost") {
		return nil, fmt.Errorf("cli: daemon reported non-loopback HTTP host %q", host)
	}
	if status.HTTPPort < 1 || status.HTTPPort > 65535 {
		return nil, fmt.Errorf("cli: daemon reported invalid HTTP port %d", status.HTTPPort)
	}
	return &bootstrapDaemon{
		DaemonStatus: status,
		Origin:       "http://" + net.JoinHostPort(status.HTTPHost, fmt.Sprintf("%d", status.HTTPPort)),
	}, nil
}

func cleanupStaleBootstrapDaemonRecord(homePaths compozyconfig.HomePaths, deps commandDeps) error {
	info, err := deps.readDaemonInfo(homePaths.DaemonInfo)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if deps.processAlive(info.PID) && deps.processMatchesStartTime(info.PID, info.StartedAt) {
		return nil
	}
	if err := deps.removeFile(homePaths.DaemonInfo); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cli: remove stale daemon record: %w", err)
	}
	return nil
}

func resolveBootstrapBundlePath(deps commandDeps, override string) (string, error) {
	if value := strings.TrimSpace(override); value != "" {
		return filepath.Abs(value)
	}
	path, err := deps.executable()
	if err != nil {
		return "", fmt.Errorf("cli: resolve bundled runtime: %w", err)
	}
	return filepath.Abs(path)
}

func bootstrapRuntimePath(homePaths compozyconfig.HomePaths) string {
	name := "compozy"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(homePaths.BinDir, name)
}

func probeBootstrapDaemon(
	ctx context.Context,
	deps commandDeps,
	homePaths compozyconfig.HomePaths,
) (DaemonStatus, bool, error) {
	_, running, err := daemonInfo(homePaths, deps)
	if err != nil || !running {
		return DaemonStatus{}, false, err
	}
	client, err := clientFromDeps(deps)
	if err != nil {
		return DaemonStatus{}, false, err
	}
	status, err := client.DaemonStatus(ctx)
	if err != nil {
		return DaemonStatus{}, false, err
	}
	if status.PID <= 0 || strings.TrimSpace(status.Status) != "running" {
		return status, false, errors.New("cli: running daemon did not report healthy status")
	}
	return status, true, nil
}

func runBootstrapDaemonDetached(
	ctx context.Context,
	deps commandDeps,
	homePaths compozyconfig.HomePaths,
	binaryPath string,
) (status DaemonStatus, returnErr error) {
	updateLock, err := acquireDaemonStartUpdateLock(daemonStartUpdateLockPath(homePaths))
	if err != nil {
		return DaemonStatus{}, err
	}
	defer func() {
		returnErr = errors.Join(returnErr, updateLock.Release())
	}()
	if _, running, err := daemonInfo(homePaths, deps); err != nil {
		return DaemonStatus{}, err
	} else if running {
		status, healthy, probeErr := probeBootstrapDaemon(ctx, deps, homePaths)
		if probeErr == nil && healthy {
			return status, nil
		}
		return DaemonStatus{}, errors.Join(errors.New("cli: daemon is listening but unhealthy"), probeErr)
	}
	child, err := spawnDetachedDaemonProcess(ctx, homePaths, func() (string, error) {
		return binaryPath, nil
	})
	if err != nil {
		return DaemonStatus{}, err
	}
	return waitForDaemonStart(ctx, deps, child)
}

func validateBootstrapCompatibility(status DaemonStatus, minimumRuntime string, appVersion string) error {
	runtimeVersion, err := semver.NewVersion(strings.TrimPrefix(strings.TrimSpace(status.Version), "v"))
	if err != nil {
		return fmt.Errorf("cli: runtime version %q is invalid; repair the runtime installation: %w", status.Version, err)
	}
	constraint, err := semver.NewConstraint(strings.TrimSpace(minimumRuntime))
	if err != nil {
		return fmt.Errorf("cli: minimum runtime constraint %q is invalid: %w", minimumRuntime, err)
	}
	if !constraint.Check(runtimeVersion) {
		return fmt.Errorf(
			"cli: runtime %s does not satisfy %s; repair or update the runtime before attaching",
			status.Version,
			minimumRuntime,
		)
	}
	if strings.TrimSpace(appVersion) == "" || strings.TrimSpace(status.MinAppVersion) == "" {
		return nil
	}
	if err := compozyupdate.CheckRuntimeCompatibility(compozyupdate.Compatibility{
		RuntimeVersion: status.Version, MinAppVersion: status.MinAppVersion,
	}, appVersion); err != nil {
		return fmt.Errorf("cli: %w; update or repair the desktop app before attaching", err)
	}
	return nil
}

func provisionBundledRuntime(sourcePath string, targetPath string) (returnErr error) {
	sourcePath = filepath.Clean(sourcePath)
	targetPath = filepath.Clean(targetPath)
	if sourcePath == targetPath {
		return nil
	}
	if !regularFileExists(sourcePath) {
		return fmt.Errorf("cli: bundled runtime payload %q is missing", sourcePath)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
		return fmt.Errorf("cli: create runtime bin directory: %w", err)
	}
	tempFile, err := os.CreateTemp(filepath.Dir(targetPath), ".compozy-bootstrap-*")
	if err != nil {
		return fmt.Errorf("cli: create runtime provision temp file: %w", err)
	}
	tempPath := tempFile.Name()
	tempOpen := true
	defer func() {
		var closeErr error
		if tempOpen {
			closeErr = tempFile.Close()
		}
		removeErr := os.Remove(tempPath)
		if errors.Is(removeErr, os.ErrNotExist) {
			removeErr = nil
		}
		returnErr = errors.Join(returnErr, closeErr, removeErr)
	}()
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("cli: open bundled runtime payload: %w", err)
	}
	if _, err := io.Copy(tempFile, source); err != nil {
		closeErr := source.Close()
		return errors.Join(fmt.Errorf("cli: copy bundled runtime payload: %w", err), closeErr)
	}
	if err := source.Close(); err != nil {
		return fmt.Errorf("cli: close bundled runtime payload: %w", err)
	}
	if err := tempFile.Chmod(0o700); err != nil {
		return fmt.Errorf("cli: make provisioned runtime executable: %w", err)
	}
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("cli: sync provisioned runtime: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("cli: close provisioned runtime: %w", err)
	}
	tempOpen = false
	if err := os.Link(tempPath, targetPath); err != nil {
		if errors.Is(err, os.ErrExist) && regularFileExists(targetPath) {
			return nil
		}
		return fmt.Errorf("cli: publish provisioned runtime: %w", err)
	}
	dir, err := os.Open(filepath.Dir(targetPath))
	if err != nil {
		return fmt.Errorf("cli: open runtime bin directory: %w", err)
	}
	if err := dir.Sync(); err != nil {
		closeErr := dir.Close()
		return errors.Join(fmt.Errorf("cli: sync runtime bin directory: %w", err), closeErr)
	}
	if err := dir.Close(); err != nil {
		return fmt.Errorf("cli: close runtime bin directory: %w", err)
	}
	return nil
}

func regularFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func waitBootstrapBackoff(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func writeBootstrapEvent(cmd *cobra.Command, event bootstrapEvent) error {
	mode, err := resolveOutputFormat(cmd)
	if err != nil {
		return err
	}
	if mode == OutputJSON || mode == OutputJSONL {
		return writeJSONLineWithoutWorkspaceResolution(cmd, event)
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", event.Phase, event.Message)
	return err
}

func writeBootstrapFailure(
	cmd *cobra.Command,
	phase bootstrapPhase,
	classification bootstrapProbeClass,
	cause error,
) error {
	if err := writeBootstrapEvent(cmd, bootstrapEvent{
		Type: "bootstrap", Phase: phase, Status: "failed", Classification: classification,
		Message: cause.Error(),
	}); err != nil {
		return err
	}
	return cause
}
