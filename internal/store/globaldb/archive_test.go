package globaldb

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGetWorkflowArchiveEligibilityUsesSyncedTaskState(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspace := mustWorkspace(t, db)
	syncResult := mustReconcileArchiveWorkflow(t, db, workspace.ID, "pending-demo", []TaskItemInput{
		{
			TaskNumber: 1,
			TaskID:     "task_1",
			Title:      "Pending task",
			Status:     "pending",
			Kind:       "backend",
			SourcePath: "task_001.md",
		},
	}, nil)

	eligibility, err := db.GetWorkflowArchiveEligibility(context.Background(), workspace.ID, "pending-demo")
	if err != nil {
		t.Fatalf("GetWorkflowArchiveEligibility() error = %v", err)
	}
	if eligibility.Workflow.ID != syncResult.Workflow.ID {
		t.Fatalf("workflow id = %q, want %q", eligibility.Workflow.ID, syncResult.Workflow.ID)
	}
	if eligibility.TaskTotal != 1 || eligibility.PendingTasks != 1 {
		t.Fatalf("task counts = %#v, want total=1 pending=1", eligibility)
	}
	if got := eligibility.SkipReason(); got != "task workflow not fully completed" {
		t.Fatalf("SkipReason() = %q, want task workflow not fully completed", got)
	}

	conflict := eligibility.ConflictError()
	if !errors.Is(conflict, ErrWorkflowNotArchivable) {
		t.Fatalf("ConflictError() = %v, want ErrWorkflowNotArchivable", conflict)
	}
}

func TestGetWorkflowArchiveEligibilityUsesSyncedReviewState(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspace := mustWorkspace(t, db)
	mustReconcileArchiveWorkflow(t, db, workspace.ID, "review-demo", []TaskItemInput{
		{
			TaskNumber: 1,
			TaskID:     "task_1",
			Title:      "Completed task",
			Status:     "completed",
			Kind:       "backend",
			SourcePath: "task_001.md",
		},
	}, []ReviewRoundInput{
		{
			RoundNumber:     1,
			Provider:        "coderabbit",
			PRRef:           "259",
			ResolvedCount:   0,
			UnresolvedCount: 1,
			Issues: []ReviewIssueInput{
				{
					IssueNumber: 1,
					Severity:    "medium",
					Status:      "pending",
					SourcePath:  "reviews-001/issue_001.md",
				},
			},
		},
	})

	eligibility, err := db.GetWorkflowArchiveEligibility(context.Background(), workspace.ID, "review-demo")
	if err != nil {
		t.Fatalf("GetWorkflowArchiveEligibility() error = %v", err)
	}
	if eligibility.UnresolvedReviewIssues != 1 || eligibility.ReviewRoundCount != 1 {
		t.Fatalf("review counts = %#v, want unresolved=1 rounds=1", eligibility)
	}
	if got := eligibility.SkipReason(); got != "review rounds not fully resolved" {
		t.Fatalf("SkipReason() = %q, want review rounds not fully resolved", got)
	}
}

