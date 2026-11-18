//go:build windows

package daemon

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows"
)

// stillActiveProcessExitCode is the value GetExitCodeProcess reports for a running process.
const stillActiveProcessExitCode = 259

// loopGateProcessReaped reports a pid that can no longer run. Windows rejects Signal(0), so the exit
// code is the evidence: an openable process that still reports the active code is alive.
func loopGateProcessReaped(t *testing.T, pid int) bool {
	t.Helper()
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		// A pid that cannot be opened no longer identifies a process worth waiting on.
		return errors.Is(err, windows.ERROR_INVALID_PARAMETER)
	}
	defer func() {
		if closeErr := windows.CloseHandle(handle); closeErr != nil {
			t.Errorf("close process handle for pid %d: %v", pid, closeErr)
		}
	}()
	var exitCode uint32
	if err := windows.GetExitCodeProcess(handle, &exitCode); err != nil {
		return false
	}
	return exitCode != stillActiveProcessExitCode
}
