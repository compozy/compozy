package cli

import (
	"context"
	"errors"
	"fmt"
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
	bootstrapPhaseResolve    bootstrapPhase = "resolve"
	bootstrapPhaseAttach     bootstrapPhase = "attach"
	bootstrapPhaseProvision  bootstrapPhase = "provision"
	bootstrapPhaseStart      bootstrapPhase = "start"
	bootstrapPhaseReady      bootstrapPhase = "ready"
	bootstrapPhaseFailed     bootstrapPhase = "failed"
	bootstrapEventType                      = "bootstrap"
	bootstrapStatusStarted                  = "started"
	bootstrapStatusCompleted                = "completed"
	bootstrapStatusRetrying                 = "retrying"
	bootstrapStatusFailed                   = "failed"
	bootstrapBinaryName                     = "compozy"
)

type bootstrapDaemon struct {
	DaemonStatus
	Origin string `json:"origin"`
	Owned  bool   `json:"owned"`
}

type bootstrapEvent struct {
	Type           string                  `json:"type"`
	Phase          bootstrapPhase          `json:"phase"`
	Status         string                  `json:"status"`
	Resolution     bootstrapResolution     `json:"resolution,omitempty"`
	Attempt        int                     `json:"attempt,omitempty"`
	BackoffMS      int64                   `json:"backoff_ms,omitempty"`
	Classification bootstrapProbeClass     `json:"classification,omitempty"`
	Message        string                  `json:"message"`
	Daemon         *bootstrapDaemon        `json:"daemon,omitempty"`
	Compatibility  *bootstrapCompatibility `json:"compatibility,omitempty"`
}

type bootstrapCompatibility struct {
	Reason  string `json:"reason"`
	Runtime string `json:"runtime"`
	Needed  string `json:"needed"`
}

type bootstrapCompatibilityFailure struct {
	bootstrapCompatibility
	cause error
}

func (e *bootstrapCompatibilityFailure) Error() string { return e.cause.Error() }
func (e *bootstrapCompatibilityFailure) Unwrap() error { return e.cause }

type bootstrapOptions struct {
	bundlePath     string
	minimumRuntime string
	appVersion     string
	channel        string
}

var daemonBootstrapAttemptGate bootstrapAttemptGate

func newDaemonBootstrapCommand(deps commandDeps) *cobra.Command {
	options := bootstrapOptions{}
	cmd := &cobra.Command{
		Use:   bootstrapEventType,
		Short: "Resolve, provision, and start the desktop runtime",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDaemonBootstrap(cmd, deps, options)
		},
	}
	cmd.Flags().StringVar(&options.bundlePath, "bundle-path", "", "Bundled runtime payload path")
	cmd.Flags().StringVar(&options.minimumRuntime, "minimum-runtime", ">=0.0.0", "Minimum compatible runtime version")
	cmd.Flags().StringVar(&options.appVersion, "app-version", "", "Desktop app version for compatibility checks")
	cmd.Flags().StringVar(&options.channel, "channel", "development", "Desktop app release channel")
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
		return writeBootstrapFailure(cmd, bootstrapPhaseResolve, bootstrapProbeUnavailable, err)
	}
	if err := deps.ensureHome(homePaths); err != nil {
		return writeBootstrapFailure(cmd, bootstrapPhaseResolve, bootstrapProbeUnavailable, err)
	}
	bundlePath, err := resolveBootstrapBundlePath(deps, options.bundlePath)
	if err != nil {
		return writeBootstrapFailure(cmd, bootstrapPhaseResolve, bootstrapProbeUnavailable, err)
	}
	installedPath := bootstrapRuntimePath(homePaths)
	if err := writeBootstrapEvent(cmd, bootstrapEvent{
		Type: bootstrapEventType, Phase: bootstrapPhaseResolve, Status: bootstrapStatusStarted,
		Message: "Resolving the CompozyOS runtime.",
	}); err != nil {
		return err
	}

	execution := bootstrapExecution{
		cmd: cmd, deps: deps, options: options, homePaths: homePaths,
		bundlePath: bundlePath, installedPath: installedPath,
	}
	return execution.resolveAndStart()
}

type bootstrapExecution struct {
	cmd           *cobra.Command
	deps          commandDeps
	options       bootstrapOptions
	homePaths     compozyconfig.HomePaths
	bundlePath    string
	installedPath string
}

