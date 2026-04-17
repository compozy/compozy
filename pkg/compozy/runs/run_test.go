package runs

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events"
)

func TestOpenLoadsRunSummary(t *testing.T) {
	workspaceRoot := t.TempDir()
	runID := "run-open"
	startedAt := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	endedAt := startedAt.Add(3 * time.Minute)
	writeRunFixture(t, workspaceRoot, runID, runFixture{
		runJSON: map[string]any{
			"run_id":        runID,
			"mode":          "prd-tasks",
			"ide":           "codex",
			"model":         "gpt-5.4",
			"artifacts_dir": filepath.Join(workspaceRoot, ".compozy", "runs", runID),
			"created_at":    startedAt,
		},
		resultJSON: map[string]any{
			"status": "succeeded",
		},
		events: []events.Event{
			testEvent(runID, 1, events.EventKindRunStarted),
			testEvent(runID, 2, events.EventKindRunCompleted),
		},
	})

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
	if summary.EndedAt == nil || !summary.EndedAt.Equal(time.Unix(2, 0).UTC()) {
		t.Fatalf("Summary().EndedAt = %v, want terminal event timestamp", summary.EndedAt)
	}
	if summary.ArtifactsDir != filepath.Join(workspaceRoot, ".compozy", "runs", runID) {
		t.Fatalf("Summary().ArtifactsDir = %q, want run dir", summary.ArtifactsDir)
	}
	_ = endedAt
}

func TestOpenTreatsResultStatusAsTerminalBeforeRunCompletedEvent(t *testing.T) {
	workspaceRoot := t.TempDir()
	runID := "run-result-first"
	startedAt := time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC)
	writeRunFixture(t, workspaceRoot, runID, runFixture{
		runJSON: map[string]any{
			"run_id":        runID,
			"mode":          "prd-tasks",
			"ide":           "codex",
			"artifacts_dir": filepath.Join(workspaceRoot, ".compozy", "runs", runID),
			"created_at":    startedAt,
		},
		resultJSON: map[string]any{
			"status": "succeeded",
		},
		events: []events.Event{
			testEvent(runID, 1, events.EventKindRunStarted),
			testEvent(runID, 2, events.EventKindJobCompleted),
		},
	})

	run, err := Open(workspaceRoot, runID)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	summary := run.Summary()
	if summary.Status != publicRunStatusCompleted {
		t.Fatalf("Summary().Status = %q, want %q", summary.Status, publicRunStatusCompleted)
	}
	if summary.EndedAt != nil {
		t.Fatalf("Summary().EndedAt = %v, want nil without terminal run event", summary.EndedAt)
	}
}

func TestReplayHandlesEventLinesLargerThanOneMiB(t *testing.T) {
	workspaceRoot := t.TempDir()
	runID := "run-large-line"
	largeBlob := strings.Repeat("x", 2*1024*1024)
	writeRunFixture(t, workspaceRoot, runID, runFixture{
		runJSON: map[string]any{
			"run_id":         runID,
			"mode":           "exec",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
		},
		events: []events.Event{{
			SchemaVersion: events.SchemaVersion,
			RunID:         runID,
			Seq:           1,
			Timestamp:     time.Date(2026, 4, 6, 12, 0, 1, 0, time.UTC),
			Kind:          events.EventKindSessionUpdate,
			Payload:       json.RawMessage(`{"blob":"` + largeBlob + `"}`),
		}},
	})

	run, err := Open(workspaceRoot, runID)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	gotEvents, gotErrors := collectReplay(run, 0)
	if len(gotErrors) != 0 {
		t.Fatalf("Replay() unexpected errors: %v", gotErrors)
	}
	if len(gotEvents) != 1 {
		t.Fatalf("Replay() returned %d events, want 1", len(gotEvents))
	}
	if got := string(gotEvents[0].Payload); got != `{"blob":"`+largeBlob+`"}` {
		t.Fatalf("Replay() payload length = %d, want %d", len(got), len(`{"blob":"`+largeBlob+`"}`))
	}
}

func TestOpenReturnsDescriptiveErrorForMissingRunID(t *testing.T) {
	_, err := Open(t.TempDir(), "")
	if err == nil || !strings.Contains(err.Error(), "missing run id") {
		t.Fatalf("Open() error = %v, want missing run id", err)
	}
}

func TestOpenFallsBackToHomeScopedRunArtifacts(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	workspaceRoot := t.TempDir()
	runID := "run-home-fallback"
	homeRunDir := writeRunFixture(t, homeDir, runID, runFixture{
		runJSON: map[string]any{
			"run_id":         runID,
			"mode":           "prd-tasks",
			"workspace_root": workspaceRoot,
			"artifacts_dir":  filepath.Join(homeDir, ".compozy", "runs", runID),
			"created_at":     time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
		},
		events: []events.Event{
			testEvent(runID, 1, events.EventKindRunStarted),
			testEvent(runID, 2, events.EventKindRunCompleted),
		},
	})

	run, err := Open(workspaceRoot, runID)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	summary := run.Summary()
	if got, want := summary.ArtifactsDir, homeRunDir; got != want {
		t.Fatalf("Summary().ArtifactsDir = %q, want %q", got, want)
	}

	gotEvents, gotErrors := collectReplay(run, 0)
	if len(gotErrors) != 0 {
		t.Fatalf("Replay() unexpected errors: %v", gotErrors)
	}
	if seqs := collectedSeqs(gotEvents); !slices.Equal(seqs, []uint64{1, 2}) {
		t.Fatalf("Replay() seqs = %v, want [1 2]", seqs)
	}
}

