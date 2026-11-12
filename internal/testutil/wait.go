package testutil

import (
	"testing"
	"time"
)

const (
	// eventuallyPollInterval bounds how long a satisfied condition can go
	// unobserved. It is not scaled: polling faster under -race only adds load.
	eventuallyPollInterval = 10 * time.Millisecond
	// deadlineReserve keeps a labeled wait failure ahead of the go test
	// -timeout panic, which reports no condition and kills the whole binary.
	deadlineReserve = 5 * time.Second
)

// TimeScale reports the multiplier applied to wall-clock test budgets. The
// race detector slows instrumented code several times over, so a budget sized
// for a plain build is multiplied under -race instead of being raised
// everywhere by hand.
func TimeScale() int {
	return raceTimeScale
}

// Scaled returns budget multiplied by TimeScale. A failing wait never outlives
// the test binary: when the running test has a deadline, the result is capped
// so the caller fails with its own label before go test aborts the package.
func Scaled(t testing.TB, budget time.Duration) time.Duration {
	t.Helper()

	scaled := budget * time.Duration(TimeScale())
	deadliner, ok := t.(interface{ Deadline() (time.Time, bool) })
	if !ok {
		return scaled
	}
	deadline, ok := deadliner.Deadline()
	if !ok {
		return scaled
	}
	remaining := time.Until(deadline) - deadlineReserve
	if remaining < budget {
		// Never shrink below the unscaled budget the caller asked for.
		return budget
	}
	return min(scaled, remaining)
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
