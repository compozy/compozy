//go:build windows

package acp

import (
	"os/exec"
	"syscall"
	"time"

	"github.com/compozy/agh/internal/procutil"
)

func configureManagedCommand(cmd *exec.Cmd) {
	procutil.ConfigureCommandProcessGroup(cmd)
}

func registerManagedCommand(cmd *exec.Cmd) error {
	return procutil.RegisterCommandProcessGroup(cmd)
}

func terminateManagedProcess(cmd *exec.Cmd) error {
	return signalManagedProcess(cmd, syscall.SIGTERM)
}

func killManagedProcess(cmd *exec.Cmd) error {
	return signalManagedProcess(cmd, syscall.SIGKILL)
}

func signalManagedProcess(cmd *exec.Cmd, sig syscall.Signal) error {
	return procutil.SignalCommandProcessGroup(cmd, sig)
}

func forceManagedProcessGroupExit(cmd *exec.Cmd, timeout time.Duration) error {
	return procutil.KillCommandProcessGroupAndWait(cmd, timeout)
}
