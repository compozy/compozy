package daemon

import (
	"testing"

	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
)

func summaryEvent(kind eventspkg.EventKind) eventspkg.Event {
	return eventspkg.Event{SchemaVersion: eventspkg.SchemaVersion, Kind: kind}
}

// recordedChild builds one child's job lifecycle stream in emission order.
func recordedChild(kinds ...eventspkg.EventKind) []eventspkg.Event {
	evs := make([]eventspkg.Event, 0, len(kinds))
	for _, kind := range kinds {
		evs = append(evs, summaryEvent(kind))
	}
	return evs
}

func TestClassifyChildOutcome(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		evs  []eventspkg.Event
		want childOutcome
	}{
		{
			name: "Should classify a plain completion as completed, not recovered",
			evs:  recordedChild(eventspkg.EventKindJobCompleted),
			want: childOutcomeCompleted,
		},
		{
			name: "Should classify stalled then completed as recovered",
			evs:  recordedChild(eventspkg.EventKindJobStalled, eventspkg.EventKindJobCompleted),
			want: childOutcomeRecovered,
		},
		{
			name: "Should classify stalled twice then parked as parked",
			evs: recordedChild(
				eventspkg.EventKindJobStalled,
				eventspkg.EventKindJobStalled,
				eventspkg.EventKindJobParked,
			),
			want: childOutcomeParked,
		},
		{
			name: "Should let a park win over a completion recorded for another attempt",
			evs: recordedChild(
				eventspkg.EventKindJobCompleted,
				eventspkg.EventKindJobStalled,
				eventspkg.EventKindJobParked,
			),
			want: childOutcomeParked,
		},
		{
			name: "Should classify a stall with no terminal job event as neither completed nor parked",
			evs:  recordedChild(eventspkg.EventKindJobStalled),
			want: childOutcomeOther,
		},
		{
			name: "Should classify an empty stream as neither completed nor parked",
			evs:  nil,
			want: childOutcomeOther,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := classifyChildOutcome(tc.evs); got != tc.want {
				t.Fatalf("classifyChildOutcome() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestSummarizeChildOutcomesOverRecordedBatch feeds a recorded event stream for a
// batch with one recovered and one parked child and asserts the closing counts.
func TestSummarizeChildOutcomesOverRecordedBatch(t *testing.T) {
	t.Parallel()

	t.Run("Should count one recovered and one parked child", func(t *testing.T) {
		t.Parallel()

		summary := summarizeChildOutcomes(3, [][]eventspkg.Event{
			recordedChild(eventspkg.EventKindJobCompleted),
			recordedChild(eventspkg.EventKindJobStalled, eventspkg.EventKindJobCompleted),
			recordedChild(eventspkg.EventKindJobStalled, eventspkg.EventKindJobParked),
		})

		want := taskMultiRecoverySummary{Total: 3, Completed: 2, Recovered: 1, Parked: 1}
		if summary != want {
			t.Fatalf("summarizeChildOutcomes() = %+v, want %+v", summary, want)
		}
	})

	t.Run("Should report zero recovered and parked for a batch with no stalls", func(t *testing.T) {
		t.Parallel()

		summary := summarizeChildOutcomes(2, [][]eventspkg.Event{
			recordedChild(eventspkg.EventKindJobCompleted),
			recordedChild(eventspkg.EventKindJobCompleted),
		})

		want := taskMultiRecoverySummary{Total: 2, Completed: 2}
		if summary != want {
			t.Fatalf("summarizeChildOutcomes() = %+v, want %+v", summary, want)
		}
	})

	t.Run("Should keep total as the queue size when a child never started", func(t *testing.T) {
		t.Parallel()

		summary := summarizeChildOutcomes(2, [][]eventspkg.Event{
			recordedChild(eventspkg.EventKindJobCompleted),
		})

		want := taskMultiRecoverySummary{Total: 2, Completed: 1}
		if summary != want {
			t.Fatalf("summarizeChildOutcomes() = %+v, want %+v", summary, want)
		}
	})

	t.Run("Should not count a failed child as completed or parked", func(t *testing.T) {
		t.Parallel()

		summary := summarizeChildOutcomes(1, [][]eventspkg.Event{recordedChild()})

		want := taskMultiRecoverySummary{Total: 1}
		if summary != want {
			t.Fatalf("summarizeChildOutcomes() = %+v, want %+v", summary, want)
		}
	})
}