func TestReplayYieldsAllEventsFromBeginning(t *testing.T) {
	workspaceRoot := t.TempDir()
	runID := "run-replay"
	writeRunFixture(t, workspaceRoot, runID, runFixture{
		runJSON: map[string]any{
			"run_id":         runID,
			"mode":           "exec",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC),
		},
		events: []events.Event{
			testEvent(runID, 1, events.EventKindRunStarted),
			testEvent(runID, 2, events.EventKindJobStarted),
			testEvent(runID, 3, events.EventKindRunCompleted),
		},
	})

	run, err := Open(workspaceRoot, runID)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	gotEvents, gotErrors := collectReplay(run, 0)
	if len(gotErrors) != 0 {
		t.Fatalf("Replay() unexpected errors: %v", gotErrors)
	}
	if seqs := collectedSeqs(gotEvents); !slices.Equal(seqs, []uint64{1, 2, 3}) {
		t.Fatalf("Replay() seqs = %v, want [1 2 3]", seqs)
	}
}

func TestReplaySkipsEventsBeforeSequence(t *testing.T) {
	workspaceRoot := t.TempDir()
	runID := "run-replay-seq"
	writeRunFixture(t, workspaceRoot, runID, runFixture{
		runJSON: map[string]any{
			"run_id":         runID,
			"mode":           "exec",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC),
		},
		events: []events.Event{
			testEvent(runID, 1, events.EventKindRunStarted),
			testEvent(runID, 2, events.EventKindJobStarted),
			testEvent(runID, 3, events.EventKindRunCompleted),
		},
	})

	run, err := Open(workspaceRoot, runID)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
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
	workspaceRoot := t.TempDir()
	runID := "run-partial"
	writeRunFixture(t, workspaceRoot, runID, runFixture{
		runJSON: map[string]any{
			"run_id":         runID,
			"mode":           "prd-tasks",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC),
		},
		events: []events.Event{
			testEvent(runID, 1, events.EventKindRunStarted),
			testEvent(runID, 2, events.EventKindJobStarted),
		},
		partialTail: `{"schema_version":"1.0","run_id":"run-partial"`,
	})

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
	workspaceRoot := t.TempDir()
	runID := "run-schema"
	rawLine := `{"schema_version":"99.0","run_id":"run-schema","seq":1,"ts":"2026-04-05T12:00:00Z","kind":"job.started","payload":{}}`
	writeRunFixture(t, workspaceRoot, runID, runFixture{
		runJSON: map[string]any{
			"run_id":         runID,
			"mode":           "exec",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC),
		},
		rawEventLines: []string{rawLine},
	})

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
	var schemaErr *SchemaVersionError
	if !errors.As(gotErrors[0], &schemaErr) || schemaErr.Version != "99.0" {
		t.Fatalf("Replay() schema error = %#v, want version 99.0", gotErrors[0])
	}
}

func TestReplayAcceptsAdditiveSchemaFields(t *testing.T) {
	workspaceRoot := t.TempDir()
	runID := "run-additive"
	rawLine := `{"schema_version":"1.0","run_id":"run-additive","seq":1,"ts":"2026-04-05T12:00:00Z","kind":"job.started","payload":{},"new_field":"ignored"}`
	writeRunFixture(t, workspaceRoot, runID, runFixture{
		runJSON: map[string]any{
			"run_id":         runID,
			"mode":           "exec",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC),
		},
		rawEventLines: []string{rawLine},
	})

	run, err := Open(workspaceRoot, runID)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	gotEvents, gotErrors := collectReplay(run, 0)
	if len(gotErrors) != 0 {
		t.Fatalf("Replay() unexpected errors: %v", gotErrors)
	}
	if len(gotEvents) != 1 || gotEvents[0].Seq != 1 {
		t.Fatalf("Replay() events = %#v, want one seq=1 event", gotEvents)
	}
}

func TestNormalizeStatusSupportsCancelledSpellings(t *testing.T) {
	if got := normalizeStatus("canceled"); got != publicRunStatusCancelled {
		t.Fatalf("normalizeStatus(canceled) = %q, want %s", got, publicRunStatusCancelled)
	}
	if got := normalizeStatus(publicRunStatusCancelled); got != publicRunStatusCancelled {
		t.Fatalf("normalizeStatus(canceled) = %q, want %s", got, publicRunStatusCancelled)
	}
}

func TestEventDecoderAcceptsRawJSONPayloads(t *testing.T) {
	line := []byte(
		`{"schema_version":"1.0","run_id":"run","seq":7,"ts":"2026-04-05T12:00:00Z","kind":"job.started","payload":{"value":1}}`,
	)
	ev, err := decodeEventLine(line, 1)
	if err != nil {
		t.Fatalf("decodeEventLine() error = %v", err)
	}
	if ev.Seq != 7 || ev.Kind != events.EventKindJobStarted {
		t.Fatalf("decodeEventLine() = %#v, want seq=7 kind=job.started", ev)
	}

	var payload map[string]int
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["value"] != 1 {
		t.Fatalf("payload = %#v, want value=1", payload)
	}
}
