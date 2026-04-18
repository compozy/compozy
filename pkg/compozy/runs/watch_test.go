package runs

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWatchWorkspaceEmitsCreatedStatusChangedAndRemoved(t *testing.T) {
	reader := &stubDaemonRunReader{
		listSummaries: [][]RunSummary{
			nil,
			{{
				RunID:         "run-watch",
				Status:        publicRunStatusRunning,
				WorkspaceRoot: "/workspace",
				StartedAt:     time.Unix(1, 0).UTC(),
			}},
			{{
				RunID:         "run-watch",
				Status:        publicRunStatusFailed,
				WorkspaceRoot: "/workspace",
				StartedAt:     time.Unix(1, 0).UTC(),
			}},
			nil,
		},
	}
	withStubDaemonRunReader(t, reader)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventsCh, errsCh := WatchWorkspace(ctx, "/workspace")

	created := awaitRunEvent(t, eventsCh, errsCh, time.Second)
	if created.Kind != RunEventCreated || created.RunID != "run-watch" {
		t.Fatalf("created event = %#v, want created for run-watch", created)
	}

	changed := awaitRunEvent(t, eventsCh, errsCh, time.Second)
	if changed.Kind != RunEventStatusChanged || changed.Summary == nil ||
		changed.Summary.Status != publicRunStatusFailed {
		t.Fatalf("changed event = %#v, want failed status change", changed)
	}

	removed := awaitRunEvent(t, eventsCh, errsCh, time.Second)
	if removed.Kind != RunEventRemoved || removed.RunID != "run-watch" {
		t.Fatalf("removed event = %#v, want removed for run-watch", removed)
	}
}

func TestWatchWorkspaceSurfacesInitialDaemonError(t *testing.T) {
	previous := resolveRunsDaemonReader
	resolveRunsDaemonReader = func() (daemonRunReader, error) {
		return nil, wrapDaemonUnavailable("list runs", context.DeadlineExceeded)
	}
	t.Cleanup(func() {
		resolveRunsDaemonReader = previous
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, errsCh := WatchWorkspace(ctx, "/workspace")
	var err error
	for item := range errsCh {
		err = item
	}
	if err == nil || !errors.Is(err, ErrDaemonUnavailable) {
		t.Fatalf("WatchWorkspace() error = %v, want ErrDaemonUnavailable", err)
	}
}
