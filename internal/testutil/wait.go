package testutil

import (
	"testing"
	"time"
)

// eventuallyPollInterval bounds how long a satisfied condition can go
// unobserved. It is not scaled: polling faster under -race only adds load.
const eventuallyPollInterval = 10 * time.Millisecond

// TimeScale reports the multiplier applied to wall-clock test budgets. The
// race detector slows instrumented code several times over, so a budget sized
// for a plain build is multiplied under -race instead of being raised
// everywhere by hand.
func TimeScale() int {
	return raceTimeScale
}

// Scaled returns budget multiplied by TimeScale. It deliberately ignores
// t.Deadline, which panics inside a testing/synctest bubble where Context and
// the waits below are also used.
func Scaled(t testing.TB, budget time.Duration) time.Duration {
	t.Helper()
	return budget * time.Duration(TimeScale())
}

// Eventually polls condition until it reports true and fails t with label once
// the race-scaled budget elapses. Prefer Receive when the code under test
// exposes a channel, watcher or event signal; poll only state that has none.
func Eventually(t testing.TB, label string, budget time.Duration, condition func() bool) {
	t.Helper()

	if eventually(Scaled(t, budget), condition) {
		return
	}
	t.Fatalf("timed out waiting for %s", label)
}

func eventually(timeout time.Duration, condition func() bool) bool {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(eventuallyPollInterval)
	defer ticker.Stop()

	for {
		if condition() {
			return true
		}
		select {
		case <-deadline.C:
			// One last observation so a condition satisfied while the final
			// tick was pending is not reported as a timeout.
			return condition()
		case <-ticker.C:
		}
	}
}

// Receive waits for one value from ch and fails t with label once the
// race-scaled budget elapses. A closed channel yields the zero value, so it
// also waits for done-style signals.
func Receive[T any](t testing.TB, label string, budget time.Duration, ch <-chan T) T {
	t.Helper()

	timer := time.NewTimer(Scaled(t, budget))
	defer timer.Stop()
	select {
	case value := <-ch:
		return value
	case <-timer.C:
		t.Fatalf("timed out waiting for %s", label)
		var zero T
		return zero
	}
}