func (e *bootstrapExecution) resolveAndStart() error {
	status, running, err := probeBootstrapDaemon(e.cmd.Context(), e.deps, e.homePaths)
	if err == nil && running {
		replaced, replaceErr := e.replaceOutdatedDesktopRuntime(status)
		if replaceErr != nil {
			return writeBootstrapFailure(
				e.cmd, bootstrapPhaseProvision, bootstrapProbeListeningUnhealthy, replaceErr,
			)
		}
		if !replaced {
			return writeAttachedBootstrap(e.cmd, e.options, status)
		}
	}
	if err := cleanupStaleBootstrapDaemonRecord(e.homePaths, e.deps); err != nil {
		return writeBootstrapFailure(e.cmd, bootstrapPhaseResolve, bootstrapProbeStaleRecord, err)
	}

	runtimePath, owned := resolveBootstrapRuntime(e.deps, e.homePaths, e.installedPath)
	installed := regularFileExists(runtimePath)
	resolution := resolveBootstrapAction(bootstrapProbe{Installed: installed})
	if installed && owned {
		matches, matchErr := compozyupdate.DesktopRuntimeMatchesBundle(
			e.homePaths, runtimePath, e.bundlePath, e.provenance(),
		)
		if matchErr != nil {
			return writeBootstrapFailure(
				e.cmd, bootstrapPhaseResolve, bootstrapProbeUnavailable, matchErr,
			)
		}
		if !matches {
			resolution = bootstrapResolutionProvision
		}
	}
	if err := provisionBootstrapRuntime(e.cmd, &bootstrapProvisionRequest{
		resolution:    resolution,
		homePaths:     e.homePaths,
		bundlePath:    e.bundlePath,
		installedPath: e.installedPath,
		provenance:    e.provenance(),
	}); err != nil {
		return err
	}
	if resolution == bootstrapResolutionProvision {
		runtimePath = e.installedPath
		owned = true
	}
	return e.start(resolution, runtimePath, owned)
}

func resolveBootstrapRuntime(
	deps commandDeps,
	homePaths compozyconfig.HomePaths,
	installedPath string,
) (string, bool) {
	if regularFileExists(installedPath) {
		return installedPath, compozyupdate.RuntimeOwnedByDesktopApp(homePaths, installedPath)
	}
	operatorPath, err := deps.lookPath(bootstrapBinaryName)
	if err == nil && regularFileExists(operatorPath) {
		return operatorPath, false
	}
	return installedPath, false
}

func writeAttachedBootstrap(cmd *cobra.Command, options bootstrapOptions, status DaemonStatus) error {
	if err := validateBootstrapCompatibility(status, options.minimumRuntime, options.appVersion); err != nil {
		return writeBootstrapFailure(cmd, bootstrapPhaseAttach, bootstrapProbeListeningUnhealthy, err)
	}
	daemon, err := bootstrapDaemonFromStatus(status, false)
	if err != nil {
		return writeBootstrapFailure(cmd, bootstrapPhaseAttach, bootstrapProbeListeningUnhealthy, err)
	}
	return writeBootstrapEvent(cmd, bootstrapEvent{
		Type: bootstrapEventType, Phase: bootstrapPhaseReady, Status: bootstrapStatusCompleted,
		Resolution: bootstrapResolutionAttach, Message: "Attached to the running CompozyOS runtime.",
		Daemon: daemon,
	})
}

func (e *bootstrapExecution) start(resolution bootstrapResolution, runtimePath string, owned bool) error {
	var lastErr error
	for attempt := 1; attempt <= bootstrapMaximumAttempts; attempt++ {
		if err := writeBootstrapEvent(e.cmd, bootstrapEvent{
			Type: bootstrapEventType, Phase: bootstrapPhaseStart, Status: bootstrapStatusStarted,
			Resolution: resolution, Attempt: attempt, Message: "Starting the CompozyOS runtime.",
		}); err != nil {
			return err
		}
		status, startErr := runBootstrapDaemonDetached(e.cmd.Context(), e.deps, e.homePaths, runtimePath)
		lastErr = startErr
		if lastErr == nil {
			return e.writeStarted(status, resolution, attempt, owned)
		}
		if bootstrapShouldGiveUp(attempt) {
			break
		}
		delay := bootstrapBackoff(attempt)
		if err := writeBootstrapEvent(e.cmd, bootstrapEvent{
			Type: bootstrapEventType, Phase: bootstrapPhaseStart, Status: bootstrapStatusRetrying,
			Resolution: resolution, Attempt: attempt, BackoffMS: delay.Milliseconds(),
			Classification: bootstrapProbeListeningUnhealthy,
			Message:        "The runtime did not become ready; retrying with bounded backoff.",
		}); err != nil {
			return err
		}
		if err := waitBootstrapBackoff(e.cmd.Context(), delay); err != nil {
			return err
		}
	}
	return writeBootstrapFailure(
		e.cmd,
		bootstrapPhaseFailed,
		bootstrapProbeListeningUnhealthy,
		fmt.Errorf("cli: runtime did not become ready after %d attempts: %w", bootstrapMaximumAttempts, lastErr),
	)
}

func (e *bootstrapExecution) writeStarted(
	status DaemonStatus,
	resolution bootstrapResolution,
	attempt int,
	owned bool,
) error {
	if err := validateBootstrapCompatibility(status, e.options.minimumRuntime, e.options.appVersion); err != nil {
		return writeBootstrapFailure(e.cmd, bootstrapPhaseReady, bootstrapProbeListeningUnhealthy, err)
	}
	daemon, err := bootstrapDaemonFromStatus(status, owned)
	if err != nil {
		return writeBootstrapFailure(e.cmd, bootstrapPhaseReady, bootstrapProbeListeningUnhealthy, err)
	}
	return writeBootstrapEvent(e.cmd, bootstrapEvent{
		Type: bootstrapEventType, Phase: bootstrapPhaseReady, Status: bootstrapStatusCompleted,
		Resolution: resolution, Attempt: attempt, Message: "The CompozyOS runtime is ready.",
		Daemon: daemon,
	})
}

