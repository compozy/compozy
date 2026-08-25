//go:build windows

package pty

import (
	"os/exec"
	"syscall"
	"time"

	"github.com/compozy/compozy/internal/procutil"
)

func configurePipeCommand(command *exec.Cmd) { procutil.ConfigureCommandProcessGroup(command) }

func registerPipeCommand(command *exec.Cmd) error {
	return procutil.RegisterCommandProcessGroup(command)
}

func signalPipeCommand(command *exec.Cmd, _ Signal) error {
	return procutil.SignalCommandProcessGroup(command, syscall.SIGTERM)
}

func forcePipeCommand(command *exec.Cmd) error {
	return procutil.KillCommandProcessGroupAndWait(command, time.Second)
}

func escalatePipeCommand(command *exec.Cmd) error { return forcePipeCommand(command) }
