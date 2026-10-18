//go:build !windows

package procutil

import (
	"errors"
	"fmt"
	"runtime"

	"golang.org/x/sys/unix"
)

// SetProcessGroupNiceness lowers the CPU scheduling priority of every process
// in the process group to at least niceness. Descendants forked afterward
// inherit the value. Unprivileged callers can only raise niceness, so a group
// already at or above niceness (for example under a niced daemon) is left
// unchanged instead of failing with a permission error.
func SetProcessGroupNiceness(pgid int, niceness int) error {
	if pgid <= 0 {
		return errors.New("set process group niceness: process group id must be positive")
	}
	current, err := ProcessGroupNiceness(pgid)
	if err != nil {
		return fmt.Errorf("set process group niceness (pgid %d, niceness %d): %w", pgid, niceness, err)
	}
	if current >= niceness {
		return nil
	}
	if err := unix.Setpriority(unix.PRIO_PGRP, pgid, niceness); err != nil {
		return fmt.Errorf("set process group niceness (pgid %d, niceness %d): %w", pgid, niceness, err)
	}
	return nil
}

// ProcessGroupNiceness reports the niceness of the least-niced member of the
// process group.
func ProcessGroupNiceness(pgid int) (int, error) {
	return getNiceness(unix.PRIO_PGRP, pgid)
}

// CurrentProcessNiceness reports the niceness of the calling process.
func CurrentProcessNiceness() (int, error) {
	return getNiceness(unix.PRIO_PROCESS, 0)
}

func getNiceness(which int, who int) (int, error) {
	priority, err := unix.Getpriority(which, who)
	if err != nil {
		return 0, err
	}
	if runtime.GOOS == "linux" {
		// The raw Linux syscall returns 20 - nice to avoid negative return values.
		return 20 - priority, nil
	}
	return priority, nil
}
