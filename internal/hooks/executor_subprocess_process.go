package hooks

import (
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"github.com/compozy/compozy/internal/procutil"
)

func configureSubprocessCommand(cmd *exec.Cmd) {
	procutil.ConfigureCommandProcessGroup(cmd)
}

func terminateSubprocessCommand(cmd *exec.Cmd) error {
	if err := procutil.SignalCommandProcessGroup(cmd, syscall.SIGTERM); err != nil {
		return fmt.Errorf(
			"signal process group (pid %d, sig %v): %w",
			cmd.Process.Pid,
			syscall.SIGTERM,
			err,
		)
	}
	return nil
}

func forceSubprocessCommandExit(cmd *exec.Cmd, timeout time.Duration) error {
	return procutil.KillCommandProcessGroupAndWait(cmd, timeout)
}
