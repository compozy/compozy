package runshared

import (
	"testing"
	"time"
)

func TestActivityMonitorTreatsInFlightWorkAsActive(t *testing.T) {
	t.Parallel()

	monitor := &ActivityMonitor{lastActivity: time.Now().Add(-time.Hour)}

	monitor.BeginActivity()
	if got := monitor.TimeSinceLastActivity(); got != 0 {
		t.Fatalf("expected in-flight activity to report no inactivity, got %v", got)
	}

	monitor.EndActivity()
	if got := monitor.TimeSinceLastActivity(); got > time.Second {
		t.Fatalf("expected completed activity to refresh last activity, got %v", got)
	}
}
