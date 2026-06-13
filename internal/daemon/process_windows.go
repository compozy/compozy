//go:build windows

package daemon

import (
	"math"

	"golang.org/x/sys/windows"
)

// ProcessAlive reports whether a process with pid is currently alive.
func ProcessAlive(pid int) bool {
	// Reject non-positive PIDs and any PID that cannot fit in the uint32 the
	// Windows API expects. uint64(pid) avoids the constant-overflow that
	// "pid > math.MaxUint32" would cause on 32-bit (windows/386) builds.
	if pid <= 0 || uint64(pid) > math.MaxUint32 {
		return false
	}

	handle, err := windows.OpenProcess(
		windows.SYNCHRONIZE,
		false,
		uint32(pid), //nolint:gosec // G115: pid validated as 0 < pid <= math.MaxUint32 above
	)
	if err != nil {
		return false
	}
	defer func() { _ = windows.CloseHandle(handle) }() //nolint:errcheck // handle cleanup; error is unactionable

	event, err := windows.WaitForSingleObject(handle, 0)
	if err != nil {
		return false
	}
	return event == uint32(windows.WAIT_TIMEOUT)
}
