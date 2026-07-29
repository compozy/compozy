//go:build windows

package procutil

import (
	"fmt"
	"time"

	"golang.org/x/sys/windows"
)

// StartedAt reports the observed start time for pid using the Windows process table.
func StartedAt(pid int) (time.Time, error) {
	if pid <= 0 {
		return time.Time{}, fmt.Errorf("procutil: invalid process pid %d", pid)
	}

	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return time.Time{}, fmt.Errorf("procutil: open process %d: %w", pid, err)
	}
	if handle == 0 || handle == windows.InvalidHandle {
		return time.Time{}, fmt.Errorf("procutil: open process %d: invalid handle", pid)
	}
	defer windows.CloseHandle(handle)

	// GetProcessTimes crashes with ACCESS_VIOLATION when passed nil pointers
	// for the exit/kernel/user time outputs on some Windows builds. Allocate
	// discard buffers so the syscall receives four valid Filetime pointers.
	var createdAt, exitAt, kernelTime, userTime windows.Filetime
	if err := windows.GetProcessTimes(handle, &createdAt, &exitAt, &kernelTime, &userTime); err != nil {
		return time.Time{}, fmt.Errorf("procutil: read process %d times: %w", pid, err)
	}

	return time.Unix(0, createdAt.Nanoseconds()).UTC(), nil
}
