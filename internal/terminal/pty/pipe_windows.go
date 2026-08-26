//go:build windows

package pty

import (
	"os/exec"
	"syscall"

	"github.com/compozy/compozy/internal/procutil"
)

func configurePipeCommand(command *exec.Cmd) { procutil.ConfigureCommandProcessGroup(command) }

func registerPipeCommand(command *exec.Cmd) error {
	return procutil.RegisterCommandProcessGroup(command)
}

func signalPipeCommand(command *exec.Cmd, _ Signal) error {
	// Windows has no POSIX signal delivery; every supported request maps to
	// terminating the supervised process group.
	return procutil.SignalCommandProcessGroup(command, syscall.SIGTERM)
}

func forcePipeCommand(command *exec.Cmd) error {
	return procutil.KillCommandProcessGroupAndWait(command, processGroupGrace)
}

func escalatePipeCommand(command *exec.Cmd) error { return forcePipeCommand(command) }

func reportedPipeSignal(requested Signal, exit Exit) Exit {
	if requested == "" {
		return exit
	}
	signal := string(requested)
	return Exit{Cause: "signaled", Signal: &signal}
}
