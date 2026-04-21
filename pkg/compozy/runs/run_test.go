package runs

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
)

func TestOpenLoadsDaemonBackedRunSummary(t *testing.T) {
	reader := &stubDaemonRunReader{
		openSummary: RunSummary{
			RunID:         "run-open",
			Status:        publicRunStatusCompleted,
			Mode:          "prd-tasks",
			IDE:           "codex",
			Model:         "gpt-5.4",
			WorkspaceRoot: "/workspace",
			StartedAt:     time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
			EndedAt:       timePointer(time.Date(2026, 4, 17, 12, 3, 0, 0, time.UTC)),
			ArtifactsDir:  "/home/example/.compozy/runs/run-open",
		},
	}
	withStubDaemonRunReader(t, reader)

	run, err := Open("/workspace", "run-open")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	mustDeepEqual(t, run.Summary(), reader.openSummary)
	if len(reader.openCalls) != 1 || reader.openCalls[0] != "run-open" {
		t.Fatalf("openCalls = %#v, want [run-open]", reader.openCalls)
	}
	if len(reader.openWorkspace) != 1 || reader.openWorkspace[0] != "/workspace" {
		t.Fatalf("openWorkspace = %#v, want [/workspace]", reader.openWorkspace)
	}
}

func TestOpenReturnsDescriptiveErrorForMissingRunID(t *testing.T) {
	withStubDaemonRunReader(t, &stubDaemonRunReader{})
	_, err := Open(t.TempDir(), "")
	if err == nil || !strings.Contains(err.Error(), "missing run id") {
		t.Fatalf("Open() error = %v, want missing run id", err)
	}
}

func TestOpenSurfacesStableDaemonUnavailableError(t *testing.T) {
	withStubDaemonRunReader(t, &stubDaemonRunReader{
		openErr: wrapDaemonUnavailable("open run", errors.New("daemon info missing")),
	})

	_, err := Open("/workspace", "run-unavailable")
	if err == nil || !errors.Is(err, ErrDaemonUnavailable) {
		t.Fatalf("Open() error = %v, want ErrDaemonUnavailable", err)
	}
}

func TestAdaptRemoteRunSnapshotPreservesIncompleteReasons(t *testing.T) {
	got := adaptRemoteRunSnapshot(apicore.RunSnapshot{
		Run:               apicore.Run{Status: publicRunStatusFailed},
		Incomplete:        true,
		IncompleteReasons: []string{"event_gap", "transcript_gap"},
		NextCursor: &apicore.StreamCursor{
			Timestamp: time.Date(2026, 4, 21, 7, 0, 0, 0, time.UTC),
			Sequence:  7,
		},
	})

	if !got.Incomplete {
		t.Fatal("Incomplete = false, want true")
	}
	if want := []string{"event_gap", "transcript_gap"}; !reflect.DeepEqual(got.IncompleteReasons, want) {
		t.Fatalf("IncompleteReasons = %#v, want %#v", got.IncompleteReasons, want)
	}
	if got.NextCursor == nil || got.NextCursor.Sequence != 7 {
		t.Fatalf("NextCursor = %#v, want sequence 7", got.NextCursor)
	}
}
