package cli

import (
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/daemon"
	"golang.org/x/sys/unix"
)

// killGuardianDaemonInstance force-kills the daemon recorded in info through a
// pidfd so the signal targets the exact process instance, never a recycled pid.
// It opens the pidfd first, confirms ownership via the daemon's endpoint, then
// signals through the pidfd: a pid recycled at any point yields ESRCH instead of
// terminating an unrelated process, closing the probe-then-kill race that a bare
// PID signal leaves open. Returns killed=false when the process is already gone
// or its endpoint cannot be confirmed as ours. Falls back to a bare-PID signal
// only on kernels without pidfd support (pre-5.3).
func killGuardianDaemonInstance(info daemon.Info) (bool, error) {
	pidfd, err := unix.PidfdOpen(info.PID, 0)
	if err != nil {
		switch {
		case errors.Is(err, unix.ESRCH):
			return false, nil
		case errors.Is(err, unix.ENOSYS):
			return killGuardianDaemonByPID(info)
		default:
			return false, fmt.Errorf("open pidfd for guardian daemon pid %d: %w", info.PID, err)
		}
	}
	defer func() { _ = unix.Close(pidfd) }()

	if !daemonEndpointReachable(info) {
		return false, nil
	}
	if err := unix.PidfdSendSignal(pidfd, unix.SIGKILL, nil, 0); err != nil {
		if errors.Is(err, unix.ESRCH) {
			return false, nil
		}
		return false, fmt.Errorf("pidfd kill guardian daemon pid %d: %w", info.PID, err)
	}
	return true, nil
}
