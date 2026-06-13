//go:build windows

package daemon

import "golang.org/x/sys/windows"

// ProcessAlive reports whether a process with pid is currently alive.
func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	handle, err := windows.OpenProcess(
		windows.SYNCHRONIZE,
		false,
		uint32(pid), //nolint:gosec // G115: pid validated as positive above
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