func TestGetWorkflowArchiveEligibilityForReviewOnlyWorkflows(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                     string
		slug                     string
		reviewRounds             []ReviewRoundInput
		wantReviewIssueTotal     int
		wantUnresolvedReviewRows int
		wantArchivable           bool
		wantSkipReason           string
	}{
		{
			name: "Should allow resolved review-only workflows to archive",
			slug: "review-only-resolved",
			reviewRounds: []ReviewRoundInput{
				{
					RoundNumber:     1,
					Provider:        "coderabbit",
					PRRef:           "259",
					ResolvedCount:   2,
					UnresolvedCount: 0,
					Issues: []ReviewIssueInput{
						{
							IssueNumber: 1,
							Severity:    "medium",
							Status:      "resolved",
							SourcePath:  "reviews-001/issue_001.md",
						},
						{
							IssueNumber: 2,
							Severity:    "low",
							Status:      "resolved",
							SourcePath:  "reviews-001/issue_002.md",
						},
					},
				},
			},
			wantReviewIssueTotal:     2,
			wantUnresolvedReviewRows: 0,
			wantArchivable:           true,
			wantSkipReason:           "",
		},
		{
			name: "Should block unresolved review-only workflows from archiving",
			slug: "review-only-pending",
			reviewRounds: []ReviewRoundInput{
				{
					RoundNumber:     1,
					Provider:        "coderabbit",
					PRRef:           "259",
					ResolvedCount:   1,
					UnresolvedCount: 1,
					Issues: []ReviewIssueInput{
						{
							IssueNumber: 1,
							Severity:    "medium",
							Status:      "resolved",
							SourcePath:  "reviews-001/issue_001.md",
						},
						{
							IssueNumber: 2,
							Severity:    "high",
							Status:      "pending",
							SourcePath:  "reviews-001/issue_002.md",
						},
					},
				},
			},
			wantReviewIssueTotal:     2,
			wantUnresolvedReviewRows: 1,
			wantArchivable:           false,
			wantSkipReason:           "review rounds not fully resolved",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db := openTestGlobalDB(t)
			defer func() {
				_ = db.Close()
			}()

			workspace := mustWorkspace(t, db)
			mustReconcileArchiveWorkflow(t, db, workspace.ID, tc.slug, nil, tc.reviewRounds)

			eligibility, err := db.GetWorkflowArchiveEligibility(context.Background(), workspace.ID, tc.slug)
			if err != nil {
				t.Fatalf("GetWorkflowArchiveEligibility() error = %v", err)
			}
			if eligibility.TaskTotal != 0 || eligibility.ReviewIssueTotal != tc.wantReviewIssueTotal ||
				eligibility.UnresolvedReviewIssues != tc.wantUnresolvedReviewRows {
				t.Fatalf("unexpected eligibility counts: %#v", eligibility)
			}
			if eligibility.Archivable() != tc.wantArchivable {
				t.Fatalf("Archivable() = %v, want %v", eligibility.Archivable(), tc.wantArchivable)
			}
			if got := eligibility.SkipReason(); got != tc.wantSkipReason {
				t.Fatalf("SkipReason() = %q, want %q", got, tc.wantSkipReason)
			}
		})
	}
}

