package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

var acquireDaemonLock = func(path string, pid int, processAlive func(int) bool) (*Lock, error) {
	return acquireLock(path, pid, lockDeps{
		processAlive: processAlive,
	})
}

var resolveDaemonHomePaths = compozyconfig.ResolveHomePaths

// StartOutcome describes the bootstrap result for one daemon start attempt.
type StartOutcome string

const (
	StartOutcomeStarted        StartOutcome = "started"
	StartOutcomeAlreadyRunning StartOutcome = "already_running"
)

// StartOptions control daemon bootstrap behavior.
type StartOptions struct {
	HomePaths    compozyconfig.HomePaths
	Version      string
	PID          int
	HTTPPort     int
	Now          func() time.Time
	ProcessAlive func(int) bool
	Healthy      func(context.Context, Info) error
	Prepare      func(context.Context, *Host) error
}

// StartResult captures the result of a daemon bootstrap attempt.
type StartResult struct {
	Outcome StartOutcome
	Paths   compozyconfig.HomePaths
	Info    Info
	Host    *Host
}

// Status captures the last known daemon readiness view.
type Status struct {
	Paths   compozyconfig.HomePaths
	State   ReadyState
	Healthy bool
	Info    *Info
}

// Host owns one daemon bootstrap instance and its singleton lock.
type Host struct {
	paths        compozyconfig.HomePaths
	lock         *Lock
	info         Info
	now          func() time.Time
	processAlive func(int) bool
	healthy      func(context.Context, Info) error
}

// Start bootstraps the daemon home layout, acquires singleton ownership, and marks readiness.
func Start(ctx context.Context, opts StartOptions) (_ StartResult, retErr error) {
	if ctx == nil {
		ctx = context.Background()
	}

	resolved, err := normalizeStartOptions(opts)
	if err != nil {
		return StartResult{}, err
	}
	if err := compozyconfig.EnsureHomeLayout(resolved.HomePaths); err != nil {
		return StartResult{}, fmt.Errorf("daemon: ensure home layout: %w", err)
	}

	lock, err := acquireDaemonLock(resolved.HomePaths.LockPath, resolved.PID, resolved.ProcessAlive)
	if err != nil {
		info, alreadyRunning := existingHealthyDaemonInfo(ctx, resolved.HomePaths.InfoPath, resolved.Healthy)
		if alreadyRunning {
			return StartResult{
				Outcome: StartOutcomeAlreadyRunning,
				Paths:   resolved.HomePaths,
				Info:    info,
			}, nil
		}
		return StartResult{}, err
	}

	host := &Host{
		paths:        resolved.HomePaths,
		lock:         lock,
		now:          resolved.Now,
		processAlive: resolved.ProcessAlive,
		healthy:      resolved.Healthy,
	}
	defer func() {
		if retErr == nil {
			return
		}
		retErr = errors.Join(retErr, host.Close(context.Background()))
	}()

	if err := host.cleanupStaleRuntime(ctx); err != nil {
		return StartResult{}, err
	}

	host.info = Info{
		PID:        resolved.PID,
		Version:    strings.TrimSpace(resolved.Version),
		SocketPath: resolved.HomePaths.SocketPath,
		HTTPPort:   resolved.HTTPPort,
		StartedAt:  resolved.Now().UTC(),
		State:      ReadyStateStarting,
	}
	if err := WriteInfo(host.paths.InfoPath, host.info); err != nil {
		return StartResult{}, fmt.Errorf("daemon: write starting info: %w", err)
	}

	if resolved.Prepare != nil {
		if err := resolved.Prepare(ctx, host); err != nil {
			return StartResult{}, fmt.Errorf("daemon: prepare startup: %w", err)
		}
	}
	if err := host.MarkReady(ctx); err != nil {
		return StartResult{}, err
	}

	return StartResult{
		Outcome: StartOutcomeStarted,
		Paths:   host.paths,
		Info:    host.info,
		Host:    host,
	}, nil
}

// QueryStatus reports the current daemon readiness without guessing from the filesystem layout.
func QueryStatus(ctx context.Context, paths compozyconfig.HomePaths, opts ProbeOptions) (Status, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	resolvedPaths, processAlive, healthy, err := normalizeProbeOptions(paths, opts)
	if err != nil {
		return Status{}, err
	}

	info, err := ReadInfo(resolvedPaths.InfoPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Status{Paths: resolvedPaths, State: ReadyStateStopped}, nil
		}
		return Status{}, err
	}

	healthyErr := healthy(ctx, info)
	status := Status{
		Paths:   resolvedPaths,
		State:   info.State,
		Healthy: healthyErr == nil,
		Info:    &info,
	}
	if !processAlive(info.PID) {
		status.State = ReadyStateStopped
		status.Healthy = false
	}

	return status, nil
}

// ProbeOptions control readiness checks for an existing daemon instance.
type ProbeOptions struct {
	ProcessAlive func(int) bool
	Healthy      func(context.Context, Info) error
}

// Paths returns the home-scoped daemon paths for this host.
func (h *Host) Paths() compozyconfig.HomePaths {
	if h == nil {
		return compozyconfig.HomePaths{}
	}
	return h.paths
}

// Info returns the current persisted daemon discovery info for this host.
func (h *Host) Info() Info {
	if h == nil {
		return Info{}
	}
	return h.info
}

// MarkReady persists the ready state once startup preparation is complete.
func (h *Host) MarkReady(_ context.Context) error {
	if h == nil {
		return errors.New("daemon: host is required")
	}
	h.info.State = ReadyStateReady
	if err := WriteInfo(h.paths.InfoPath, h.info); err != nil {
		return fmt.Errorf("daemon: write ready info: %w", err)
	}
	return nil
}

