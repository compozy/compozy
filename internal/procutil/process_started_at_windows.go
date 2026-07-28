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
	defer windows.CloseHandle(handle)

	var createdAt windows.Filetime
	if err := windows.GetProcessTimes(handle, &createdAt, nil, nil, nil); err != nil {
		return time.Time{}, fmt.Errorf("procutil: read process %d times: %w", pid, err)
	}

	return time.Unix(0, createdAt.Nanoseconds()).UTC(), nil
}
