package acp

import (
	"os/exec"
	"syscall"
	"time"

	"github.com/compozy/compozy/internal/procutil"
)

func terminateManagedProcess(cmd *exec.Cmd) error {
	return signalManagedProcess(cmd, syscall.SIGTERM)
}

func killManagedProcess(cmd *exec.Cmd) error {
	return signalManagedProcess(cmd, syscall.SIGKILL)
}

func signalManagedProcess(cmd *exec.Cmd, signal syscall.Signal) error {
	return procutil.SignalCommandProcessGroup(cmd, signal)
}

func forceManagedProcessGroupExit(cmd *exec.Cmd, timeout time.Duration) error {
	return procutil.KillCommandProcessGroupAndWait(cmd, timeout)
}
