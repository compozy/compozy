package daemon

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"github.com/compozy/compozy/internal/procutil"
)

const (
	loopGateCommandShutdownGrace    = 5 * time.Second
	loopGateCommandOutputDrain      = 2 * time.Second
	loopGateCommandProcessGroupWait = 2 * time.Second
)

// loopGateCommandCleanupError reports that a gate command's process group could not be drained after
// the shell exited. It stays distinct from the process exit status so the runner never reports a
// cleanup failure as the command's exit code.
type loopGateCommandCleanupError struct {
	err error
}

func (e *loopGateCommandCleanupError) Error() string {
	return "daemon: clean gate command process group: " + e.err.Error()
}

func (e *loopGateCommandCleanupError) Unwrap() error { return e.err }

// runLoopGateCommandTree runs a gate command as its own process group. Cancellation sends SIGTERM
// to the whole group, escalates to SIGKILL after the grace period, and waits for the group to
// drain, so a command that ignores SIGTERM cannot leave descendants (or the containers they own)
// behind a canceled or superseded run. The drain bound makes the shell's exit observable on its own:
// a background descendant that still holds stdout or stderr cannot keep the gate "running" until it
// exits.
func runLoopGateCommandTree(ctx context.Context, cmd *exec.Cmd, grace time.Duration, drain time.Duration) error {
	if cause := context.Cause(ctx); cause != nil {
		return fmt.Errorf("daemon: gate command interrupted before start: %w", cause)
	}
	if grace <= 0 {
		grace = loopGateCommandShutdownGrace
	}
	if drain <= 0 {
		drain = loopGateCommandOutputDrain
	}
	cmd.WaitDelay = drain
	procutil.ConfigureCommandProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("daemon: start gate command: %w", err)
	}
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()
	if err := procutil.RegisterCommandProcessGroup(cmd); err != nil {
		return errors.Join(
			fmt.Errorf("daemon: register gate command process group: %w", err),
			stopLoopGateCommandTree(cmd, waitCh, grace),
		)
	}
	select {
	case waitErr := <-waitCh:
		// exec.ErrWaitDelay only reports that a descendant kept the captured streams open past the
		// drain bound, so the shell completed instead of the command failing.
		if errors.Is(waitErr, exec.ErrWaitDelay) {
			waitErr = nil
		}
		// The shell exited; reap any member it left running in the group.
		if groupErr := procutil.KillCommandProcessGroupAndWait(cmd, loopGateCommandProcessGroupWait); groupErr != nil {
			return errors.Join(waitErr, &loopGateCommandCleanupError{err: groupErr})
		}
		return waitErr
	case <-ctx.Done():
		return errors.Join(
			fmt.Errorf("daemon: gate command interrupted: %w", context.Cause(ctx)),
			stopLoopGateCommandTree(cmd, waitCh, grace),
		)
	}
}

func stopLoopGateCommandTree(cmd *exec.Cmd, waitCh <-chan error, grace time.Duration) error {
	deadline := time.Now().Add(grace)
	terminateErr := procutil.SignalCommandProcessGroup(cmd, syscall.SIGTERM)
	timer := time.NewTimer(grace)
	defer timer.Stop()
	shellExited := false
	select {
	case shellErr := <-waitCh:
		shellExited = true
		terminateErr = errors.Join(terminateErr, signalFreeShellError(shellErr))
	case <-timer.C:
		if err := procutil.SignalCommandProcessGroup(cmd, syscall.SIGKILL); err != nil {
			terminateErr = errors.Join(terminateErr, err)
		}
		terminateErr = errors.Join(terminateErr, signalFreeShellError(<-waitCh))
	}
	if shellExited {
		// The shell exiting is not proof the group is gone: a descendant that received SIGTERM keeps
		// the rest of the grace period to finish its own cleanup before it is forced.
		if err := procutil.WaitForCommandProcessGroupExit(cmd, time.Until(deadline)); err != nil {
			if killErr := procutil.SignalCommandProcessGroup(cmd, syscall.SIGKILL); killErr != nil {
				terminateErr = errors.Join(terminateErr, killErr)
			}
		}
	}
	// The whole group must drain completely, including descendants the shell left behind.
	return errors.Join(terminateErr, procutil.KillCommandProcessGroupAndWait(cmd, loopGateCommandProcessGroupWait))
}

// signalFreeShellError drops the shell's own signal exit status: the group drain above decides
// whether the tree is actually gone, and an ExitError here would mask the interruption cause.
func signalFreeShellError(err error) error {
	if _, ok := errors.AsType[*exec.ExitError](err); ok {
		return nil
	}
	return err
}
