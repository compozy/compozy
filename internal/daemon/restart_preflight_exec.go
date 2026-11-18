package daemon

import (
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"syscall"

	"github.com/compozy/compozy/internal/procutil"
)

// restartPreflightOutput keeps a bounded prefix of the replacement's standard output and records
// whether anything was dropped past it.
type restartPreflightOutput struct {
	data      []byte
	limit     int
	truncated bool
}

func (o *restartPreflightOutput) Write(chunk []byte) (int, error) {
	remaining := max(o.limit-len(o.data), 0)
	if len(chunk) > remaining {
		o.truncated = true
	}
	o.data = append(o.data, chunk[:min(len(chunk), remaining)]...)
	return len(chunk), nil
}

// defaultRestartPreflight runs the replacement's pre-flight command in the environment the
// replacement inherits, bounded by defaultRestartPreflightTimeout.
func defaultRestartPreflight(ctx context.Context, req restartPreflightRequest) restartPreflightOutcome {
	commandCtx, cancel := context.WithTimeout(ctx, defaultRestartPreflightTimeout)
	defer cancel()

	output := &restartPreflightOutput{limit: restartPreflightOutputLimit}
	// #nosec G204 -- the binary is this daemon's own install path running its fixed pre-flight command.
	command := exec.CommandContext(commandCtx, req.binary, req.args...)
	command.Cancel = func() error {
		return procutil.SignalCommandProcessGroup(command, syscall.SIGKILL)
	}
	command.WaitDelay = restartPreflightKillWait
	command.Env = append([]string(nil), req.sandbox...)
	command.Stdout = output
	procutil.ConfigureCommandProcessGroup(command)
	if err := command.Start(); err != nil {
		return restartPreflightOutcome{
			status: restartPreflightUnavailable,
			reason: "start replacement pre-flight: " + err.Error(),
		}
	}
	if err := procutil.RegisterCommandProcessGroup(command); err != nil {
		killErr := procutil.KillCommandProcessGroupAndWait(command, restartPreflightKillWait)
		waitErr := command.Wait()
		return restartPreflightOutcome{
			status: restartPreflightUnavailable,
			reason: "register replacement pre-flight process group: " + errors.Join(err, killErr, waitErr).Error(),
		}
	}
	waitErr := command.Wait()
	// Read before the cleanup below, which may outlast the deadline the replacement itself met.
	ctxErr := commandCtx.Err()
	// Wait does not release the process-group registration, so the bounded cleanup runs on every
	// path, success included. The report already decided the outcome; a cleanup failure is logged.
	if err := procutil.KillCommandProcessGroupAndWait(command, restartPreflightKillWait); err != nil {
		slog.WarnContext(ctx, "daemon: replacement pre-flight process group cleanup failed",
			"binary", req.binary, "error", err)
	}
	return classifyRestartPreflightOutput(output, waitErr, ctxErr)
}
