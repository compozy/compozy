package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/compozy/compozy/internal/procutil"
)

var errLoopGateSupervisorExited = errors.New("gate supervisor exited without authored command completion")

var errLoopGateSupervisorUnjoined = errors.New("daemon: timed out joining gate supervisor")

const (
	loopGateCommandShutdownGrace    = 5 * time.Second
	loopGateCommandOutputDrain      = 2 * time.Second
	loopGateCommandProcessGroupWait = 2 * time.Second
)

// loopGateCommandCleanupError reports that a gate command's process group could not be drained after
// the shell exited. It stays distinct from the process exit status so the runner never reports a
// cleanup failure as the command's exit code.
type loopGateCommandCleanupError struct {
	err      error
	exitCode *int
}

func (e *loopGateCommandCleanupError) Error() string {
	return "daemon: clean gate command process group: " + e.err.Error()
}

func (e *loopGateCommandCleanupError) Unwrap() error { return e.err }

// runLoopGateCommandTree runs a gate command as its own process group. Cancellation sends SIGTERM
// to the whole group, escalates to SIGKILL after the grace period, and waits for the group to
// drain, so a command that ignores SIGTERM cannot leave its local process group
// behind a canceled or superseded run. Daemon-owned and remote containers require the
// external resource contract tracked in #1743. The drain bound makes the shell's exit observable on its own:
// a background descendant that still holds stdout or stderr cannot keep the gate "running" until it
// exits.
func runLoopGateCommandTree(
	ctx context.Context, cmd *exec.Cmd, grace time.Duration, drain time.Duration, onStart func() error,
) error {
	if cause := context.Cause(ctx); cause != nil {
		return fmt.Errorf("daemon: gate command interrupted before start: %w", cause)
	}
	if grace <= 0 {
		grace = loopGateCommandShutdownGrace
	}
	if drain <= 0 {
		drain = loopGateCommandOutputDrain
	}
	statusPath, cleanup, err := prepareLoopGateCommand(ctx, cmd)
	if err != nil {
		return err
	}
	defer cleanup()
	cmd.WaitDelay = drain
	procutil.ConfigureCommandProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("daemon: start gate command: %w", err)
	}
	// Observe and checkpoint the leader before Wait can reap it. A short-lived
	// shell may otherwise vanish before its start time can prove group ownership.
	registerErr := procutil.RegisterCommandProcessGroup(cmd)
	if registerErr == nil && onStart != nil {
		registerErr = onStart()
	}
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()
	if registerErr != nil {
		return errors.Join(
			fmt.Errorf("daemon: register gate command process: %w", registerErr),
			stopLoopGateCommandTree(cmd, waitCh, grace),
		)
	}
	return waitLoopGateCommandTree(ctx, cmd, waitCh, statusPath, grace)
}

func waitLoopGateCommandTree(
	ctx context.Context, cmd *exec.Cmd, waitCh <-chan error, statusPath string, grace time.Duration,
) error {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case waitErr := <-waitCh:
			groupErr := procutil.KillCommandProcessGroupAndWait(cmd, loopGateCommandProcessGroupWait)
			code, ready, statusErr := loopGateCommandExit(statusPath)
			if !ready && statusErr == nil {
				statusErr = fmt.Errorf("daemon: %w", errors.Join(errLoopGateSupervisorExited, waitErr))
			}
			return loopGateCommandResult(code, statusErr, groupErr)
		case <-ctx.Done():
			return errors.Join(
				fmt.Errorf("daemon: gate command interrupted: %w", context.Cause(ctx)),
				stopLoopGateCommandTree(cmd, waitCh, grace),
			)
		case <-ticker.C:
			code, ready, statusErr := loopGateCommandExit(statusPath)
			if !ready && statusErr == nil {
				continue
			}
			// Keep the verifiable group leader alive until every descendant has
			// received its final signal, even if the authored shell already exited.
			groupErr := killLoopGateSupervisor(cmd, waitCh)
			if groupErr != nil {
				return loopGateCommandResult(code, statusErr, groupErr)
			}
			if cause := context.Cause(ctx); cause != nil {
				return fmt.Errorf("daemon: gate command interrupted: %w", cause)
			}
			return loopGateCommandResult(code, statusErr, nil)
		}
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
		terminateErr = errors.Join(terminateErr, killLoopGateSupervisor(cmd, waitCh))
	}
	if shellExited {
		// Unexpected supervisor exit is not proof the group is gone: a descendant that received SIGTERM keeps
		// the rest of the grace period to finish its own cleanup before it is forced.
		if err := procutil.WaitForCommandProcessGroupExit(cmd, time.Until(deadline)); err != nil {
			if killErr := procutil.SignalCommandProcessGroup(cmd, syscall.SIGKILL); killErr != nil {
				terminateErr = errors.Join(terminateErr, killErr)
			}
		}
	}
	// The whole group must drain completely, including descendants the shell left behind.
	groupErr := procutil.KillCommandProcessGroupAndWait(cmd, loopGateCommandProcessGroupWait)
	if err := errors.Join(terminateErr, groupErr); err != nil {
		return &loopGateCommandCleanupError{err: err}
	}
	return nil
}