func bootstrapDaemonFromStatus(status DaemonStatus, owned bool) (*bootstrapDaemon, error) {
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
		Owned:        owned,
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
		resolved, err := filepath.Abs(value)
		if err != nil {
			return "", fmt.Errorf("cli: resolve bootstrap bundle override %q: %w", value, err)
		}
		return resolved, nil
	}
	path, err := deps.executable()
	if err != nil {
		return "", fmt.Errorf("cli: resolve bundled runtime: %w", err)
	}
	resolved, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("cli: resolve bootstrap executable bundle path %q: %w", path, err)
	}
	return resolved, nil
}

func bootstrapRuntimePath(homePaths compozyconfig.HomePaths) string {
	name := bootstrapBinaryName
	if runtime.GOOS == platformWindows {
		name += ".exe"
	}
	return filepath.Join(homePaths.BinDir, name)
}

func probeBootstrapDaemon(
	ctx context.Context,
	deps commandDeps,
	homePaths compozyconfig.HomePaths,
) (DaemonStatus, bool, error) {
	info, running, err := daemonInfo(homePaths, deps)
	if err != nil || !running {
		return DaemonStatus{}, false, err
	}
	client, err := deps.newClient(LocalClientTarget(homePaths.DaemonSocket))
	if err != nil {
		return DaemonStatus{}, false, err
	}
	status, err := client.DaemonStatus(ctx)
	if err != nil {
		return DaemonStatus{}, false, err
	}
	if status.PID <= 0 || strings.TrimSpace(status.Status) != daemonRunningStatus {
		return status, false, errors.New("cli: running daemon did not report healthy status")
	}
	if status.PID != info.PID || !status.StartedAt.Equal(info.StartedAt) ||
		status.HTTPPort != info.Port || filepath.Clean(status.Socket) != filepath.Clean(homePaths.DaemonSocket) {
		return status, false, errors.New("cli: daemon status does not match the local discovery record")
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
	return waitForBootstrapDaemonStart(ctx, deps, child)
}

func waitForBootstrapDaemonStart(
	ctx context.Context,
	deps commandDeps,
	child daemonProcess,
) (DaemonStatus, error) {
	status, waitErr := waitForDaemonStart(ctx, deps, child)
	if waitErr == nil {
		return status, nil
	}
	terminateErr := child.Terminate()
	reapErr := child.Wait()
	if terminateErr != nil {
		terminateErr = fmt.Errorf("cli: terminate unready detached daemon: %w", terminateErr)
	}
	if reapErr != nil {
		reapErr = fmt.Errorf("cli: reap unready detached daemon: %w", reapErr)
	}
	return DaemonStatus{}, errors.Join(waitErr, terminateErr, reapErr)
}

func validateBootstrapCompatibility(status DaemonStatus, minimumRuntime string, appVersion string) error {
	runtimeVersion, err := semver.NewVersion(strings.TrimPrefix(strings.TrimSpace(status.Version), "v"))
	if err != nil {
		return fmt.Errorf(
			"cli: runtime version %q is invalid; repair the runtime installation: %w",
			status.Version,
			err,
		)
	}
	constraint, err := semver.NewConstraint(strings.TrimSpace(minimumRuntime))
	if err != nil {
		return fmt.Errorf("cli: minimum runtime constraint %q is invalid: %w", minimumRuntime, err)
	}
	if !constraint.Check(runtimeVersion) {
		cause := fmt.Errorf(
			"cli: runtime %s does not satisfy %s; repair or update the runtime before attaching",
			status.Version,
			minimumRuntime,
		)
		return &bootstrapCompatibilityFailure{
			bootstrapCompatibility: bootstrapCompatibility{
				Reason: "runtime_below_minimum", Runtime: status.Version, Needed: minimumRuntime,
			},
			cause: cause,
		}
	}
	if strings.TrimSpace(appVersion) == "" || strings.TrimSpace(status.MinAppVersion) == "" {
		return nil
	}
	if err := compozyupdate.CheckRuntimeCompatibility(compozyupdate.Compatibility{
		RuntimeVersion: status.Version, MinAppVersion: status.MinAppVersion,
	}, compozyupdate.InstalledApp{Present: true, Version: appVersion}); err != nil {
		cause := fmt.Errorf("cli: %w; update or repair the desktop app before attaching", err)
		return &bootstrapCompatibilityFailure{
			bootstrapCompatibility: bootstrapCompatibility{
				Reason: "app_below_minimum", Runtime: status.Version, Needed: status.MinAppVersion,
			},
			cause: cause,
		}
	}
	return nil
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
