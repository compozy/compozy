package runs

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestSchemaVersionErrorFormattingAndUnwrap(t *testing.T) {
	t.Parallel()

	var nilErr *SchemaVersionError
	if got := nilErr.Error(); got != ErrIncompatibleSchemaVersion.Error() {
		t.Fatalf("nil SchemaVersionError.Error() = %q, want %q", got, ErrIncompatibleSchemaVersion.Error())
	}

	err := &SchemaVersionError{Version: "99.0"}
	if got := err.Error(); !strings.Contains(got, "99.0") {
		t.Fatalf("SchemaVersionError.Error() = %q, want version", got)
	}
	if !errors.Is(err, ErrIncompatibleSchemaVersion) {
		t.Fatalf("errors.Is(%v, ErrIncompatibleSchemaVersion) = false, want true", err)
	}
}

func TestSummaryHandlesNilRun(t *testing.T) {
	t.Parallel()

	var run *Run
	if got := run.Summary(); !reflect.DeepEqual(got, RunSummary{}) {
		t.Fatalf("Summary() = %#v, want zero summary", got)
	}
}

func TestNormalizeStatusAndTerminalStates(t *testing.T) {
	t.Parallel()

	if got := normalizeStatus("succeeded"); got != publicRunStatusCompleted {
		t.Fatalf("normalizeStatus(succeeded) = %q, want %q", got, publicRunStatusCompleted)
	}
	if got := normalizeStatus("canceled"); got != publicRunStatusCancelled {
		t.Fatalf("normalizeStatus(canceled) = %q, want %q", got, publicRunStatusCancelled)
	}
	if !isTerminalStatus(publicRunStatusCrashed) {
		t.Fatal("expected crashed to be terminal")
	}
}

func TestCleanWorkspaceRoot(t *testing.T) {
	t.Parallel()

	if got := cleanWorkspaceRoot(" . "); got != "" {
		t.Fatalf("cleanWorkspaceRoot('.') = %q, want empty", got)
	}
	root := "/tmp/workspace"
	if got := cleanWorkspaceRoot(root + "/../workspace"); got != root {
		t.Fatalf("cleanWorkspaceRoot(clean) = %q, want %q", got, root)
	}
}

func TestApplySummaryEventDetailsUsesRunAndJobPayloads(t *testing.T) {
	t.Parallel()

	summary := RunSummary{
		RunID:        "run-123",
		Status:       publicRunStatusRunning,
		StartedAt:    time.Unix(1, 0).UTC(),
		ArtifactsDir: "/home/example/.compozy/runs/run-123",
	}
	applySummaryEventDetails(&summary, []events.Event{
		{
			SchemaVersion: events.SchemaVersion,
			RunID:         "run-123",
			Seq:           1,
			Timestamp:     time.Unix(1, 0).UTC(),
			Kind:          events.EventKindRunStarted,
			Payload: []byte(
				`{"workspace_root":"/workspace","artifacts_dir":"/home/example/.compozy/runs/run-123","ide":"codex","model":"gpt-5.5"}`,
			),
		},
		{
			SchemaVersion: events.SchemaVersion,
			RunID:         "run-123",
			Seq:           2,
			Timestamp:     time.Unix(2, 0).UTC(),
			Kind:          events.EventKindRunFailed,
			Payload:       []byte(`{"artifacts_dir":"/home/example/.compozy/runs/run-123","error":"boom"}`),
		},
	})

	if summary.WorkspaceRoot != "/workspace" {
		t.Fatalf("summary.WorkspaceRoot = %q, want /workspace", summary.WorkspaceRoot)
	}
	if summary.IDE != "codex" || summary.Model != "gpt-5.5" {
		t.Fatalf("summary IDE/model = %q/%q, want codex/gpt-5.5", summary.IDE, summary.Model)
	}
	if summary.EndedAt == nil || !summary.EndedAt.Equal(time.Unix(2, 0).UTC()) {
		t.Fatalf("summary.EndedAt = %v, want failure timestamp", summary.EndedAt)
	}
}

func TestApplySummaryEventDetailsProjectsLatestIndexedSpeedOutcomes(t *testing.T) {
	t.Parallel()

	summary := RunSummary{}
	applySummaryEventDetails(&summary, []events.Event{
		{
			Kind:    events.EventKindRunQueued,
			Payload: []byte(`{"speed":"fast"}`),
		},
		{
			Kind:    events.EventKindJobStarted,
			Payload: []byte(`{"index":0,"attempt":1,"speed":"fast"}`),
		},
		{
			Kind: events.EventKindJobFailed,
			Payload: []byte(
				`{"index":0,"attempt":1,"speed_resolution":{"requested":"fast","status":"rejected","reason":"provider_rejected"}}`,
			),
		},
		{
			Kind:    events.EventKindJobRetryScheduled,
			Payload: []byte(`{"index":0,"attempt":2}`),
		},
		{
			Kind:    events.EventKindJobStarted,
			Payload: []byte(`{"index":0,"attempt":2,"speed":"fast"}`),
		},
		{
			Kind: events.EventKindSessionStarted,
			Payload: []byte(
				`{"index":0,"acp_session_id":"session-0","speed_resolution":{"requested":"fast","status":"applied"}}`,
			),
		},
		{
			Kind:    events.EventKindJobStarted,
			Payload: []byte(`{"index":1,"attempt":1,"speed":"fast"}`),
		},
		{
			Kind: events.EventKindSessionStarted,
			Payload: []byte(
				`{"index":1,"acp_session_id":"session-1","speed_resolution":{"requested":"fast","status":"unsupported","reason":"capability_absent"}}`,
			),
		},
	})

	if summary.Speed != kinds.SpeedFast {
		t.Fatalf("summary speed = %q, want fast", summary.Speed)
	}
	want := map[int]kinds.SpeedResolution{
		0: {
			Requested: kinds.SpeedFast,
			Status:    kinds.SpeedResolutionStatusApplied,
		},
		1: {
			Requested: kinds.SpeedFast,
			Status:    kinds.SpeedResolutionStatusUnsupported,
			Reason:    kinds.SpeedResolutionReasonCapabilityAbsent,
		},
	}
	if !reflect.DeepEqual(summary.SpeedResolutions, want) {
		t.Fatalf("summary speed resolutions = %#v, want %#v", summary.SpeedResolutions, want)
	}
}

func TestApplySummaryEventDetailsPreservesHistoricalSpeedAbsence(t *testing.T) {
	t.Parallel()

	summary := RunSummary{}
	applySummaryEventDetails(&summary, []events.Event{
		{
			Kind:    events.EventKindRunStarted,
			Payload: []byte(`{"mode":"exec"}`),
		},
		{
			Kind:    events.EventKindJobStarted,
			Payload: []byte(`{"index":0,"attempt":1}`),
		},
		{
			Kind:    events.EventKindSessionStarted,
			Payload: []byte(`{"index":0,"acp_session_id":"historical"}`),
		},
	})

	if summary.Speed != "" || summary.SpeedResolutions != nil {
		t.Fatalf("historical summary speed fields = %#v, want absent", summary)
	}
}
