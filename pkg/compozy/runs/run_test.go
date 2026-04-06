package runs

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events"
)

func TestOpenLoadsRunSummaryAndReplayFromSequence(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	runID := "run-open"
	startedAt := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	endedAt := startedAt.Add(3 * time.Minute)
	writeRunFixture(t, workspaceRoot, runID, map[string]any{
		"run_id":         runID,
		"status":         "completed",
		"mode":           "prd-tasks",
		"ide":            "codex",
		"model":          "gpt-5.4",
		"workspace_root": workspaceRoot,
		"artifacts_dir":  filepath.Join(workspaceRoot, ".compozy", "runs", runID),
		"created_at":     startedAt,
		"updated_at":     endedAt,
	}, []events.Event{
		testReplayEvent(runID, 1),
		testReplayEvent(runID, 2),
		testReplayEvent(runID, 3),
	}, "")

	run, err := Open(workspaceRoot, runID)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	summary := run.Summary()
	if summary.RunID != runID {
		t.Fatalf("Summary().RunID = %q, want %q", summary.RunID, runID)
	}
	if summary.Status != "completed" {
		t.Fatalf("Summary().Status = %q, want completed", summary.Status)
	}
	if summary.Mode != "prd-tasks" {
		t.Fatalf("Summary().Mode = %q, want prd-tasks", summary.Mode)
	}
	if summary.WorkspaceRoot != workspaceRoot {
		t.Fatalf("Summary().WorkspaceRoot = %q, want %q", summary.WorkspaceRoot, workspaceRoot)
	}
	if !summary.StartedAt.Equal(startedAt) {
		t.Fatalf("Summary().StartedAt = %s, want %s", summary.StartedAt, startedAt)
	}
	if summary.EndedAt == nil || !summary.EndedAt.Equal(endedAt) {
		t.Fatalf("Summary().EndedAt = %v, want %s", summary.EndedAt, endedAt)
	}

	gotEvents, gotErrors := collectReplay(run, 2)
	if len(gotErrors) != 0 {
		t.Fatalf("Replay() unexpected errors: %v", gotErrors)
	}
	if seqs := collectedSeqs(gotEvents); !slices.Equal(seqs, []uint64{2, 3}) {
		t.Fatalf("Replay() seqs = %v, want [2 3]", seqs)
	}
}

func TestReplayReportsPartialFinalLineWithoutDroppingCompleteEvents(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	runID := "run-partial"
	writeRunFixture(t, workspaceRoot, runID, map[string]any{
		"run_id":         runID,
		"mode":           "prd-tasks",
		"workspace_root": workspaceRoot,
	}, []events.Event{
		testReplayEvent(runID, 1),
		testReplayEvent(runID, 2),
	}, `{"schema_version":"1.0","run_id":"run-partial"`)

	run, err := Open(workspaceRoot, runID)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	gotEvents, gotErrors := collectReplay(run, 0)
	if seqs := collectedSeqs(gotEvents); !slices.Equal(seqs, []uint64{1, 2}) {
		t.Fatalf("Replay() seqs = %v, want [1 2]", seqs)
	}
	if len(gotErrors) != 1 || !errors.Is(gotErrors[0], ErrPartialEventLine) {
		t.Fatalf("Replay() errors = %v, want partial-final-line error", gotErrors)
	}
}

func TestReplayRejectsIncompatibleSchemaVersion(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	runID := "run-schema"
	ev := testReplayEvent(runID, 1)
	ev.SchemaVersion = "99.0"
	writeRunFixture(t, workspaceRoot, runID, map[string]any{
		"run_id":         runID,
		"workspace_root": workspaceRoot,
	}, []events.Event{ev}, "")

	run, err := Open(workspaceRoot, runID)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	gotEvents, gotErrors := collectReplay(run, 0)
	if len(gotEvents) != 0 {
		t.Fatalf("Replay() events = %v, want none", gotEvents)
	}
	if len(gotErrors) != 1 || !errors.Is(gotErrors[0], ErrIncompatibleSchemaVersion) {
		t.Fatalf("Replay() errors = %v, want incompatible-schema error", gotErrors)
	}
}

func TestOpenTreatsCancelledStatusAsTerminal(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	runID := "run-canceled"
	startedAt := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	endedAt := startedAt.Add(2 * time.Minute)
	writeRunFixture(t, workspaceRoot, runID, map[string]any{
		"run_id":         runID,
		"status":         "canceled",
		"workspace_root": workspaceRoot,
		"created_at":     startedAt,
		"updated_at":     endedAt,
	}, []events.Event{testReplayEvent(runID, 1)}, "")

	run, err := Open(workspaceRoot, runID)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	summary := run.Summary()
	if summary.EndedAt == nil || !summary.EndedAt.Equal(endedAt) {
		t.Fatalf("Summary().EndedAt = %v, want %s", summary.EndedAt, endedAt)
	}
}

func writeRunFixture(
	t *testing.T,
	workspaceRoot string,
	runID string,
	runJSON map[string]any,
	eventsToWrite []events.Event,
	partialTail string,
) {
	t.Helper()

	runDir := filepath.Join(workspaceRoot, ".compozy", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	payload, err := json.Marshal(runJSON)
	if err != nil {
		t.Fatalf("marshal run.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "run.json"), payload, 0o600); err != nil {
		t.Fatalf("write run.json: %v", err)
	}

	file, err := os.OpenFile(filepath.Join(runDir, "events.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatalf("open events.jsonl: %v", err)
	}
	defer func() {
		_ = file.Close()
	}()

	encoder := json.NewEncoder(file)
	for _, ev := range eventsToWrite {
		if err := encoder.Encode(ev); err != nil {
			t.Fatalf("encode event: %v", err)
		}
	}
	if partialTail != "" {
		if _, err := file.WriteString(partialTail); err != nil {
			t.Fatalf("write partial tail: %v", err)
		}
	}
}

func collectReplay(run *Run, fromSeq uint64) ([]events.Event, []error) {
	var (
		gotEvents []events.Event
		gotErrors []error
	)

	for ev, err := range run.Replay(fromSeq) {
		if err != nil {
			gotErrors = append(gotErrors, err)
			continue
		}
		gotEvents = append(gotEvents, ev)
	}
	return gotEvents, gotErrors
}

func collectedSeqs(items []events.Event) []uint64 {
	seqs := make([]uint64, 0, len(items))
	for _, item := range items {
		seqs = append(seqs, item.Seq)
	}
	return seqs
}

func testReplayEvent(runID string, seq uint64) events.Event {
	return events.Event{
		SchemaVersion: events.SchemaVersion,
		RunID:         runID,
		Seq:           seq,
		Timestamp:     time.Unix(int64(seq), 0).UTC(),
		Kind:          events.EventKindJobStarted,
		Payload:       json.RawMessage(`{"seq":1}`),
	}
}
