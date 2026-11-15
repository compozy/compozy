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
	loopGateCommandProcessGroupWait = 2 * time.Second
)

// runLoopGateCommandTree runs a gate command as its own process group. Cancellation sends SIGTERM
// to the whole group, escalates to SIGKILL after the grace period, and waits for the group to
// drain, so a command that ignores SIGTERM cannot leave descendants (or the containers they own)
// behind a canceled or superseded run.
func runLoopGateCommandTree(ctx context.Context, cmd *exec.Cmd, grace time.Duration) error {
	if cause := context.Cause(ctx); cause != nil {
		return fmt.Errorf("daemon: gate command interrupted before start: %w", cause)
	}
	if grace <= 0 {
		grace = loopGateCommandShutdownGrace
	}
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
		// The shell exited; reap any member it left running in the group.
		if groupErr := procutil.KillCommandProcessGroupAndWait(cmd, loopGateCommandProcessGroupWait); groupErr != nil {
			return errors.Join(waitErr, fmt.Errorf("daemon: clean gate command process group: %w", groupErr))
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
	terminateErr := procutil.SignalCommandProcessGroup(cmd, syscall.SIGTERM)
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case shellErr := <-waitCh:
		terminateErr = errors.Join(terminateErr, signalFreeShellError(shellErr))
	case <-timer.C:
		if err := procutil.SignalCommandProcessGroup(cmd, syscall.SIGKILL); err != nil {
			terminateErr = errors.Join(terminateErr, err)
		}
		terminateErr = errors.Join(terminateErr, signalFreeShellError(<-waitCh))
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
