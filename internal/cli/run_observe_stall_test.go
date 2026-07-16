package cli

import (
	"encoding/json"
	"testing"

	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func renderObservedPayload(t *testing.T, kind eventspkg.EventKind, payload any) string {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return renderObservedRunEvent(eventspkg.Event{Kind: kind, Payload: raw})
}

// TestRenderObservedTaskMultiSummaryLine pins the end-of-run summary line the
// walked-away user reads: a batch with one recovered and one parked child, and a
// clean batch that still closes with explicit zeroes.
func TestRenderObservedTaskMultiSummaryLine(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		payload kinds.TaskRunMultiplePayload
		want    string
	}{
		{
			name:    "Should render one recovered and one parked child",
			payload: kinds.TaskRunMultiplePayload{Total: 3, Completed: 2, Recovered: 1, Parked: 1},
			want:    "task queue summary | total=3 completed=2 recovered=1 parked=1\n",
		},
		{
			name:    "Should render zeroes for a batch with no stalls",
			payload: kinds.TaskRunMultiplePayload{Total: 2, Completed: 2},
			want:    "task queue summary | total=2 completed=2 recovered=0 parked=0\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := renderObservedPayload(t, eventspkg.EventKindTaskRunMultipleSummary, tc.payload)
			if got != tc.want {
				t.Fatalf("renderObservedRunEvent() = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("Should fall back when the summary payload is undecodable", func(t *testing.T) {
		t.Parallel()
		got := renderObservedRunEvent(eventspkg.Event{
			Kind:    eventspkg.EventKindTaskRunMultipleSummary,
			Payload: []byte("{"),
		})
		if got != "task queue summary\n" {
			t.Fatalf("renderObservedRunEvent() = %q, want %q", got, "task queue summary\n")
		}
	})
}

func TestRenderObservedStallJobEvents(t *testing.T) {
	t.Parallel()

	t.Run("Should render a stalled job with its last tool call", func(t *testing.T) {
		t.Parallel()

		got := renderObservedPayload(t, eventspkg.EventKindJobStalled, kinds.JobStalledPayload{
			JobAttemptInfo: kinds.JobAttemptInfo{Index: 0, Attempt: 1, MaxAttempts: 2},
			Reason:         "no output for 3m0s",
			LastToolCall:   "Bash go test ./...",
		})
		want := "job[1] stalled | no output for 3m0s | last tool call: Bash go test ./...\n"
		if got != want {
			t.Fatalf("renderObservedRunEvent() = %q, want %q", got, want)
		}
	})

	t.Run("Should render a parked job with its preserved worktree", func(t *testing.T) {
		t.Parallel()

		got := renderObservedPayload(t, eventspkg.EventKindJobParked, kinds.JobParkedPayload{
			JobAttemptInfo: kinds.JobAttemptInfo{Index: 1, Attempt: 2, MaxAttempts: 2},
			Reason:         "no output for 3m0s",
			WorktreePath:   "/tmp/wt/task_02",
		})
		want := "job[2] parked | no output for 3m0s | worktree=/tmp/wt/task_02\n"
		if got != want {
			t.Fatalf("renderObservedRunEvent() = %q, want %q", got, want)
		}
	})
}
