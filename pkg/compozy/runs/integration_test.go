package runs

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/run/journal"
	"github.com/compozy/compozy/pkg/compozy/events"
)

func TestReplayRoundTripWithJournal(t *testing.T) {
	workspaceRoot := t.TempDir()
	runID := "run-journal"
	runDir := writeRunFixture(t, workspaceRoot, runID, runFixture{
		runJSON: map[string]any{
			"run_id":         runID,
			"mode":           "exec",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
		},
	})

	writer, err := journal.Open(filepath.Join(runDir, "events.jsonl"), nil, 16)
	if err != nil {
		t.Fatalf("journal.Open() error = %v", err)
	}

	expectedKinds := make([]events.EventKind, 0, 100)
	for index := 1; index <= 100; index++ {
		kind := events.EventKindJobStarted
		if index%2 == 0 {
			kind = events.EventKindJobCompleted
		}
		expectedKinds = append(expectedKinds, kind)
		if err := writer.Submit(context.Background(), events.Event{
			RunID:     runID,
			Timestamp: time.Unix(int64(index), 0).UTC(),
			Kind:      kind,
			Payload:   []byte(fmt.Sprintf(`{"index":%d}`, index)),
		}); err != nil {
			t.Fatalf("Submit() error = %v", err)
		}
	}
	if err := writer.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	run, err := Open(workspaceRoot, runID)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	replayed, replayErrs := collectReplay(run, 0)
	if len(replayErrs) != 0 {
		t.Fatalf("Replay() unexpected errors: %v", replayErrs)
	}
	if len(replayed) != 100 {
		t.Fatalf("Replay() returned %d events, want 100", len(replayed))
	}

	gotKinds := make([]events.EventKind, 0, len(replayed))
	wantSeqs := make([]uint64, 0, len(replayed))
	for index, ev := range replayed {
		gotKinds = append(gotKinds, ev.Kind)
		wantSeqs = append(wantSeqs, uint64(index+1))
	}
	if !slices.Equal(collectedSeqs(replayed), wantSeqs) {
		t.Fatalf("Replay() seqs = %v, want %v", collectedSeqs(replayed), wantSeqs)
	}
	if !slices.Equal(gotKinds, expectedKinds) {
		t.Fatalf("Replay() kinds = %v, want %v", gotKinds, expectedKinds)
	}
}
