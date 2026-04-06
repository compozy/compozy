package runs

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events"
)

func TestTailReplaysHistoricalEventsBeforeLiveEvents(t *testing.T) {
	workspaceRoot := t.TempDir()
	runID := "run-tail"
	runDir := writeRunFixture(t, workspaceRoot, runID, runFixture{
		runJSON: map[string]any{
			"run_id":         runID,
			"mode":           "exec",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
		},
		events: []events.Event{
			testEvent(runID, 1, events.EventKindRunStarted),
			testEvent(runID, 2, events.EventKindJobStarted),
			testEvent(runID, 3, events.EventKindJobCompleted),
		},
	})

	run, err := Open(workspaceRoot, runID)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventsCh, errsCh := run.Tail(ctx, 2)
	appendEvents(t, filepath.Join(runDir, "events.jsonl"), []events.Event{
		testEvent(runID, 4, events.EventKindUsageUpdated),
		testEvent(runID, 5, events.EventKindRunCompleted),
	})

	got := collectTailEvents(t, eventsCh, errsCh, 4, 2*time.Second)
	if seqs := collectedSeqs(got); !slices.Equal(seqs, []uint64{2, 3, 4, 5}) {
		t.Fatalf("Tail() seqs = %v, want [2 3 4 5]", seqs)
	}
}

func TestTailCancelsCleanly(t *testing.T) {
	workspaceRoot := t.TempDir()
	runID := "run-tail-cancel"
	writeRunFixture(t, workspaceRoot, runID, runFixture{
		runJSON: map[string]any{
			"run_id":         runID,
			"mode":           "exec",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
		},
		events: []events.Event{
			testEvent(runID, 1, events.EventKindRunStarted),
		},
	})

	run, err := Open(workspaceRoot, runID)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	eventsCh, errsCh := run.Tail(ctx, 0)
	got := collectTailEvents(t, eventsCh, errsCh, 1, time.Second)
	if seqs := collectedSeqs(got); !slices.Equal(seqs, []uint64{1}) {
		t.Fatalf("Tail() seqs = %v, want [1]", seqs)
	}

	cancelAndAwaitClose(t, cancel, eventsCh, errsCh)
}

func TestTailLiveFollowIntegration(t *testing.T) {
	workspaceRoot := t.TempDir()
	runID := "run-tail-integration"
	runDir := writeRunFixture(t, workspaceRoot, runID, runFixture{
		runJSON: map[string]any{
			"run_id":         runID,
			"mode":           "exec",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
		},
		events: []events.Event{
			testEvent(runID, 1, events.EventKindRunStarted),
			testEvent(runID, 2, events.EventKindJobStarted),
			testEvent(runID, 3, events.EventKindJobCompleted),
			testEvent(runID, 4, events.EventKindUsageUpdated),
			testEvent(runID, 5, events.EventKindUsageUpdated),
			testEvent(runID, 6, events.EventKindUsageUpdated),
			testEvent(runID, 7, events.EventKindUsageUpdated),
			testEvent(runID, 8, events.EventKindUsageUpdated),
			testEvent(runID, 9, events.EventKindUsageUpdated),
			testEvent(runID, 10, events.EventKindUsageUpdated),
		},
	})

	run, err := Open(workspaceRoot, runID)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventsCh, errsCh := run.Tail(ctx, 11)
	liveEvents := make([]events.Event, 0, 50)
	for seq := uint64(11); seq <= 60; seq++ {
		liveEvents = append(liveEvents, testEvent(runID, seq, events.EventKindUsageUpdated))
	}
	appendEvents(t, filepath.Join(runDir, "events.jsonl"), liveEvents)

	got := collectTailEvents(t, eventsCh, errsCh, len(liveEvents), 3*time.Second)
	if seqs := collectedSeqs(got); !slices.Equal(seqs, collectedSeqs(liveEvents)) {
		t.Fatalf("Tail() live seqs = %v, want %v", seqs, collectedSeqs(liveEvents))
	}
}

func TestTailStopsWhenContextIsAlreadyCanceled(t *testing.T) {
	workspaceRoot := t.TempDir()
	runID := "run-tail-canceled-context"
	writeRunFixture(t, workspaceRoot, runID, runFixture{
		runJSON: map[string]any{
			"run_id":         runID,
			"mode":           "exec",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
		},
		events: []events.Event{
			testEvent(runID, 1, events.EventKindRunStarted),
		},
	})

	run, err := Open(workspaceRoot, runID)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	eventsCh, errsCh := run.Tail(ctx, 0)
	waitForEventChannelClose(t, eventsCh, "events", time.Second)
	waitForEventChannelClose(t, errsCh, "errors", time.Second)
}

func TestTailSkipsDuplicateLiveEvents(t *testing.T) {
	workspaceRoot := t.TempDir()
	runID := "run-tail-duplicates"
	runDir := writeRunFixture(t, workspaceRoot, runID, runFixture{
		runJSON: map[string]any{
			"run_id":         runID,
			"mode":           "exec",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
		},
		events: []events.Event{
			testEvent(runID, 1, events.EventKindRunStarted),
			testEvent(runID, 2, events.EventKindJobStarted),
		},
	})

	run, err := Open(workspaceRoot, runID)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventsCh, errsCh := run.Tail(ctx, 3)
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()
	<-timer.C

	appendEvents(t, filepath.Join(runDir, "events.jsonl"), []events.Event{
		testEvent(runID, 2, events.EventKindJobStarted),
		testEvent(runID, 3, events.EventKindRunCompleted),
	})

	got := collectTailEvents(t, eventsCh, errsCh, 1, 2*time.Second)
	if seqs := collectedSeqs(got); !slices.Equal(seqs, []uint64{3}) {
		t.Fatalf("Tail() seqs = %v, want [3]", seqs)
	}
}

func TestTailReportsSnapshotErrorForInvalidEventsPath(t *testing.T) {
	run := &Run{eventsPath: t.TempDir()}
	eventsCh, errsCh := run.Tail(context.Background(), 0)

	select {
	case err := <-errsCh:
		if err == nil || !strings.Contains(err.Error(), "snapshot run tail offset") {
			t.Fatalf("Tail() error = %v, want snapshot error", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for snapshot error")
	}

	waitForEventChannelClose(t, eventsCh, "events", time.Second)
	waitForEventChannelClose(t, errsCh, "errors", time.Second)
}

func TestTailReportsReplayErrors(t *testing.T) {
	workspaceRoot := t.TempDir()
	runID := "run-tail-replay-error"
	writeRunFixture(t, workspaceRoot, runID, runFixture{
		runJSON: map[string]any{
			"run_id":         runID,
			"mode":           "exec",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
		},
		events: []events.Event{
			testEvent(runID, 1, events.EventKindRunStarted),
		},
		partialTail: `{"schema_version":"1.0","run_id":"run-tail-replay-error","seq":2,"ts":"2026-04-06T12:00:02Z","kind":"job.started","payload":{`,
	})

	run, err := Open(workspaceRoot, runID)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventsCh, errsCh := run.Tail(ctx, 0)
	first := collectTailEvents(t, eventsCh, errsCh, 1, 2*time.Second)
	if seqs := collectedSeqs(first); !slices.Equal(seqs, []uint64{1}) {
		t.Fatalf("Tail() first seqs = %v, want [1]", seqs)
	}

	select {
	case err := <-errsCh:
		if err == nil || !errors.Is(err, ErrPartialEventLine) {
			t.Fatalf("Tail() replay error = %v, want ErrPartialEventLine", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for replay error")
	}
}
