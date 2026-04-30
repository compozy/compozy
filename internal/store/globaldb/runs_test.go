package globaldb

import (
	"context"
	"reflect"
	"strings"
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

func TestPutRunNormalizesStatusAndListRunsUsesWorkspaceStatusIndex(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspace := mustWorkspace(t, db)
	workflow, err := db.PutWorkflow(context.Background(), Workflow{
		WorkspaceID: workspace.ID,
		Slug:        "demo",
		CreatedAt:   time.Date(2026, 4, 17, 19, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 17, 19, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("PutWorkflow() error = %v", err)
	}

	for idx, status := range []string{" Running ", "canceled", "completed"} {
		run := Run{
			RunID: "run-status-" + time.Date(2026, 4, 17, 19, 0, 0, idx, time.UTC).
				Format("150405.000000000"),
			WorkspaceID:      workspace.ID,
			WorkflowID:       &workflow.ID,
			Mode:             "task",
			Status:           status,
			PresentationMode: "stream",
			StartedAt:        time.Date(2026, 4, 17, 19, idx, 0, 0, time.UTC),
		}
		if _, err := db.PutRun(context.Background(), run); err != nil {
			t.Fatalf("PutRun(%q) error = %v", run.RunID, err)
		}
	}

	row, err := db.GetRun(context.Background(), "run-status-190000.000000001")
	if err != nil {
		t.Fatalf("GetRun(canceled) error = %v", err)
	}
	if row.Status != runStatusCanceled {
		t.Fatalf("normalized canceled status = %q, want %q", row.Status, runStatusCanceled)
	}

	plan := explainQueryPlan(
		t,
		db,
		`SELECT run_id
		 FROM runs
		 WHERE workspace_id = ? AND status = ?
		 ORDER BY started_at DESC, run_id ASC
		 LIMIT ?`,
		workspace.ID,
		runStatusRunning,
		10,
	)
	if !strings.Contains(plan, "idx_runs_workspace_status") {
		t.Fatalf("query plan = %q, want idx_runs_workspace_status", plan)
	}
}

func TestRunParentRunIDRoundTripsThroughDurableQueries(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspace := mustWorkspace(t, db)
	startedAt := time.Date(2026, 4, 17, 21, 0, 0, 0, time.UTC)
	parent := Run{
		RunID:            "run-parent",
		WorkspaceID:      workspace.ID,
		Mode:             "review_watch",
		Status:           "running",
		PresentationMode: "stream",
		StartedAt:        startedAt,
	}
	if _, err := db.PutRun(context.Background(), parent); err != nil {
		t.Fatalf("PutRun(parent) error = %v", err)
	}

	child, err := db.PutRun(context.Background(), Run{
		RunID:            "run-child",
		WorkspaceID:      workspace.ID,
		ParentRunID:      " " + parent.RunID + " ",
		Mode:             "review",
		Status:           "running",
		PresentationMode: "stream",
		StartedAt:        startedAt.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("PutRun(child) error = %v", err)
	}
	if child.ParentRunID != parent.RunID {
		t.Fatalf("inserted child ParentRunID = %q, want %q", child.ParentRunID, parent.RunID)
	}

	got, err := db.GetRun(context.Background(), child.RunID)
	if err != nil {
		t.Fatalf("GetRun(child) error = %v", err)
	}
	if got.ParentRunID != parent.RunID {
		t.Fatalf("GetRun(child).ParentRunID = %q, want %q", got.ParentRunID, parent.RunID)
	}

	listed, err := db.ListRuns(context.Background(), ListRunsOptions{
		WorkspaceID: workspace.ID,
		Status:      "running",
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if row := findRunByID(listed, child.RunID); row == nil || row.ParentRunID != parent.RunID {
		t.Fatalf("ListRuns child = %#v, want parent_run_id %q", row, parent.RunID)
	}

	interrupted, err := db.ListInterruptedRuns(context.Background())
	if err != nil {
		t.Fatalf("ListInterruptedRuns() error = %v", err)
	}
	if row := findRunByID(interrupted, child.RunID); row == nil || row.ParentRunID != parent.RunID {
		t.Fatalf("ListInterruptedRuns child = %#v, want parent_run_id %q", row, parent.RunID)
	}

	completedAt := startedAt.Add(2 * time.Minute)
	got.ParentRunID = ""
	got.Status = "completed"
	got.EndedAt = &completedAt
	updated, err := db.UpdateRun(context.Background(), got)
	if err != nil {
		t.Fatalf("UpdateRun(child) error = %v", err)
	}
	if updated.ParentRunID != "" {
		t.Fatalf("updated child ParentRunID = %q, want empty", updated.ParentRunID)
	}
}

func TestMarkRunsCrashedAndDeleteRunsBatch(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspace := mustWorkspace(t, db)
	startedAt := time.Date(2026, 4, 17, 20, 0, 0, 0, time.UTC)
	for idx, runID := range []string{"run-batch-a", "run-batch-b", "run-batch-c"} {
		if _, err := db.PutRun(context.Background(), Run{
			RunID:            runID,
			WorkspaceID:      workspace.ID,
			Mode:             "task",
			Status:           "running",
			PresentationMode: "stream",
			StartedAt:        startedAt.Add(time.Duration(idx) * time.Second),
		}); err != nil {
			t.Fatalf("PutRun(%q) error = %v", runID, err)
		}
	}

	crashedAt := startedAt.Add(5 * time.Minute)
	if err := db.MarkRunsCrashed(context.Background(), []RunCrashUpdate{
		{RunID: "run-batch-a", EndedAt: crashedAt, ErrorText: "daemon stop"},
		{RunID: "run-batch-b", EndedAt: crashedAt.Add(time.Second), ErrorText: "journal unavailable"},
	}); err != nil {
		t.Fatalf("MarkRunsCrashed() error = %v", err)
	}

	for _, tc := range []struct {
		runID     string
		errorText string
	}{
		{runID: "run-batch-a", errorText: "daemon stop"},
		{runID: "run-batch-b", errorText: "journal unavailable"},
	} {
		row, err := db.GetRun(context.Background(), tc.runID)
		if err != nil {
			t.Fatalf("GetRun(%q) error = %v", tc.runID, err)
		}
		if row.Status != runStatusCrashed {
			t.Fatalf("run %q status = %q, want crashed", tc.runID, row.Status)
		}
		if strings.TrimSpace(row.ErrorText) != tc.errorText {
			t.Fatalf("run %q error_text = %q, want %q", tc.runID, row.ErrorText, tc.errorText)
		}
		if row.EndedAt == nil {
			t.Fatalf("run %q EndedAt = nil, want timestamp", tc.runID)
		}
	}

	if err := db.DeleteRuns(context.Background(), []string{"run-batch-a", "run-batch-b"}); err != nil {
		t.Fatalf("DeleteRuns() error = %v", err)
	}
	for _, runID := range []string{"run-batch-a", "run-batch-b"} {
		if _, err := db.GetRun(context.Background(), runID); err == nil {
			t.Fatalf("GetRun(%q) error = nil, want missing row", runID)
		}
	}
	if _, err := db.GetRun(context.Background(), "run-batch-c"); err != nil {
		t.Fatalf("GetRun(run-batch-c) error = %v, want row to remain", err)
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

func findRunByID(runs []Run, runID string) *Run {
	for i := range runs {
		if runs[i].RunID == runID {
			return &runs[i]
		}
	}
	return nil
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func explainQueryPlan(t *testing.T, db *GlobalDB, query string, args ...any) string {
	t.Helper()

	rows, err := db.db.QueryContext(context.Background(), "EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN error = %v", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	parts := make([]string, 0)
	for rows.Next() {
		var (
			id      int
			parent  int
			notused int
			detail  string
		)
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatalf("scan query plan row: %v", err)
		}
		parts = append(parts, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate query plan: %v", err)
	}
	return strings.Join(parts, " | ")
}
