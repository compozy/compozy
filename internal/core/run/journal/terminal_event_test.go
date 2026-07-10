package journal

import (
	"testing"

	"github.com/compozy/compozy/pkg/compozy/events"
)

func TestIsDurabilityCriticalEventJobParked(t *testing.T) {
	t.Parallel()

	t.Run("Should classify job.parked as durability-critical so it forces flush and fsync", func(t *testing.T) {
		t.Parallel()
		if !isDurabilityCriticalEvent(events.EventKindJobParked) {
			t.Fatalf("isDurabilityCriticalEvent(%q) = false, want true", events.EventKindJobParked)
		}
	})

	t.Run("Should keep existing run-terminal events durability-critical", func(t *testing.T) {
		t.Parallel()
		for _, kind := range []events.EventKind{
			events.EventKindRunCrashed,
			events.EventKindRunCompleted,
			events.EventKindRunFailed,
			events.EventKindRunCancelled,
		} {
			if !isDurabilityCriticalEvent(kind) {
				t.Fatalf("isDurabilityCriticalEvent(%q) = false, want true", kind)
			}
		}
	})

	t.Run("Should not classify non-terminal job events as durability-critical", func(t *testing.T) {
		t.Parallel()
		for _, kind := range []events.EventKind{
			events.EventKindJobStarted,
			events.EventKindJobStalled,
			events.EventKindJobRetryScheduled,
		} {
			if isDurabilityCriticalEvent(kind) {
				t.Fatalf("isDurabilityCriticalEvent(%q) = true, want false", kind)
			}
		}
	})
}
