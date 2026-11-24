package procutil

import (
	"os/exec"
	"runtime"
	"syscall"
)

func pinParentCommandThread(cmd *exec.Cmd) func() {
	// Linux delivers Pdeathsig on the death of the thread that spawned the child.
	runtime.LockOSThread()
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Pdeathsig = syscall.SIGTERM
	return runtime.UnlockOSThread
}
