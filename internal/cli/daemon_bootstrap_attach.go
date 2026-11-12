package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// bootstrapAttachReadyTimeout bounds how long bootstrap waits for a live lock owner that is
// still booting. Boot migrations on large databases can take minutes before the listener opens.
const bootstrapAttachReadyTimeout = 10 * time.Minute

// bootstrapLockOwnerBusy reports that the detached child lost the singleton lock to a live daemon.
type bootstrapLockOwnerBusy struct {
	pid   int
	cause error
}

func (e *bootstrapLockOwnerBusy) Error() string {
	return fmt.Sprintf("cli: CompozyOS daemon pid=%d owns the runtime lock: %v", e.pid, e.cause)
}

func (e *bootstrapLockOwnerBusy) Unwrap() error { return e.cause }

// classifyBootstrapLockOwner turns a failed child start into lock contention only when the daemon
// lock is actually held and its recorded holder is alive. A free lock (stale file, possibly with a
// recycled PID), an unrecorded holder, or a dead owner keeps the original launch failure, so those
// cases recover or fail through the normal start retries.
func classifyBootstrapLockOwner(
	deps commandDeps,
	homePaths compozyconfig.HomePaths,
	childPID int,
	startErr error,
) error {
	owner, err := deps.inspectDaemonLock(homePaths.DaemonLock)
	if err != nil {
		return errors.Join(startErr, fmt.Errorf("cli: inspect daemon lock owner: %w", err))
	}
	if !owner.Held || owner.PID <= 0 || owner.PID == childPID || !deps.processAlive(owner.PID) {
		return startErr
	}
	return &bootstrapLockOwnerBusy{pid: owner.PID, cause: startErr}
}

// attachBootingOwner waits for the live lock owner instead of launching competing daemons.
func (e *bootstrapExecution) attachBootingOwner(ownerPID int) error {
	if err := writeBootstrapEvent(e.cmd, bootstrapEvent{
		Type: bootstrapEventType, Phase: bootstrapPhaseAttach, Status: bootstrapStatusStarted,
		Resolution: bootstrapResolutionAttach,
		Message: fmt.Sprintf(
			"A CompozyOS runtime is already starting (pid %d); waiting for it to become ready.", ownerPID,
		),
	}); err != nil {
		return err
	}
	status, err := e.waitForLockOwnerReady(ownerPID)
	if err != nil {
		return writeBootstrapFailure(e.cmd, bootstrapPhaseAttach, bootstrapProbeUnavailable, err)
	}
	return e.attachRunning(status)
}

type bootstrapDaemonProbe struct {
	deps      commandDeps
	homePaths compozyconfig.HomePaths
	target    ClientTarget
	client    DaemonClient
}

// status reuses one transport while rechecking discovery identity on every observation.
func (p *bootstrapDaemonProbe) status(ctx context.Context) (DaemonStatus, bool, error) {
	info, running, err := daemonInfo(p.homePaths, p.deps)
	if err != nil || !running {
		return DaemonStatus{}, false, err
	}
	if p.client == nil {
		p.target, err = resolveLocalClientTarget(p.deps, p.homePaths)
		if err != nil {
			return DaemonStatus{}, false, &bootstrapConfigFailure{cause: err}
		}
		p.client, err = p.deps.newClient(p.target)
		if err != nil {
			return DaemonStatus{}, false, err
		}
	}
	status, err := p.client.DaemonStatus(ctx)
	if err != nil {
		return DaemonStatus{}, false, &bootstrapStatusFailure{cause: err}
	}
	if status.PID <= 0 || strings.TrimSpace(status.Status) != daemonRunningStatus {
		return status, false, &bootstrapStatusFailure{
			cause: errors.New("cli: running daemon did not report healthy status"),
		}
	}
	if status.PID != info.PID || !status.StartedAt.Equal(info.StartedAt) ||
		status.HTTPPort != info.Port || filepath.Clean(status.Socket) != filepath.Clean(p.target.socketPath) {
		return status, false, &bootstrapStatusFailure{
			cause: errors.New("cli: daemon status does not match the local discovery record"),
		}
	}
	return status, true, nil
}

