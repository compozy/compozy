package procutil

import (
	"errors"
	"fmt"
	"syscall"
	"time"
)

const processStartMatchTolerance = 2 * time.Second

// MatchesStartTime reports whether pid currently belongs to a process whose
// observed start time matches the recorded value closely enough to account for
// launcher-vs-kernel timestamp skew.
func MatchesStartTime(pid int, startedAt time.Time) bool {
	if pid <= 0 || startedAt.IsZero() {
		return false
	}

	observed, err := StartedAt(pid)
	if err != nil {
		return false
	}

	return MatchesObservedStartTime(observed, startedAt)
}

// MatchesObservedStartTime compares persisted and kernel timestamps with launcher skew tolerance.
func MatchesObservedStartTime(observed, startedAt time.Time) bool {
	diff := observed.UTC().Sub(startedAt.UTC())
	if diff < 0 {
		diff = -diff
	}
	return diff <= processStartMatchTolerance
}

// VerifyProcessExit checks whether the recorded process identity is gone.
// A reused PID proves the original process exited; a failed lookup does not.
func VerifyProcessExit(pid int, startedAt time.Time) (bool, error) {
	return verifyProcessExit(pid, startedAt, Signal, StartedAt)
}

func verifyProcessExit(
	pid int,
	startedAt time.Time,
	signal func(int, syscall.Signal) error,
	lookup func(int) (time.Time, error),
) (bool, error) {
	if pid <= 0 || startedAt.IsZero() {
		return false, errors.New("procutil: process exit verification requires PID and start time")
	}
	if err := signal(pid, 0); err != nil {
		if IsProcessMissingError(err) {
			return true, nil
		}
		return false, fmt.Errorf("procutil: probe process %d: %w", pid, err)
	}
	observed, err := lookup(pid)
	if err != nil {
		// Exit can race the start-time query. Only a positive missing-process
		// result from the second probe turns that lookup failure into proof.
		if probeErr := signal(pid, 0); IsProcessMissingError(probeErr) {
			return true, nil
		}
		return false, fmt.Errorf("procutil: verify process %d identity: %w", pid, err)
	}
	return !MatchesObservedStartTime(observed, startedAt), nil
}
