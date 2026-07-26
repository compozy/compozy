//go:build !windows

// Package procutil provides shared process helpers for AGH runtime components.
package procutil

import (
	"errors"
	"fmt"
	"syscall"
)

// Alive reports whether a process with the given PID is running.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}

	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// Signal sends sig to the process with the given PID.
func Signal(pid int, sig syscall.Signal) error {
	if pid <= 0 {
		return fmt.Errorf("procutil: invalid process pid %d", pid)
	}
	if err := syscall.Kill(pid, sig); err != nil {
		return fmt.Errorf("procutil: signal process %d with %s: %w", pid, sig.String(), err)
	}
	return nil
}
