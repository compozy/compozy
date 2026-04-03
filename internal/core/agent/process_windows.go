//go:build windows

package agent

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

func configureACPCommand(cmd *exec.Cmd) error {
	if cmd == nil {
		return fmt.Errorf("missing ACP command")
	}
	return nil
}

func forceTerminateProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("kill ACP process %d: %w", cmd.Process.Pid, err)
	}
	return nil
}
