package observe

import "time"

const (
	freshnessStatusCurrent = "current"
	freshnessStatusEmpty   = "empty"
	freshnessStatusStale   = "stale"
)

// classifyTaskDashboardFreshness derives the shared dashboard/overview freshness
// status so both read models apply one empty/stale rule and their vocabulary
// cannot drift.
func classifyTaskDashboardFreshness(
	latest time.Time,
	hasLiveWork bool,
	age, staleAfter time.Duration,
) (status string, stale bool) {
	switch {
	case latest.IsZero():
		return freshnessStatusEmpty, false
	case hasLiveWork && staleAfter > 0 && age > staleAfter:
		return freshnessStatusStale, true
	default:
		return freshnessStatusCurrent, false
	}
}
