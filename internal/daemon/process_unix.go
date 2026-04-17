//go:build !windows

package daemon

import (
	"errors"
	"syscall"
)

// ProcessAlive reports whether a process with pid is currently alive.
func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
