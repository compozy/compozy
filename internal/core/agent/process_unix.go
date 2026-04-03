//go:build !windows

package agent

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func configureACPCommand(cmd *exec.Cmd) error {
	if cmd == nil {
		return fmt.Errorf("missing ACP command")
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return nil
}

func forceTerminateProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil && pgid > 0 {
		if killErr := syscall.Kill(-pgid, syscall.SIGKILL); killErr == nil || killErr == syscall.ESRCH {
			return nil
		}
	}
	if err := cmd.Process.Kill(); err != nil && err != os.ErrProcessDone {
		return err
	}
	return nil
}
