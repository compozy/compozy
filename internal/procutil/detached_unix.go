//go:build !windows

package procutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func spawnDetachedLoggedProcess(
	ctx context.Context,
	req DetachedLaunchRequest,
) (*DetachedProcess, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	binary, err := resolveLaunchBinary(req.Binary)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(req.LogPath), 0o755); err != nil {
		return nil, fmt.Errorf("procutil: create log directory for %q: %w", req.LogPath, err)
	}

	logFile, err := os.OpenFile(req.LogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("procutil: open log %q: %w", req.LogPath, err)
	}

	logInfo, err := logFile.Stat()
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("procutil: stat log %q: %w", req.LogPath, err),
			closeDetachedLaunchHandles(nil, logFile, req.LogPath),
		)
	}

	stdinFile, err := os.Open(os.DevNull)
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("procutil: open %q: %w", os.DevNull, err),
			closeDetachedLaunchHandles(nil, logFile, req.LogPath),
		)
	}

	if err := ctx.Err(); err != nil {
		return nil, errors.Join(err, closeDetachedLaunchHandles(stdinFile, logFile, req.LogPath))
	}

	process, err := startDetachedProcess(binary, launchArgv(binary, req.Args), &os.ProcAttr{
		Env:   launchSandbox(req.Sandbox),
		Files: []*os.File{stdinFile, logFile, logFile},
		Sys:   &syscall.SysProcAttr{Setpgid: true},
	})
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("procutil: spawn detached process: %w", err),
			closeDetachedLaunchHandles(stdinFile, logFile, req.LogPath),
		)
	}
	if err := closeDetachedLaunchHandles(stdinFile, logFile, req.LogPath); err != nil {
		return nil, errors.Join(err, cleanupStartedDetachedProcess(process))
	}

	return newDetachedProcess(process, req.LogPath, logInfo.Size()), nil
}

func cleanupStartedDetachedProcess(process *os.Process) error {
	if process == nil || process.Pid <= 0 {
		return nil
	}
	signalErr := SignalProcessGroupID(process.Pid, syscall.SIGKILL)
	_, waitErr := process.Wait()
	drainErr := WaitForProcessGroupIDExit(process.Pid, processGroupDrainDeadline)
	return joinProcessGroupKillResult(signalErr, errors.Join(waitErr, drainErr))
}
