package globaldb

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestListInterruptedRunsAndMarkRunCrashed(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspace := mustWorkspace(t, db)
	startedAt := time.Date(2026, 4, 17, 18, 0, 0, 0, time.UTC)
	for _, run := range []Run{
		{
			RunID:            "run-starting",
			WorkspaceID:      workspace.ID,
			Mode:             "task",
			Status:           "starting",
			PresentationMode: "stream",
			StartedAt:        startedAt,
		},
		{
			RunID:            "run-running",
			WorkspaceID:      workspace.ID,
			Mode:             "task",
			Status:           "running",
			PresentationMode: "stream",
			StartedAt:        startedAt.Add(time.Second),
		},
		{
			RunID:            "run-completed",
			WorkspaceID:      workspace.ID,
			Mode:             "task",
			Status:           "completed",
			PresentationMode: "stream",
			StartedAt:        startedAt.Add(2 * time.Second),
			EndedAt:          timePtr(startedAt.Add(3 * time.Second)),
		},
	} {
		if _, err := db.PutRun(context.Background(), run); err != nil {
			t.Fatalf("PutRun(%q) error = %v", run.RunID, err)
		}
	}

	interrupted, err := db.ListInterruptedRuns(context.Background())
	if err != nil {
		t.Fatalf("ListInterruptedRuns() error = %v", err)
	}
	if got, want := runIDs(interrupted), []string{"run-starting", "run-running"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("interrupted runs = %v, want %v", got, want)
	}

	reconciledAt := startedAt.Add(10 * time.Minute)
	updated, err := db.MarkRunCrashed(context.Background(), "run-running", reconciledAt, "reconciled crash")
	if err != nil {
		t.Fatalf("MarkRunCrashed() error = %v", err)
	}
	if updated.Status != "crashed" {
		t.Fatalf("status = %q, want crashed", updated.Status)
	}
	if updated.EndedAt == nil || !updated.EndedAt.Equal(reconciledAt) {
		t.Fatalf("ended_at = %#v, want %v", updated.EndedAt, reconciledAt)
	}
	if updated.ErrorText != "reconciled crash" {
		t.Fatalf("error_text = %q, want reconciled crash", updated.ErrorText)
	}
}

func TestListTerminalRunsForPurgeRespectsKeepDaysAndKeepMaxOldestFirst(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspace := mustWorkspace(t, db)
	now := time.Date(2026, 4, 17, 18, 0, 0, 0, time.UTC)

	type testRun struct {
		id      string
		status  string
		endedAt time.Time
	}
	runs := []testRun{
		{id: "run-oldest", status: "completed", endedAt: now.AddDate(0, 0, -21)},
		{id: "run-old-age", status: "failed", endedAt: now.AddDate(0, 0, -15)},
		{id: "run-middle", status: "crashed", endedAt: now.AddDate(0, 0, -5)},
		{id: "run-newest", status: "canceled", endedAt: now.AddDate(0, 0, -1)},
		{id: "run-active", status: "running", endedAt: now},
	}
	for idx, item := range runs {
		run := Run{
			RunID:            item.id,
			WorkspaceID:      workspace.ID,
			Mode:             "task",
			Status:           item.status,
			PresentationMode: "stream",
			StartedAt:        item.endedAt.Add(-time.Minute).Add(time.Duration(idx)),
		}
		if item.status != "running" {
			run.EndedAt = timePtr(item.endedAt)
		}
		if _, err := db.PutRun(context.Background(), run); err != nil {
			t.Fatalf("PutRun(%q) error = %v", run.RunID, err)
		}
	}

	candidates, err := db.ListTerminalRunsForPurge(context.Background(), RunRetentionPolicy{
		KeepTerminalDays: 14,
		KeepMax:          2,
		Now:              now,
	})
	if err != nil {
		t.Fatalf("ListTerminalRunsForPurge() error = %v", err)
	}

	if got, want := runIDs(candidates), []string{"run-oldest", "run-old-age"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("purge candidates = %v, want %v", got, want)
	}
}

func TestCountWorkspacesAndActiveRuns(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspaceA := mustWorkspace(t, db)
	workspaceB := mustWorkspace(t, db)
	startedAt := time.Date(2026, 4, 17, 18, 0, 0, 0, time.UTC)
	for _, run := range []Run{
		{
			RunID:            "run-a",
			WorkspaceID:      workspaceA.ID,
			Mode:             "task",
			Status:           "running",
			PresentationMode: "stream",
			StartedAt:        startedAt,
		},
		{
			RunID:            "run-b",
			WorkspaceID:      workspaceB.ID,
			Mode:             "task",
			Status:           "completed",
			PresentationMode: "stream",
			StartedAt:        startedAt,
			EndedAt:          timePtr(startedAt.Add(time.Minute)),
		},
	} {
		if _, err := db.PutRun(context.Background(), run); err != nil {
			t.Fatalf("PutRun(%q) error = %v", run.RunID, err)
		}
	}

	workspaceCount, err := db.CountWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("CountWorkspaces() error = %v", err)
	}
	if workspaceCount != 2 {
		t.Fatalf("workspace count = %d, want 2", workspaceCount)
	}

	activeRuns, err := db.CountActiveRuns(context.Background())
	if err != nil {
		t.Fatalf("CountActiveRuns() error = %v", err)
	}
	if activeRuns != 1 {
		t.Fatalf("active run count = %d, want 1", activeRuns)
	}
}

func mustWorkspace(t *testing.T, db *GlobalDB) Workspace {
	t.Helper()

	workspaceRoot := t.TempDir()
	workspace, err := db.Register(context.Background(), workspaceRoot, "")
	if err != nil {
		t.Fatalf("Register(%q) error = %v", workspaceRoot, err)
	}
	return workspace
}

func runIDs(runs []Run) []string {
	ids := make([]string, 0, len(runs))
	for i := range runs {
		ids = append(ids, runs[i].RunID)
	}
	return ids
}

func timePtr(value time.Time) *time.Time {
	return &value
}