// SetHTTPPort persists the effective HTTP port once the transport listener binds.
func (h *Host) SetHTTPPort(_ context.Context, port int) error {
	if h == nil {
		return errors.New("daemon: host is required")
	}
	if port < 0 || port > 65535 {
		return fmt.Errorf("daemon: daemon http port must be between 0 and 65535: %d", port)
	}

	h.info.HTTPPort = port
	if err := WriteInfo(h.paths.InfoPath, h.info); err != nil {
		return fmt.Errorf("daemon: write daemon info with http port: %w", err)
	}
	return nil
}

// Close removes daemon discovery state and releases singleton ownership.
func (h *Host) Close(_ context.Context) error {
	if h == nil {
		return nil
	}

	var errs []error
	currentInfo, err := ReadInfo(h.paths.InfoPath)
	if err == nil {
		if sameInfoOwner(currentInfo, h.info) {
			if removeErr := RemoveInfo(h.paths.InfoPath); removeErr != nil {
				errs = append(errs, removeErr)
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		errs = append(errs, err)
	}

	if releaseErr := h.lock.Release(); releaseErr != nil {
		errs = append(errs, releaseErr)
	}

	h.lock = nil
	return errors.Join(errs...)
}

func (h *Host) cleanupStaleRuntime(ctx context.Context) error {
	info, err := ReadInfo(h.paths.InfoPath)
	switch {
	case err == nil:
		if sameInfoOwner(info, h.info) {
			return nil
		}
		if err := h.healthy(ctx, info); err == nil {
			return fmt.Errorf("daemon: healthy daemon info already exists for pid %d", info.PID)
		}
		if err := RemoveInfo(h.paths.InfoPath); err != nil {
			return err
		}
	case errors.Is(err, os.ErrNotExist):
		// Nothing to clean up.
	default:
		if removeErr := RemoveInfo(h.paths.InfoPath); removeErr != nil {
			return errors.Join(err, removeErr)
		}
	}

	if h.lock.StalePID() > 0 || shouldRemoveSocket(h.paths.SocketPath) {
		if err := removeSocketPath(h.paths.SocketPath); err != nil {
			return err
		}
	}

	return nil
}

func sameInfoOwner(a, b Info) bool {
	return a.PID > 0 && a.PID == b.PID && a.StartedAt.Equal(b.StartedAt)
}

func existingHealthyDaemonInfo(
	ctx context.Context,
	infoPath string,
	healthy func(context.Context, Info) error,
) (Info, bool) {
	info, err := ReadInfo(infoPath)
	if err != nil {
		return Info{}, false
	}
	if healthy(ctx, info) != nil {
		return Info{}, false
	}
	return info, true
}

func normalizeStartOptions(opts StartOptions) (StartOptions, error) {
	resolvedPaths, processAlive, healthy, err := normalizeProbeOptions(opts.HomePaths, ProbeOptions{
		ProcessAlive: opts.ProcessAlive,
		Healthy:      opts.Healthy,
	})
	if err != nil {
		return StartOptions{}, err
	}

	now := opts.Now
	if now == nil {
		now = time.Now
	}

	pid := opts.PID
	if pid <= 0 {
		pid = os.Getpid()
	}

	return StartOptions{
		HomePaths:    resolvedPaths,
		Version:      opts.Version,
		PID:          pid,
		HTTPPort:     normalizedDaemonHTTPPort(opts.HTTPPort),
		Now:          now,
		ProcessAlive: processAlive,
		Healthy:      healthy,
		Prepare:      opts.Prepare,
	}, nil
}

func normalizedDaemonHTTPPort(port int) int {
	switch port {
	case EphemeralHTTPPort:
		return 0
	case 0:
		return DefaultHTTPPort
	default:
		return port
	}
}

func normalizeProbeOptions(
	paths compozyconfig.HomePaths,
	opts ProbeOptions,
) (compozyconfig.HomePaths, func(int) bool, func(context.Context, Info) error, error) {
	resolvedPaths := paths
	if strings.TrimSpace(resolvedPaths.HomeDir) == "" {
		loadedPaths, err := resolveDaemonHomePaths()
		if err != nil {
			return compozyconfig.HomePaths{}, nil, nil, err
		}
		resolvedPaths = loadedPaths
	}

	processAlive := opts.ProcessAlive
	if processAlive == nil {
		processAlive = ProcessAlive
	}

	healthy := opts.Healthy
	if healthy == nil {
		healthy = func(_ context.Context, info Info) error {
			if err := info.Validate(); err != nil {
				return err
			}
			if info.State != ReadyStateReady {
				return fmt.Errorf("daemon: daemon is not ready (state=%s)", info.State)
			}
			if !processAlive(info.PID) {
				return fmt.Errorf("daemon: daemon pid %d is not running", info.PID)
			}
			return nil
		}
	}

	return resolvedPaths, processAlive, healthy, nil
}

func shouldRemoveSocket(path string) bool {
	return strings.TrimSpace(path) != ""
}

func removeSocketPath(path string) error {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return nil
	}

	info, err := os.Lstat(cleanPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("daemon: stat daemon socket %q: %w", cleanPath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("daemon: daemon socket path %q is a directory", cleanPath)
	}
	if err := os.Remove(cleanPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("daemon: remove stale daemon socket %q: %w", cleanPath, err)
	}
	return nil
}
