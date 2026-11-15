//go:build windows

package procutil

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// ConfigureCommandSession starts a command without inheriting or opening a console.
func ConfigureCommandSession(command *exec.Cmd) {
	if command == nil {
		return
	}
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.HideWindow = true
	command.SysProcAttr.CreationFlags |= windows.CREATE_NO_WINDOW
}
