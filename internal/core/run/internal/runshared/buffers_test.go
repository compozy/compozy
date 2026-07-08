package runshared

import (
	"testing"
	"time"
)

func TestActivityMonitorTreatsInFlightWorkAsActive(t *testing.T) {
	t.Parallel()

	monitor := &ActivityMonitor{lastActivity: time.Now().Add(-time.Hour)}

	t.Run("Should report in-flight work as active and refresh after completion", func(t *testing.T) {
		monitor.BeginActivity()
		if got := monitor.TimeSinceLastActivity(); got != 0 {
			t.Fatalf("expected in-flight activity to report no inactivity, got %v", got)
		}

		monitor.EndActivity()
		if got := monitor.TimeSinceLastActivity(); got > time.Second {
			t.Fatalf("expected completed activity to refresh last activity, got %v", got)
		}
	})
}

// stepClock is a minimal Clock whose Now advances only when the test sets it,
// exercising the injectable clock seam without real sleeps.
type stepClock struct {
	now time.Time
}

func (c *stepClock) Now() time.Time                 { return c.now }
func (c *stepClock) NewTicker(time.Duration) Ticker { return noopTicker{} }

type noopTicker struct{}

func (noopTicker) C() <-chan time.Time { return nil }
func (noopTicker) Stop()               {}

func TestActivityMonitorMeasuresIdleAgainstInjectedClock(t *testing.T) {
	t.Parallel()

	clk := &stepClock{now: time.Unix(0, 0)}
	monitor := NewActivityMonitorWithClock(clk)
	monitor.RecordActivity()

	t.Run("Should measure inactivity against the injected clock", func(t *testing.T) {
		clk.now = clk.now.Add(30 * time.Second)
		if got := monitor.TimeSinceLastActivity(); got != 30*time.Second {
			t.Fatalf("idle = %v, want 30s measured on the injected clock", got)
		}
	})

	t.Run("Should reset the idle window to the clock's now on any activity", func(t *testing.T) {
		monitor.RecordActivity()
		if got := monitor.TimeSinceLastActivity(); got != 0 {
			t.Fatalf("idle after reset = %v, want 0", got)
		}
	})
}