// bootstrapStatusFailure marks a failed daemon status response: the recorded daemon was asked and
// did not answer healthy. Only this class is reported as "listening but unhealthy".
type bootstrapStatusFailure struct {
	cause error
}

func (e *bootstrapStatusFailure) Error() string { return e.cause.Error() }

func (e *bootstrapStatusFailure) Unwrap() error { return e.cause }

// waitForLockOwnerReady rechecks ownership because a holder may record its PID after taking the lock.
func (e *bootstrapExecution) waitForLockOwnerReady(ownerPID int) (DaemonStatus, error) {
	timeout := e.deps.bootstrapAttachTimeout
	ctx, cancel := context.WithTimeout(e.cmd.Context(), timeout)
	defer cancel()
	probe := bootstrapDaemonProbe{deps: e.deps, homePaths: e.homePaths}
	serving := bootstrapServingProbe{execution: e}
	ticker := time.NewTicker(e.deps.pollInterval)
	defer ticker.Stop()
	for ctx.Err() == nil {
		var lockHeld bool
		ownerPID, lockHeld = e.currentLockOwner(ownerPID)
		status, running, probeErr := probe.status(ctx)
		if ctx.Err() != nil {
			break
		}
		if probeErr == nil && running && daemonReadyAs(status, ownerPID) {
			return status, nil
		}
		if servingStatus, ok := serving.status(ctx, ownerPID); ok {
			return servingStatus, nil
		}
		if _, ok := errors.AsType[*bootstrapConfigFailure](probeErr); ok {
			return DaemonStatus{}, probeErr
		}
		// The holder may not have recorded its PID yet, so a held lock takes precedence over liveness.
		if !lockHeld && !e.deps.processAlive(ownerPID) {
			return DaemonStatus{}, fmt.Errorf(
				"cli: the running CompozyOS daemon pid=%d exited before it became ready; inspect the runtime log",
				ownerPID,
			)
		}
		select {
		case <-ctx.Done():
		case <-ticker.C:
		}
	}
	if e.cmd.Context().Err() != nil {
		return DaemonStatus{}, fmt.Errorf(
			"cli: stopped waiting for the running CompozyOS daemon pid=%d: %w", ownerPID, e.cmd.Context().Err(),
		)
	}
	return DaemonStatus{}, fmt.Errorf(
		"cli: the running CompozyOS daemon pid=%d did not become ready within %s; "+
			"it may still be starting, inspect the runtime log before retrying: %w",
		ownerPID, timeout, ctx.Err(),
	)
}

// currentLockOwner returns the PID recorded by the current lock holder and whether the lock is
// held. It keeps the last known owner when the lock is free, unrecorded, or unreadable; an
// unreadable lock reports not held so liveness decides the outcome.
func (e *bootstrapExecution) currentLockOwner(lastPID int) (int, bool) {
	owner, err := e.deps.inspectDaemonLock(e.homePaths.DaemonLock)
	if err != nil {
		return lastPID, false
	}
	if !owner.Held || owner.PID <= 0 {
		return lastPID, owner.Held
	}
	return owner.PID, true
}

// attachRunning attaches to a healthy daemon, replacing it first when it is an outdated desktop runtime.
func (e *bootstrapExecution) attachRunning(status DaemonStatus) error {
	replaced, err := e.replaceOutdatedDesktopRuntime(status)
	if err != nil {
		return writeBootstrapFailure(e.cmd, bootstrapPhaseProvision, bootstrapProbeListeningUnhealthy, err)
	}
	if !replaced {
		return writeAttachedBootstrap(e.cmd, e.options, status)
	}
	return e.start(bootstrapResolutionProvision, e.installedPath, true)
}
