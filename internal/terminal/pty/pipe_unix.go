//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package pty

import (
	"fmt"
	"os/exec"
	"syscall"

	"github.com/compozy/compozy/internal/procutil"
)

func configurePipeCommand(command *exec.Cmd) {
	procutil.ConfigureCommandSession(command)
}

func registerPipeCommand(*exec.Cmd) error { return nil }

func signalPipeCommand(command *exec.Cmd, signal Signal) error {
	resolved, err := unixSignal(signal)
	if err != nil {
		return err
	}
	return procutil.SignalCommandProcessGroup(command, resolved)
}

func forcePipeCommand(command *exec.Cmd) error {
	if command == nil || command.Process == nil {
		return nil
	}
	if err := procutil.SignalCommandProcessGroup(command, syscall.SIGTERM); err != nil {
		return fmt.Errorf("signal TERM: %w", err)
	}
	if err := procutil.WaitForCommandProcessGroupExit(command, processGroupGrace); err == nil {
		return nil
	}
	return procutil.KillCommandProcessGroupAndWait(command, processGroupGrace)
}

func escalatePipeCommand(command *exec.Cmd, beforeForce func()) error {
	if command == nil || command.Process == nil {
		return nil
	}
	if err := procutil.WaitForCommandProcessGroupExit(command, processGroupGrace); err == nil {
		return nil
	}
	if beforeForce != nil {
		beforeForce()
	}
	return procutil.KillCommandProcessGroupAndWait(command, processGroupGrace)
}

func reportedExitForSignal(requested Signal, exit Exit) Exit {
	if requested != SignalKILL {
		return exit
	}
	signal := string(SignalKILL)
	return Exit{Cause: "signaled", Signal: &signal}
}
