package journal

import (
	"testing"

	"github.com/compozy/compozy/pkg/compozy/events"
)

func TestIsTerminalEventJobParked(t *testing.T) {
	t.Parallel()

	t.Run("Should classify job.parked as terminal so it forces flush and fsync", func(t *testing.T) {
		t.Parallel()
		if !isTerminalEvent(events.EventKindJobParked) {
			t.Fatalf("isTerminalEvent(%q) = false, want true", events.EventKindJobParked)
		}
	})

	t.Run("Should keep existing run-terminal events terminal", func(t *testing.T) {
		t.Parallel()
		for _, kind := range []events.EventKind{
			events.EventKindRunCrashed,
			events.EventKindRunCompleted,
			events.EventKindRunFailed,
			events.EventKindRunCancelled,
		} {
			if !isTerminalEvent(kind) {
				t.Fatalf("isTerminalEvent(%q) = false, want true", kind)
			}
		}
	})

	t.Run("Should not classify non-terminal job events as terminal", func(t *testing.T) {
		t.Parallel()
		for _, kind := range []events.EventKind{
			events.EventKindJobStarted,
			events.EventKindJobStalled,
			events.EventKindJobRetryScheduled,
		} {
			if isTerminalEvent(kind) {
				t.Fatalf("isTerminalEvent(%q) = true, want false", kind)
			}
		}
	})
}
