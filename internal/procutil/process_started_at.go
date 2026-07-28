package procutil

import "time"

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

	diff := observed.UTC().Sub(startedAt.UTC())
	if diff < 0 {
		diff = -diff
	}
	return diff <= processStartMatchTolerance
}
