package procutil

import (
	"errors"
	"fmt"
	"syscall"
	"time"
)

// ErrProcessIdentityGone distinguishes a vanished process from unreadable identity data.
var ErrProcessIdentityGone = errors.New("procutil: process identity is gone")

// CreationProcessToken preserves timestamp identity when kernel identity is unavailable at creation.
// Existing tokenized records must still propagate lookup errors instead of downgrading identity.
func CreationProcessToken(pid int) string {
	token, err := ProcessToken(pid)
	if err != nil {
		return ""
	}
	return token
}

const processStartMatchTolerance = 2 * time.Second

// MatchesProcessIdentity prefers a persisted kernel identity when available.
func MatchesProcessIdentity(pid int, startedAt time.Time, token string) (bool, error) {
	return matchesProcessIdentity(pid, startedAt, token, ProcessToken, MatchesStartTime)
}

func matchesProcessIdentity(pid int, startedAt time.Time, token string,
	lookup func(int) (string, error), matchTime func(int, time.Time) bool,
) (bool, error) {
	if token == "" {
		return matchTime(pid, startedAt), nil
	}
	observed, err := lookup(pid)
	if errors.Is(err, ErrProcessIdentityGone) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return observed == token, nil
}

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