// killLoopGateSupervisor bounds joining even when a group member refuses a signal.
// A failed group signal must not prevent killing the daemon-owned supervisor PID.
func killLoopGateSupervisor(cmd *exec.Cmd, waitCh <-chan error) error {
	groupErr := procutil.SignalCommandProcessGroup(cmd, syscall.SIGKILL)
	if groupErr != nil {
		err := procutil.Signal(cmd.Process.Pid, syscall.SIGKILL)
		if err != nil && !procutil.IsProcessMissingError(err) {
			groupErr = errors.Join(groupErr, fmt.Errorf("daemon: kill gate supervisor: %w", err))
		}
	}
	timer := time.NewTimer(cmd.WaitDelay + loopGateCommandProcessGroupWait)
	defer timer.Stop()
	select {
	case waitErr := <-waitCh:
		groupErr = errors.Join(groupErr, signalFreeShellError(waitErr))
	case <-timer.C:
		groupErr = errors.Join(groupErr, errLoopGateSupervisorUnjoined)
	}
	return errors.Join(groupErr, procutil.KillCommandProcessGroupAndWait(cmd, loopGateCommandProcessGroupWait))
}

func loopGateCommandResult(code int, statusErr, cleanupErr error) error {
	if cleanupErr != nil {
		cleanup := &loopGateCommandCleanupError{err: cleanupErr}
		if statusErr == nil {
			cleanup.exitCode = new(code)
		}
		return errors.Join(statusErr, cleanup)
	}
	if statusErr != nil {
		return statusErr
	}
	if code != 0 {
		return &loopGateCommandExitError{code: code}
	}
	return nil
}

// signalFreeShellError drops the supervisor's signal status and bounded output-drain
// result. Group cleanup proves ownership cleanup; an escaped pipe holder cannot
// replace the authored status or the execution's interruption cause.
func signalFreeShellError(err error) error {
	if errors.Is(err, exec.ErrWaitDelay) {
		return nil
	}
	if _, ok := errors.AsType[*exec.ExitError](err); ok {
		return nil
	}
	return err
}

// The leader must survive the authored shell's exit: a reaped shell cannot prove
// ownership of a surviving group after a daemon crash. Arguments keep authored
// code separate from this fixed supervisor protocol. A caught (rather than
// ignored) TERM resets to the default in the child shell.
const loopGateSupervisorScript = `trap ':' TERM
sh -c "$1"
code=$?
printf '%s\n' "$code" > "$2.tmp" && mv "$2.tmp" "$2" || exit 125
while :; do sleep 3600 & wait $!; done
`

func prepareLoopGateCommand(ctx context.Context, cmd *exec.Cmd) (string, func(), error) {
	controlDir, err := os.MkdirTemp("", "compozy-loop-gate-*")
	if err != nil {
		return "", nil, fmt.Errorf("daemon: create gate completion directory: %w", err)
	}
	cleanup := func() {
		if err := os.RemoveAll(controlDir); err != nil {
			slog.WarnContext(ctx, "remove gate completion directory", "error", err)
		}
	}
	statusPath := filepath.Join(controlDir, "exit")
	cmd.Args = []string{"sh", "-c", loopGateSupervisorScript, "compozy-loop-gate", cmd.Args[2], statusPath}
	return statusPath, cleanup, nil
}

type loopGateCommandExitError struct{ code int }

func (e *loopGateCommandExitError) Error() string {
	return fmt.Sprintf("daemon: gate command exited with status %d", e.code)
}

func loopGateCommandExit(path string) (int, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("daemon: read gate completion: %w", err)
	}
	code, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, false, fmt.Errorf("daemon: decode gate exit status: %w", err)
	}
	return code, true, nil
}
