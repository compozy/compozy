package cli

import (
	"encoding/json"
	"testing"

	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func mustObservedParkedEvent(t *testing.T, payload kinds.JobParkedPayload) eventspkg.Event {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal parked payload: %v", err)
	}
	return eventspkg.Event{Kind: eventspkg.EventKindJobParked, Payload: raw}
}

// `compozy runs watch` is where the walked-away user lands after the alert, so a
// parked line must carry everything needed to rerun or inspect the job.
func TestRenderObservedJobParkedCarriesFullTriageDetail(t *testing.T) {
	t.Parallel()

	event := mustObservedParkedEvent(t, kinds.JobParkedPayload{
		JobAttemptInfo:  kinds.JobAttemptInfo{Index: 1, Attempt: 2, MaxAttempts: 2},
		Reason:          "stalled again after clean-state retry",
		LastToolCall:    "Bash go test ./...",
		LastProgressSeq: 1284,
		WorktreePath:    "/tmp/wt/task_02",
		LogPath:         "/tmp/logs/task_02.out.log",
	})

	want := "job[2] parked | stalled again after clean-state retry | " +
		"last tool call: Bash go test ./... | last_progress_seq=1284 | " +
		"worktree=/tmp/wt/task_02 | log=/tmp/logs/task_02.out.log\n"
	if got := renderObservedRunEvent(event); got != want {
		t.Fatalf("renderObservedRunEvent()\n got = %q\nwant = %q", got, want)
	}
}

// Absent detail must never render a dangling key: a park with no durable progress
// and no preserved log still reads cleanly.
func TestRenderObservedJobParkedSkipsEmptyDetail(t *testing.T) {
	t.Parallel()

	event := mustObservedParkedEvent(t, kinds.JobParkedPayload{
		JobAttemptInfo: kinds.JobAttemptInfo{Index: 0},
		Reason:         "no output for 3m0s",
	})

	want := "job[1] parked | no output for 3m0s\n"
	if got := renderObservedRunEvent(event); got != want {
		t.Fatalf("renderObservedRunEvent()\n got = %q\nwant = %q", got, want)
	}
}

func TestRenderObservedJobParkedFallsBackOnUndecodablePayload(t *testing.T) {
	t.Parallel()

	got := renderObservedRunEvent(eventspkg.Event{
		Kind:    eventspkg.EventKindJobParked,
		Payload: []byte("{"),
	})
	if want := "job parked\n"; got != want {
		t.Fatalf("renderObservedRunEvent() = %q, want %q", got, want)
	}
}
