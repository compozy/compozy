//go:build windows

package subprocess

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

func configureCommand(cmd *exec.Cmd) error {
	if cmd == nil {
		return fmt.Errorf("missing subprocess command")
	}
	return nil
}

func terminateProcess(cmd *exec.Cmd) error {
	return forceTerminateProcess(cmd)
}

func forceTerminateProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("kill subprocess %d: %w", cmd.Process.Pid, err)
	}
	return nil
}
