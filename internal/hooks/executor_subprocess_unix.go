//go:build !windows

package hooks

import (
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"github.com/compozy/agh/internal/procutil"
)

func configureSubprocessCommand(cmd *exec.Cmd) {
	procutil.ConfigureCommandProcessGroup(cmd)
}

func terminateSubprocessCommand(cmd *exec.Cmd) error {
	return signalSubprocessCommand(cmd, syscall.SIGTERM)
}

func killSubprocessCommand(cmd *exec.Cmd) error {
	return signalSubprocessCommand(cmd, syscall.SIGKILL)
}

func signalSubprocessCommand(cmd *exec.Cmd, sig syscall.Signal) error {
	if err := procutil.SignalCommandProcessGroup(cmd, sig); err != nil {
		return fmt.Errorf("kill process group (pid %d, sig %v): %w", cmd.Process.Pid, sig, err)
	}
	return nil
}

func forceSubprocessCommandExit(cmd *exec.Cmd, timeout time.Duration) error {
	return procutil.KillCommandProcessGroupAndWait(cmd, timeout)
}
