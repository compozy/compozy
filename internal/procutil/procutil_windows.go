//go:build windows

// Package procutil provides shared process helpers for AGH runtime components.
package procutil

import (
	"errors"
	"fmt"
	"syscall"
)

const (
	windowsAliveAccess     = syscall.SYNCHRONIZE | syscall.PROCESS_QUERY_INFORMATION
	windowsTerminateAccess = syscall.SYNCHRONIZE | syscall.PROCESS_TERMINATE
)

// Alive reports whether a process with the given PID is running.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}

	handle, err := syscall.OpenProcess(windowsAliveAccess, false, uint32(pid))
	if err != nil {
		return errors.Is(err, syscall.ERROR_ACCESS_DENIED)
	}
	defer syscall.CloseHandle(handle)

	state, waitErr := syscall.WaitForSingleObject(handle, 0)
	if waitErr != nil {
		return false
	}
	return state == syscall.WAIT_TIMEOUT
}

// Signal sends sig to the process with the given PID.
func Signal(pid int, sig syscall.Signal) error {
	if pid <= 0 {
		return fmt.Errorf("procutil: invalid process pid %d", pid)
	}

	if sig == 0 {
		return signalZero(pid, sig)
	}

	handle, err := syscall.OpenProcess(windowsTerminateAccess, false, uint32(pid))
	if err != nil {
		return fmt.Errorf("procutil: signal process %d with %s: %w", pid, sig.String(), err)
	}
	defer syscall.CloseHandle(handle)

	switch sig {
	case syscall.SIGTERM, syscall.SIGKILL:
		if err := syscall.TerminateProcess(handle, 1); err != nil {
			return fmt.Errorf("procutil: signal process %d with %s: %w", pid, sig.String(), err)
		}
		return nil
	default:
		return fmt.Errorf("procutil: signal process %d with %s: unsupported signal on windows", pid, sig.String())
	}
}

func signalZero(pid int, sig syscall.Signal) error {
	handle, err := syscall.OpenProcess(windowsAliveAccess, false, uint32(pid))
	if err != nil {
		if errors.Is(err, syscall.ERROR_ACCESS_DENIED) {
			return nil
		}
		return fmt.Errorf("procutil: signal process %d with %s: %w", pid, sig.String(), err)
	}
	defer syscall.CloseHandle(handle)

	state, waitErr := syscall.WaitForSingleObject(handle, 0)
	if waitErr != nil {
		return fmt.Errorf("procutil: signal process %d with %s: %w", pid, sig.String(), waitErr)
	}
	if state != syscall.WAIT_TIMEOUT {
		return fmt.Errorf("procutil: signal process %d with %s: process is not running", pid, sig.String())
	}
	return nil
}