func TestGetWorkflowArchiveEligibilityReportsActiveRuns(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspace := mustWorkspace(t, db)
	syncResult := mustReconcileArchiveWorkflow(t, db, workspace.ID, "active-demo", []TaskItemInput{
		{
			TaskNumber: 1,
			TaskID:     "task_1",
			Title:      "Completed task",
			Status:     "completed",
			Kind:       "backend",
			SourcePath: "task_001.md",
		},
	}, nil)

	if _, err := db.PutRun(context.Background(), Run{
		RunID:            "run-active-demo",
		WorkspaceID:      workspace.ID,
		WorkflowID:       &syncResult.Workflow.ID,
		Mode:             "task",
		Status:           "running",
		PresentationMode: "stream",
		StartedAt:        time.Date(2026, 4, 17, 19, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("PutRun() error = %v", err)
	}

	eligibility, err := db.GetWorkflowArchiveEligibility(context.Background(), workspace.ID, "active-demo")
	if err != nil {
		t.Fatalf("GetWorkflowArchiveEligibility() error = %v", err)
	}
	if eligibility.ActiveRuns != 1 {
		t.Fatalf("ActiveRuns = %d, want 1", eligibility.ActiveRuns)
	}
	if got := eligibility.SkipReason(); got != "workflow has active runs" {
		t.Fatalf("SkipReason() = %q, want workflow has active runs", got)
	}

	conflict := eligibility.ConflictError()
	if !errors.Is(conflict, ErrWorkflowHasActiveRuns) {
		t.Fatalf("ConflictError() = %v, want ErrWorkflowHasActiveRuns", conflict)
	}
}

func TestMarkWorkflowArchivedAndLookupArchivedWorkflowBySlug(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspace := mustWorkspace(t, db)
	syncResult := mustReconcileArchiveWorkflow(t, db, workspace.ID, "archive-demo", []TaskItemInput{
		{
			TaskNumber: 1,
			TaskID:     "task_1",
			Title:      "Completed task",
			Status:     "completed",
			Kind:       "backend",
			SourcePath: "task_001.md",
		},
	}, nil)

	archivedAt := time.Date(2026, 4, 17, 19, 5, 1, 123000000, time.UTC)
	archivedWorkflow, err := db.MarkWorkflowArchived(context.Background(), syncResult.Workflow.ID, archivedAt)
	if err != nil {
		t.Fatalf("MarkWorkflowArchived() error = %v", err)
	}
	if archivedWorkflow.ArchivedAt == nil || !archivedWorkflow.ArchivedAt.Equal(archivedAt) {
		t.Fatalf("ArchivedAt = %#v, want %v", archivedWorkflow.ArchivedAt, archivedAt)
	}

	lookup, err := db.GetLatestArchivedWorkflowBySlug(context.Background(), workspace.ID, "archive-demo")
	if err != nil {
		t.Fatalf("GetLatestArchivedWorkflowBySlug() error = %v", err)
	}
	if lookup.ID != archivedWorkflow.ID {
		t.Fatalf("archived workflow id = %q, want %q", lookup.ID, archivedWorkflow.ID)
	}

	if _, err := db.MarkWorkflowArchived(
		context.Background(),
		syncResult.Workflow.ID,
		archivedAt,
	); !errors.Is(
		err,
		ErrWorkflowArchived,
	) {
		t.Fatalf("MarkWorkflowArchived(already archived) error = %v, want ErrWorkflowArchived", err)
	}
}

func TestWorkflowArchiveEligibilityHelpersAndErrors(t *testing.T) {
	t.Parallel()

	eligibility := WorkflowArchiveEligibility{}
	if eligibility.Archivable() {
		t.Fatal("Archivable() = true, want false for no-task workflow")
	}
	if got := eligibility.SkipReason(); got != "no task files present" {
		t.Fatalf("SkipReason() = %q, want no task files present", got)
	}

	eligible := WorkflowArchiveEligibility{
		Workflow:     Workflow{Slug: "demo"},
		TaskTotal:    1,
		PendingTasks: 0,
	}
	if !eligible.Archivable() {
		t.Fatal("Archivable() = false, want true for completed workflow")
	}
	if got := eligible.SkipReason(); got != "" {
		t.Fatalf("SkipReason() = %q, want empty", got)
	}

	activeRunsErr := WorkflowActiveRunsError{Slug: "demo", ActiveRuns: 2}
	if !errors.Is(activeRunsErr, ErrWorkflowHasActiveRuns) {
		t.Fatalf("errors.Is(activeRunsErr, ErrWorkflowHasActiveRuns) = false")
	}
	if got := activeRunsErr.Error(); !strings.Contains(got, "active run") {
		t.Fatalf("WorkflowActiveRunsError.Error() = %q, want active-run detail", got)
	}

	notArchivableErr := WorkflowNotArchivableError{Slug: "demo", Reason: "task workflow not fully completed"}
	if !errors.Is(notArchivableErr, ErrWorkflowNotArchivable) {
		t.Fatalf("errors.Is(notArchivableErr, ErrWorkflowNotArchivable) = false")
	}
	if got := notArchivableErr.Error(); !strings.Contains(got, "not archivable") {
		t.Fatalf("WorkflowNotArchivableError.Error() = %q, want not-archivable detail", got)
	}

	archivedErr := WorkflowArchivedError{Slug: "demo"}
	if !errors.Is(archivedErr, ErrWorkflowArchived) {
		t.Fatalf("errors.Is(archivedErr, ErrWorkflowArchived) = false")
	}
	if got := archivedErr.Error(); !strings.Contains(got, "already archived") {
		t.Fatalf("WorkflowArchivedError.Error() = %q, want archived detail", got)
	}
}

func TestWorkflowArchiveEligibilityNoTasksAndArchivedLookupNotFound(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspace := mustWorkspace(t, db)
	mustReconcileArchiveWorkflow(t, db, workspace.ID, "empty-demo", nil, nil)

	eligibility, err := db.GetWorkflowArchiveEligibility(context.Background(), workspace.ID, "empty-demo")
	if err != nil {
		t.Fatalf("GetWorkflowArchiveEligibility() error = %v", err)
	}
	if got := eligibility.SkipReason(); got != "no task files present" {
		t.Fatalf("SkipReason() = %q, want no task files present", got)
	}
	if _, err := db.GetLatestArchivedWorkflowBySlug(
		context.Background(),
		workspace.ID,
		"missing-demo",
	); !errors.Is(
		err,
		ErrWorkflowNotFound,
	) {
		t.Fatalf("GetLatestArchivedWorkflowBySlug(missing) error = %v, want ErrWorkflowNotFound", err)
	}
}

func TestMarkWorkflowArchivedUsesDefaultTimestamp(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 17, 20, 0, 0, 0, time.UTC)
	dbPath := filepath.Join(t.TempDir(), "global.db")
	db, err := openWithOptions(context.Background(), dbPath, openOptions{
		now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("openWithOptions() error = %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	workspaceRoot := t.TempDir()
	workspace, err := db.Register(context.Background(), workspaceRoot, "")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	syncResult := mustReconcileArchiveWorkflow(t, db, workspace.ID, "default-time-demo", []TaskItemInput{
		{
			TaskNumber: 1,
			TaskID:     "task_1",
			Title:      "Completed task",
			Status:     "completed",
			Kind:       "backend",
			SourcePath: "task_001.md",
		},
	}, nil)

	archivedWorkflow, err := db.MarkWorkflowArchived(context.Background(), syncResult.Workflow.ID, time.Time{})
	if err != nil {
		t.Fatalf("MarkWorkflowArchived(zero time) error = %v", err)
	}
	if archivedWorkflow.ArchivedAt == nil || !archivedWorkflow.ArchivedAt.Equal(now) {
		t.Fatalf("ArchivedAt = %#v, want %v", archivedWorkflow.ArchivedAt, now)
	}
}

func TestArchiveCatalogMissingWorkflowBranches(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspace := mustWorkspace(t, db)
	if _, err := db.GetWorkflowArchiveEligibility(
		context.Background(),
		workspace.ID,
		"missing-demo",
	); !errors.Is(
		err,
		ErrWorkflowNotFound,
	) {
		t.Fatalf("GetWorkflowArchiveEligibility(missing) error = %v, want ErrWorkflowNotFound", err)
	}
	if _, err := db.MarkWorkflowArchived(
		context.Background(),
		"missing-workflow",
		time.Time{},
	); !errors.Is(
		err,
		ErrWorkflowNotFound,
	) {
		t.Fatalf("MarkWorkflowArchived(missing) error = %v, want ErrWorkflowNotFound", err)
	}
}

func TestArchiveErrorFallbackMessages(t *testing.T) {
	t.Parallel()

	if got := (WorkflowArchivedError{}).Error(); got == "" {
		t.Fatal("WorkflowArchivedError{}.Error() = empty, want fallback message")
	}
	if got := (WorkflowActiveRunsError{}).Error(); got == "" {
		t.Fatal("WorkflowActiveRunsError{}.Error() = empty, want fallback message")
	}
	if got := (WorkflowNotArchivableError{}).Error(); got == "" {
		t.Fatal("WorkflowNotArchivableError{}.Error() = empty, want fallback message")
	}
}

func mustReconcileArchiveWorkflow(
	t *testing.T,
	db *GlobalDB,
	workspaceID string,
	slug string,
	tasks []TaskItemInput,
	reviews []ReviewRoundInput,
) WorkflowSyncResult {
	t.Helper()

	result, err := db.ReconcileWorkflowSync(context.Background(), WorkflowSyncInput{
		WorkspaceID:        workspaceID,
		WorkflowSlug:       slug,
		SyncedAt:           time.Date(2026, 4, 17, 18, 50, 0, 0, time.UTC),
		CheckpointScope:    "workflow",
		CheckpointChecksum: slug + "-checkpoint",
		TaskItems:          tasks,
		ReviewRounds:       reviews,
	})
	if err != nil {
		t.Fatalf("ReconcileWorkflowSync(%q) error = %v", slug, err)
	}
	return result
}
