//go:build windows

package extensionpkg

import (
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"github.com/compozy/compozy/internal/procutil"
)

func configureBuildCommand(command *exec.Cmd) {
	procutil.ConfigureCommandProcessGroup(command)
}

func terminateBuildCommand(command *exec.Cmd) error {
	return signalBuildCommand(command, syscall.SIGTERM)
}

func killBuildCommand(command *exec.Cmd) error {
	return signalBuildCommand(command, syscall.SIGKILL)
}

func signalBuildCommand(command *exec.Cmd, signal syscall.Signal) error {
	if err := procutil.SignalCommandProcessGroup(command, signal); err != nil {
		return fmt.Errorf("extension: signal build process group with %v: %w", signal, err)
	}
	return nil
}

func forceBuildCommandExit(command *exec.Cmd, timeout time.Duration) error {
	return procutil.KillCommandProcessGroupAndWait(command, timeout)
}
