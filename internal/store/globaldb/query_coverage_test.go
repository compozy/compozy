package globaldb

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubRollbackTx struct {
	called bool
	err    error
}

func (s *stubRollbackTx) Rollback() error {
	s.called = true
	return s.err
}

func TestReviewQueriesLoadLatestRoundAndIssues(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspace := registerSyncTestWorkspace(t, db)
	syncedAt := time.Date(2026, 4, 20, 21, 0, 0, 0, time.UTC)

	result, err := db.ReconcileWorkflowSync(context.Background(), WorkflowSyncInput{
		WorkspaceID:        workspace.ID,
		WorkflowSlug:       "review-query-demo",
		SyncedAt:           syncedAt,
		CheckpointChecksum: "review-query-checkpoint",
		ReviewRounds: []ReviewRoundInput{
			{
				RoundNumber:     1,
				Provider:        "coderabbit",
				PRRef:           "101",
				ResolvedCount:   0,
				UnresolvedCount: 2,
				Issues: []ReviewIssueInput{
					{IssueNumber: 1, Severity: "high", Status: "pending", SourcePath: "reviews-001/issue_001.md"},
					{IssueNumber: 2, Severity: "medium", Status: "pending", SourcePath: "reviews-001/issue_002.md"},
				},
			},
			{
				RoundNumber:     2,
				Provider:        "coderabbit",
				PRRef:           "101",
				ResolvedCount:   1,
				UnresolvedCount: 0,
				Issues: []ReviewIssueInput{
					{IssueNumber: 1, Severity: "low", Status: "resolved", SourcePath: "reviews-002/issue_001.md"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("ReconcileWorkflowSync() error = %v", err)
	}

	latest, err := db.GetLatestReviewRound(context.Background(), result.Workflow.ID)
	if err != nil {
		t.Fatalf("GetLatestReviewRound() error = %v", err)
	}
	if latest.RoundNumber != 2 || latest.Provider != "coderabbit" || latest.PRRef != "101" {
		t.Fatalf("unexpected latest review round: %#v", latest)
	}

	firstRound, err := db.GetReviewRound(context.Background(), result.Workflow.ID, 1)
	if err != nil {
		t.Fatalf("GetReviewRound(round=1) error = %v", err)
	}
	if firstRound.ResolvedCount != 0 || firstRound.UnresolvedCount != 2 {
		t.Fatalf("unexpected first round counts: %#v", firstRound)
	}

	issues, err := db.ListReviewIssues(context.Background(), firstRound.ID)
	if err != nil {
		t.Fatalf("ListReviewIssues() error = %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("len(issues) = %d, want 2", len(issues))
	}
	if issues[0].IssueNumber != 1 || issues[1].IssueNumber != 2 {
		t.Fatalf("unexpected issue ordering: %#v", issues)
	}
	if issues[0].Severity != "high" || issues[1].SourcePath != "reviews-001/issue_002.md" {
		t.Fatalf("unexpected issue payloads: %#v", issues)
	}

	if _, err := db.GetReviewRound(
		context.Background(),
		result.Workflow.ID,
		99,
	); !errors.Is(
		err,
		ErrReviewRoundNotFound,
	) {
		t.Fatalf("GetReviewRound(missing) error = %v, want ErrReviewRoundNotFound", err)
	}
	if _, err := db.GetLatestReviewRound(
		context.Background(),
		"missing-workflow",
	); !errors.Is(
		err,
		ErrReviewRoundNotFound,
	) {
		t.Fatalf("GetLatestReviewRound(missing) error = %v, want ErrReviewRoundNotFound", err)
	}
}

func TestRunQueriesDeleteAndWorkflowSlugHelpers(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspace := mustWorkspace(t, db)
	workflowA, err := db.PutWorkflow(context.Background(), Workflow{
		WorkspaceID: workspace.ID,
		Slug:        "workflow-a",
	})
	if err != nil {
		t.Fatalf("PutWorkflow(workflow-a) error = %v", err)
	}
	workflowB, err := db.PutWorkflow(context.Background(), Workflow{
		WorkspaceID: workspace.ID,
		Slug:        "workflow-b",
	})
	if err != nil {
		t.Fatalf("PutWorkflow(workflow-b) error = %v", err)
	}

	startedAt := time.Date(2026, 4, 20, 21, 30, 0, 0, time.UTC)
	for _, run := range []Run{
		{
			RunID:            "run-active",
			WorkspaceID:      workspace.ID,
			WorkflowID:       &workflowA.ID,
			Mode:             "task",
			Status:           "running",
			PresentationMode: "stream",
			StartedAt:        startedAt,
		},
		{
			RunID:            "run-delete",
			WorkspaceID:      workspace.ID,
			WorkflowID:       &workflowA.ID,
			Mode:             "task",
			Status:           "completed",
			PresentationMode: "stream",
			StartedAt:        startedAt.Add(time.Minute),
			EndedAt:          timePtr(startedAt.Add(2 * time.Minute)),
		},
		{
			RunID:            "run-completed-latest",
			WorkspaceID:      workspace.ID,
			WorkflowID:       &workflowB.ID,
			Mode:             "task",
			Status:           "completed",
			PresentationMode: "stream",
			StartedAt:        startedAt.Add(3 * time.Minute),
			EndedAt:          timePtr(startedAt.Add(4 * time.Minute)),
		},
	} {
		if _, err := db.PutRun(context.Background(), run); err != nil {
			t.Fatalf("PutRun(%q) error = %v", run.RunID, err)
		}
	}

	runs, err := db.ListRuns(context.Background(), ListRunsOptions{
		WorkspaceID: workspace.ID,
		Status:      " completed ",
		Mode:        "task",
		Limit:       1,
	})
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if len(runs) != 1 || runs[0].RunID != "run-completed-latest" {
		t.Fatalf("unexpected filtered runs: %#v", runs)
	}

	slugs, err := db.WorkflowSlugsByIDs(context.Background(), []string{"", workflowA.ID, workflowB.ID, workflowA.ID})
	if err != nil {
		t.Fatalf("WorkflowSlugsByIDs() error = %v", err)
	}
	if len(slugs) != 2 || slugs[workflowA.ID] != "workflow-a" || slugs[workflowB.ID] != "workflow-b" {
		t.Fatalf("unexpected workflow slug map: %#v", slugs)
	}
	if empty, err := db.WorkflowSlugsByIDs(context.Background(), []string{"", " "}); err != nil || len(empty) != 0 {
		t.Fatalf("WorkflowSlugsByIDs(empty) = %#v, %v; want empty map and nil error", empty, err)
	}

	if err := db.DeleteRun(context.Background(), "run-delete"); err != nil {
		t.Fatalf("DeleteRun() error = %v", err)
	}
	if _, err := db.GetRun(context.Background(), "run-delete"); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("GetRun(after delete) error = %v, want ErrRunNotFound", err)
	}
	if err := db.DeleteRun(context.Background(), "run-delete"); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("DeleteRun(missing) error = %v, want ErrRunNotFound", err)
	}

	if got := terminalRunAt(nil); !got.IsZero() {
		t.Fatalf("terminalRunAt(nil) = %v, want zero time", got)
	}
	if got := terminalRunAt(&Run{StartedAt: startedAt}); !got.Equal(startedAt) {
		t.Fatalf("terminalRunAt(no ended_at) = %v, want %v", got, startedAt)
	}
	endedLocal := time.Date(2026, 4, 20, 18, 35, 0, 0, time.FixedZone("UTC-3", -3*60*60))
	if got := terminalRunAt(&Run{StartedAt: startedAt, EndedAt: &endedLocal}); !got.Equal(endedLocal.UTC()) {
		t.Fatalf("terminalRunAt(ended) = %v, want %v", got, endedLocal.UTC())
	}

	tx := &stubRollbackTx{}
	rollbackPendingTx(tx)
	if !tx.called {
		t.Fatal("rollbackPendingTx() did not call Rollback()")
	}
	rollbackPendingTx(&stubRollbackTx{err: errors.New("rollback failed")})
	rollbackPendingTx(nil)
}
