package cli

import (
	"errors"
	"fmt"
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

// waitForLockOwnerReady polls the local daemon until ownerPID reports running.
// The deadline is the single extension seam: a future boot-progress signal can push it out
// while the owner keeps reporting progress, without changing the readiness predicate.
func (e *bootstrapExecution) waitForLockOwnerReady(ownerPID int) (DaemonStatus, error) {
	ctx := e.cmd.Context()
	timeout := e.deps.bootstrapAttachTimeout
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(e.deps.pollInterval)
	defer ticker.Stop()
	for {
		// Probe errors are expected while the owner boots without a listener; liveness and the
		// deadline decide failure.
		status, running, probeErr := probeBootstrapDaemon(ctx, e.deps, e.homePaths)
		if probeErr == nil && running && daemonReadyAs(status, ownerPID) {
			return status, nil
		}
		if !e.deps.processAlive(ownerPID) {
			return DaemonStatus{}, fmt.Errorf(
				"cli: the running CompozyOS daemon pid=%d exited before it became ready; inspect the runtime log",
				ownerPID,
			)
		}
		if !time.Now().Before(deadline) {
			return DaemonStatus{}, fmt.Errorf(
				"cli: the running CompozyOS daemon pid=%d did not become ready within %s; "+
					"it may still be starting, inspect the runtime log before retrying",
				ownerPID,
				timeout,
			)
		}
		select {
		case <-ctx.Done():
			return DaemonStatus{}, fmt.Errorf(
				"cli: stopped waiting for the running CompozyOS daemon pid=%d: %w", ownerPID, ctx.Err(),
			)
		case <-ticker.C:
		}
	}
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
