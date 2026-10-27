package procutil

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// OwnsFileLock verifies Linux kernel lock ownership independently of wall-clock drift.
func OwnsFileLock(pid int, path string) (_ bool, returnErr error) {
	if pid <= 0 || path == "" {
		return false, nil
	}
	var stat unix.Stat_t
	if err := unix.Stat(path, &stat); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("procutil: stat process lock: %w", err)
	}
	file, err := os.Open("/proc/locks")
	if err != nil {
		return false, fmt.Errorf("procutil: read kernel locks: %w", err)
	}
	defer func() { returnErr = errors.Join(returnErr, file.Close()) }()
	owned, err := ownsFileLock(file, pid, uint64(unix.Major(stat.Dev)), uint64(unix.Minor(stat.Dev)), stat.Ino)
	if err != nil {
		return false, fmt.Errorf("procutil: inspect kernel locks: %w", err)
	}
	return owned, nil
}
