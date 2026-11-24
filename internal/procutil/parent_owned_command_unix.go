//go:build !windows

package procutil

import "os/exec"

func startParentOwnedCommand(cmd *exec.Cmd) error {
	ConfigureCommandProcessGroup(cmd)
	return cmd.Start()
}

func releaseParentOwnedCommand(*exec.Cmd) error { return nil }
