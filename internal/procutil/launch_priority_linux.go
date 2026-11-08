package procutil

import (
	"runtime"

	"golang.org/x/sys/unix"
)

// StartWithNiceness starts on a disposable Linux thread so the child inherits priority before exec.
func StartWithNiceness(niceness int, start func() error) (priorityErr, startErr error) {
	if niceness <= 0 {
		return nil, start()
	}
	type result struct{ priorityErr, startErr error }
	done := make(chan result)
	go func() {
		runtime.LockOSThread()
		// Do not unlock: Go must discard this thread rather than reuse its lowered priority.
		current, err := CurrentProcessNiceness()
		if err == nil && current < niceness {
			err = unix.Setpriority(unix.PRIO_PROCESS, unix.Gettid(), niceness)
		}
		done <- result{priorityErr: err, startErr: start()}
	}()
	outcome := <-done
	return outcome.priorityErr, outcome.startErr
}
