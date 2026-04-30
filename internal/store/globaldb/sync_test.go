package globaldb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReconcileWorkflowSyncUpsertsSnapshotsAndStructuredRows(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspace := registerSyncTestWorkspace(t, db)
	syncedAt := time.Date(2026, 4, 17, 20, 0, 0, 0, time.UTC)

	result, err := db.ReconcileWorkflowSync(context.Background(), WorkflowSyncInput{
		WorkspaceID:        workspace.ID,
		WorkflowSlug:       "demo",
		SyncedAt:           syncedAt,
		CheckpointChecksum: "workflow-checksum-1",
		ArtifactSnapshots: []ArtifactSnapshotInput{
			{
				ArtifactKind:    "techspec",
				RelativePath:    "_techspec.md",
				Checksum:        "checksum-techspec",
				FrontmatterJSON: `{"status":"draft"}`,
				BodyText:        "# TechSpec",
				SourceMTime:     syncedAt.Add(-time.Minute),
			},
			{
				ArtifactKind:    "task",
				RelativePath:    "task_01.md",
				Checksum:        "checksum-task",
				FrontmatterJSON: `{"status":"pending","title":"Demo task"}`,
				BodyText:        "# Task 01",
				SourceMTime:     syncedAt.Add(-2 * time.Minute),
			},
		},
		TaskItems: []TaskItemInput{
			{
				TaskNumber: 1,
				TaskID:     "task_1",
				Title:      "Demo task",
				Status:     "pending",
				Kind:       "backend",
				DependsOn:  []string{"task_00.md"},
				SourcePath: "task_01.md",
			},
		},
		ReviewRounds: []ReviewRoundInput{
			{
				RoundNumber:     1,
				Provider:        "coderabbit",
				PRRef:           "123",
				ResolvedCount:   0,
				UnresolvedCount: 1,
				Issues: []ReviewIssueInput{
					{
						IssueNumber: 1,
						Severity:    "high",
						Status:      "pending",
						SourcePath:  "reviews-001/issue_001.md",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("ReconcileWorkflowSync(): %v", err)
	}
	if result.SnapshotsUpserted != 2 || result.TaskItemsUpserted != 1 {
		t.Fatalf("unexpected upsert counts: %#v", result)
	}
	if result.ReviewRoundsUpserted != 1 || result.ReviewIssuesUpserted != 1 {
		t.Fatalf("unexpected review upsert counts: %#v", result)
	}
	if result.CheckpointsUpdated != 1 {
		t.Fatalf("unexpected checkpoint count: %#v", result)
	}
	if result.Workflow.ID == "" {
		t.Fatalf("expected workflow row to be returned: %#v", result.Workflow)
	}

	assertRowCount(t, db, "artifact_snapshots", 2)
	assertRowCount(t, db, "task_items", 1)
	assertRowCount(t, db, "review_rounds", 1)
	assertRowCount(t, db, "review_issues", 1)
	assertRowCount(t, db, "sync_checkpoints", 1)

	var (
		checksum       string
		bodyStorage    string
		sourceMTimeRaw string
	)
	if err := db.db.QueryRowContext(
		context.Background(),
		`SELECT checksum, body_storage_kind, source_mtime
		 FROM artifact_snapshots
		 WHERE workflow_id = ? AND relative_path = ?`,
		result.Workflow.ID,
		"task_01.md",
	).Scan(&checksum, &bodyStorage, &sourceMTimeRaw); err != nil {
		t.Fatalf("query artifact snapshot: %v", err)
	}
	if checksum != "checksum-task" {
		t.Fatalf("artifact checksum = %q, want checksum-task", checksum)
	}
	if bodyStorage != artifactBodyInlineKind {
		t.Fatalf("artifact body_storage_kind = %q, want %q", bodyStorage, artifactBodyInlineKind)
	}
	if sourceMTimeRaw != "2026-04-17T19:58:00.000000000Z" {
		t.Fatalf("artifact source_mtime = %q, want 2026-04-17T19:58:00.000000000Z", sourceMTimeRaw)
	}

	var (
		taskRowID     string
		taskID        string
		taskTitle     string
		taskStatus    string
		dependsOnJSON string
	)
	if err := db.db.QueryRowContext(
		context.Background(),
		`SELECT id, task_id, title, status, depends_on_json
		 FROM task_items
		 WHERE workflow_id = ? AND task_number = 1`,
		result.Workflow.ID,
	).Scan(&taskRowID, &taskID, &taskTitle, &taskStatus, &dependsOnJSON); err != nil {
		t.Fatalf("query task item: %v", err)
	}
	if taskRowID == "" || taskID != "task_1" {
		t.Fatalf("unexpected task identity row: id=%q task_id=%q", taskRowID, taskID)
	}
	if taskTitle != "Demo task" || taskStatus != "pending" {
		t.Fatalf("unexpected task projection: title=%q status=%q", taskTitle, taskStatus)
	}
	if dependsOnJSON != `["task_00.md"]` {
		t.Fatalf("depends_on_json = %q, want [\"task_00.md\"]", dependsOnJSON)
	}

	var checkpointChecksum string
	if err := db.db.QueryRowContext(
		context.Background(),
		`SELECT checksum FROM sync_checkpoints WHERE workflow_id = ? AND scope = ?`,
		result.Workflow.ID,
		defaultSyncScope,
	).Scan(&checkpointChecksum); err != nil {
		t.Fatalf("query sync checkpoint: %v", err)
	}
	if checkpointChecksum != "workflow-checksum-1" {
		t.Fatalf("sync checkpoint checksum = %q, want workflow-checksum-1", checkpointChecksum)
	}
}

func TestReconcileWorkflowSyncKeepsStableChecksumsOnIdempotentResync(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspace := registerSyncTestWorkspace(t, db)
	firstSync := time.Date(2026, 4, 17, 20, 5, 0, 0, time.UTC)
	secondSync := firstSync.Add(2 * time.Hour)

	input := WorkflowSyncInput{
		WorkspaceID:        workspace.ID,
		WorkflowSlug:       "stable",
		SyncedAt:           firstSync,
		CheckpointChecksum: "same-checksum",
		ArtifactSnapshots: []ArtifactSnapshotInput{
			{
				ArtifactKind:    "task",
				RelativePath:    "task_01.md",
				Checksum:        "stable-checksum",
				FrontmatterJSON: `{"status":"pending"}`,
				BodyText:        "# Task 01",
				SourceMTime:     firstSync.Add(-time.Minute),
			},
		},
	}
	firstResult, err := db.ReconcileWorkflowSync(context.Background(), input)
	if err != nil {
		t.Fatalf("ReconcileWorkflowSync(first): %v", err)
	}

	input.SyncedAt = secondSync
	input.ArtifactSnapshots[0].SourceMTime = secondSync
	secondResult, err := db.ReconcileWorkflowSync(context.Background(), input)
	if err != nil {
		t.Fatalf("ReconcileWorkflowSync(second): %v", err)
	}
	if firstResult.Workflow.ID != secondResult.Workflow.ID {
		t.Fatalf(
			"workflow id changed across idempotent sync\nfirst: %q\nsecond: %q",
			firstResult.Workflow.ID,
			secondResult.Workflow.ID,
		)
	}

	var (
		checksum         string
		bodyText         string
		lastScanAtRaw    string
		lastSuccessAtRaw string
	)
	if err := db.db.QueryRowContext(
		context.Background(),
		`SELECT checksum, COALESCE(body_text, ''), synced_at
		 FROM artifact_snapshots
		 WHERE workflow_id = ? AND relative_path = ?`,
		firstResult.Workflow.ID,
		"task_01.md",
	).Scan(&checksum, &bodyText, &lastScanAtRaw); err != nil {
		t.Fatalf("query artifact snapshot after resync: %v", err)
	}
	if checksum != "stable-checksum" {
		t.Fatalf("artifact checksum = %q, want stable-checksum", checksum)
	}
	if bodyText != "# Task 01" {
		t.Fatalf("artifact body_text = %q, want original body", bodyText)
	}
	if lastScanAtRaw != "2026-04-17T22:05:00.000000000Z" {
		t.Fatalf("artifact synced_at = %q, want 2026-04-17T22:05:00.000000000Z", lastScanAtRaw)
	}

	if err := db.db.QueryRowContext(
		context.Background(),
		`SELECT last_scan_at, last_success_at
		 FROM sync_checkpoints
		 WHERE workflow_id = ? AND scope = ?`,
		firstResult.Workflow.ID,
		defaultSyncScope,
	).Scan(&lastScanAtRaw, &lastSuccessAtRaw); err != nil {
		t.Fatalf("query sync checkpoint after resync: %v", err)
	}
	if lastScanAtRaw != "2026-04-17T22:05:00.000000000Z" ||
		lastSuccessAtRaw != "2026-04-17T22:05:00.000000000Z" {
		t.Fatalf(
			"unexpected checkpoint timestamps: last_scan_at=%q last_success_at=%q",
			lastScanAtRaw,
			lastSuccessAtRaw,
		)
	}
}

func TestReconcileWorkflowSyncDeletesStaleProjectionRows(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspace := registerSyncTestWorkspace(t, db)
	firstSync := time.Date(2026, 4, 17, 21, 0, 0, 0, time.UTC)

	firstInput := WorkflowSyncInput{
		WorkspaceID:        workspace.ID,
		WorkflowSlug:       "prune-demo",
		SyncedAt:           firstSync,
		CheckpointChecksum: "checksum-1",
		ArtifactSnapshots: []ArtifactSnapshotInput{
			{
				ArtifactKind:    "task",
				RelativePath:    "task_01.md",
				Checksum:        "task-checksum",
				FrontmatterJSON: `{"status":"pending"}`,
				BodyText:        "# Task 01",
				SourceMTime:     firstSync,
			},
			{
				ArtifactKind:    "adr",
				RelativePath:    "adrs/adr-001.md",
				Checksum:        "adr-checksum",
				FrontmatterJSON: `{}`,
				BodyText:        "# ADR 001",
				SourceMTime:     firstSync,
			},
		},
		TaskItems: []TaskItemInput{
			{
				TaskNumber: 1,
				TaskID:     "task_1",
				Title:      "Task one",
				Status:     "pending",
				Kind:       "backend",
				SourcePath: "task_01.md",
			},
		},
		ReviewRounds: []ReviewRoundInput{
			{
				RoundNumber:     1,
				Provider:        "coderabbit",
				PRRef:           "123",
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
		},
	}
	result, err := db.ReconcileWorkflowSync(context.Background(), firstInput)
	if err != nil {
		t.Fatalf("ReconcileWorkflowSync(first): %v", err)
	}

	secondInput := WorkflowSyncInput{
		WorkspaceID:        workspace.ID,
		WorkflowSlug:       "prune-demo",
		SyncedAt:           firstSync.Add(time.Hour),
		CheckpointChecksum: "checksum-2",
		ArtifactSnapshots: []ArtifactSnapshotInput{
			{
				ArtifactKind:    "adr",
				RelativePath:    "adrs/adr-001.md",
				Checksum:        "adr-checksum",
				FrontmatterJSON: `{}`,
				BodyText:        "# ADR 001",
				SourceMTime:     firstSync.Add(time.Hour),
			},
		},
	}
	if _, err := db.ReconcileWorkflowSync(context.Background(), secondInput); err != nil {
		t.Fatalf("ReconcileWorkflowSync(second): %v", err)
	}

	assertRowCountByWorkflow(t, db, "artifact_snapshots", result.Workflow.ID, 1)
	assertRowCountByWorkflow(t, db, "task_items", result.Workflow.ID, 0)
	assertRowCountByWorkflow(t, db, "review_rounds", result.Workflow.ID, 0)
	assertRowCount(t, db, "review_issues", 0)
}

func TestPruneMissingActiveWorkflowsDeletesOnlyStaleActiveRows(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspace := registerSyncTestWorkspace(t, db)
	otherWorkspace := registerSyncTestWorkspace(t, db)
	syncedAt := time.Date(2026, 4, 18, 2, 0, 0, 0, time.UTC)

	present := mustReconcilePruneWorkflow(t, db, workspace.ID, "present", syncedAt)
	stale := mustReconcilePruneWorkflow(t, db, workspace.ID, "stale", syncedAt)
	archived := mustReconcilePruneWorkflow(t, db, workspace.ID, "archived", syncedAt)
	activeRun := mustReconcilePruneWorkflow(t, db, workspace.ID, "active-run", syncedAt)
	otherWorkspaceStale := mustReconcilePruneWorkflow(t, db, otherWorkspace.ID, "stale", syncedAt)

	var staleRoundID string
	if err := db.db.QueryRowContext(
		context.Background(),
		`SELECT id FROM review_rounds WHERE workflow_id = ? AND round_number = 1`,
		stale.Workflow.ID,
	).Scan(&staleRoundID); err != nil {
		t.Fatalf("query stale review round id: %v", err)
	}

	terminalEndedAt := syncedAt.Add(time.Minute)
	if _, err := db.PutRun(context.Background(), Run{
		RunID:            "run-stale-terminal",
		WorkspaceID:      workspace.ID,
		WorkflowID:       &stale.Workflow.ID,
		Mode:             "task",
		Status:           "completed",
		PresentationMode: "stream",
		StartedAt:        syncedAt,
		EndedAt:          &terminalEndedAt,
	}); err != nil {
		t.Fatalf("PutRun(terminal): %v", err)
	}
	if _, err := db.PutRun(context.Background(), Run{
		RunID:            "run-active",
		WorkspaceID:      workspace.ID,
		WorkflowID:       &activeRun.Workflow.ID,
		Mode:             "task",
		Status:           "running",
		PresentationMode: "stream",
		StartedAt:        syncedAt,
	}); err != nil {
		t.Fatalf("PutRun(active): %v", err)
	}
	if _, err := db.MarkWorkflowArchived(
		context.Background(),
		archived.Workflow.ID,
		syncedAt.Add(2*time.Minute),
	); err != nil {
		t.Fatalf("MarkWorkflowArchived(): %v", err)
	}

	result, err := db.PruneMissingActiveWorkflows(context.Background(), workspace.ID, []string{"present"})
	if err != nil {
		t.Fatalf("PruneMissingActiveWorkflows(): %v", err)
	}
	if !equalStringSlices(result.PrunedSlugs, []string{"stale"}) {
		t.Fatalf("PrunedSlugs = %#v, want [stale]", result.PrunedSlugs)
	}
	if len(result.Skipped) != 1 || result.Skipped[0].Slug != "active-run" ||
		result.Skipped[0].ActiveRuns != 1 || result.Skipped[0].Reason != archiveReasonActiveRuns {
		t.Fatalf("unexpected skipped rows: %#v", result.Skipped)
	}

	if _, err := db.GetActiveWorkflowBySlug(context.Background(), workspace.ID, present.Workflow.Slug); err != nil {
		t.Fatalf("present active workflow lookup: %v", err)
	}
	if _, err := db.GetActiveWorkflowBySlug(context.Background(), workspace.ID, activeRun.Workflow.Slug); err != nil {
		t.Fatalf("active-run workflow lookup: %v", err)
	}
	if _, err := db.GetActiveWorkflowBySlug(
		context.Background(),
		otherWorkspace.ID,
		otherWorkspaceStale.Workflow.Slug,
	); err != nil {
		t.Fatalf("other workspace workflow lookup: %v", err)
	}
	if _, err := db.GetLatestArchivedWorkflowBySlug(
		context.Background(),
		workspace.ID,
		archived.Workflow.Slug,
	); err != nil {
		t.Fatalf("archived workflow lookup: %v", err)
	}
	if _, err := db.GetActiveWorkflowBySlug(
		context.Background(),
		workspace.ID,
		stale.Workflow.Slug,
	); !errors.Is(
		err,
		ErrWorkflowNotFound,
	) {
		t.Fatalf("stale active lookup error = %v, want ErrWorkflowNotFound", err)
	}

	assertRowCountByWorkflow(t, db, "artifact_snapshots", stale.Workflow.ID, 0)
	assertRowCountByWorkflow(t, db, "task_items", stale.Workflow.ID, 0)
	assertRowCountByWorkflow(t, db, "review_rounds", stale.Workflow.ID, 0)
	assertRowCountByWorkflow(t, db, "sync_checkpoints", stale.Workflow.ID, 0)
	if got := queryTableRowCount(t, db, "review_issues", "round_id = ?", staleRoundID); got != 0 {
		t.Fatalf("review_issues for stale round = %d, want 0", got)
	}

	var terminalWorkflowID string
	if err := db.db.QueryRowContext(
		context.Background(),
		`SELECT COALESCE(workflow_id, '') FROM runs WHERE run_id = ?`,
		"run-stale-terminal",
	).Scan(&terminalWorkflowID); err != nil {
		t.Fatalf("query terminal run workflow id: %v", err)
	}
	if terminalWorkflowID != "" {
		t.Fatalf("terminal run workflow_id = %q, want empty after ON DELETE SET NULL", terminalWorkflowID)
	}
}

func TestReconcileWorkflowSyncStoresOversizedBodiesInDeduplicatedTable(t *testing.T) {
	t.Parallel()

	t.Run("Should store oversized bodies in the deduplicated body table", func(t *testing.T) {
		t.Parallel()

		db := openTestGlobalDB(t)
		defer func() {
			_ = db.Close()
		}()

		workspace := registerSyncTestWorkspace(t, db)
		body := strings.Repeat("x", artifactBodyLimitBytes+1024)

		result, err := db.ReconcileWorkflowSync(context.Background(), WorkflowSyncInput{
			WorkspaceID:        workspace.ID,
			WorkflowSlug:       "overflow",
			SyncedAt:           time.Date(2026, 4, 17, 22, 0, 0, 0, time.UTC),
			CheckpointChecksum: "overflow-checksum",
			ArtifactSnapshots: []ArtifactSnapshotInput{
				{
					ArtifactKind:    "qa",
					RelativePath:    "qa/verification-report.md",
					Checksum:        "body-overflow-checksum",
					FrontmatterJSON: `{}`,
					BodyText:        body,
					SourceMTime:     time.Date(2026, 4, 17, 21, 59, 0, 0, time.UTC),
				},
			},
		})
		if err != nil {
			t.Fatalf("ReconcileWorkflowSync(): %v", err)
		}

		var (
			bodyStorageKind string
			bodyText        string
		)
		if err := db.db.QueryRowContext(
			context.Background(),
			`SELECT body_storage_kind, COALESCE(body_text, '')
			 FROM artifact_snapshots
			 WHERE workflow_id = ? AND relative_path = ?`,
			result.Workflow.ID,
			"qa/verification-report.md",
		).Scan(&bodyStorageKind, &bodyText); err != nil {
			t.Fatalf("query oversized snapshot: %v", err)
		}
		if bodyStorageKind != artifactBodyBlobKind {
			t.Fatalf("body_storage_kind = %q, want %q", bodyStorageKind, artifactBodyBlobKind)
		}
		if bodyText != "" {
			t.Fatalf("snapshot body_text = %q, want empty inline body", bodyText)
		}

		var (
			storedBody string
			sizeBytes  int
		)
		if err := db.db.QueryRowContext(
			context.Background(),
			`SELECT body_text, size_bytes
			 FROM artifact_bodies
			 WHERE checksum = ?`,
			"body-overflow-checksum",
		).Scan(&storedBody, &sizeBytes); err != nil {
			t.Fatalf("query artifact body: %v", err)
		}
		if storedBody != body {
			t.Fatalf("artifact body length = %d, want %d", len(storedBody), len(body))
		}
		if sizeBytes != len([]byte(body)) {
			t.Fatalf("artifact body size_bytes = %d, want %d", sizeBytes, len([]byte(body)))
		}

		snapshots, err := db.ListArtifactSnapshots(context.Background(), result.Workflow.ID)
		if err != nil {
			t.Fatalf("ListArtifactSnapshots(): %v", err)
		}
		if len(snapshots) != 1 {
			t.Fatalf("ListArtifactSnapshots() count = %d, want 1", len(snapshots))
		}
		if snapshots[0].BodyText != body {
			t.Fatalf("ListArtifactSnapshots() body length = %d, want %d", len(snapshots[0].BodyText), len(body))
		}
	})
}

func TestWorkflowSyncHelperValidationAndNormalization(t *testing.T) {
	t.Parallel()

	t.Run("validate workflow sync input", func(t *testing.T) {
		t.Parallel()

		if err := validateWorkflowSyncInput(WorkflowSyncInput{}); err == nil {
			t.Fatal("expected missing workflow sync input to fail validation")
		}
		if err := validateWorkflowSyncInput(WorkflowSyncInput{WorkspaceID: "ws-1"}); err == nil {
			t.Fatal("expected missing workflow slug to fail validation")
		}
		if err := validateWorkflowSyncInput(WorkflowSyncInput{
			WorkspaceID:  "ws-1",
			WorkflowSlug: "demo",
		}); err != nil {
			t.Fatalf("validateWorkflowSyncInput(valid) error = %v", err)
		}
	})

	t.Run("prepare artifact snapshot", func(t *testing.T) {
		t.Parallel()

		prepared, key, err := prepareArtifactSnapshot(ArtifactSnapshotInput{
			ArtifactKind: "task",
			RelativePath: "task_01.md",
			Checksum:     "checksum-1",
			BodyText:     strings.Repeat("x", artifactBodyLimitBytes+1),
			SourceMTime:  time.Date(2026, 4, 17, 23, 0, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("prepareArtifactSnapshot(valid) error = %v", err)
		}
		if prepared.FrontmatterJSON != "{}" {
			t.Fatalf("FrontmatterJSON = %q, want {}", prepared.FrontmatterJSON)
		}
		if prepared.BodyStorageKind != artifactBodyBlobKind {
			t.Fatalf("BodyStorageKind = %q, want %q", prepared.BodyStorageKind, artifactBodyBlobKind)
		}
		if prepared.BodyText != "" || prepared.BodyBlobText == "" {
			t.Fatalf(
				"prepared body fields = inline %q blob length %d, want blob-only body",
				prepared.BodyText,
				len(prepared.BodyBlobText),
			)
		}
		if key != artifactKey("task", "task_01.md") {
			t.Fatalf("artifact key = %q, want %q", key, artifactKey("task", "task_01.md"))
		}

		cases := []ArtifactSnapshotInput{
			{},
			{ArtifactKind: "task"},
			{ArtifactKind: "task", RelativePath: "task_01.md"},
			{ArtifactKind: "task", RelativePath: "task_01.md", Checksum: "checksum-1"},
		}
		for _, tc := range cases {
			if _, _, err := prepareArtifactSnapshot(tc); err == nil {
				t.Fatalf("expected invalid artifact snapshot %#v to fail validation", tc)
			}
		}
	})

	t.Run("prepare task item", func(t *testing.T) {
		t.Parallel()

		prepared, err := prepareTaskItem(TaskItemInput{
			TaskNumber: 1,
			TaskID:     " task_1 ",
			Title:      " Demo ",
			Status:     " Completed ",
			Kind:       "backend",
			SourcePath: "task_01.md",
		})
		if err != nil {
			t.Fatalf("prepareTaskItem(valid) error = %v", err)
		}
		if prepared.TaskID != "task_1" || prepared.Status != "completed" || prepared.Title != "Demo" {
			t.Fatalf("unexpected prepared task item: %#v", prepared)
		}
		cases := []TaskItemInput{
			{},
			{TaskNumber: 1},
			{TaskNumber: 1, TaskID: "task_1"},
			{TaskNumber: 1, TaskID: "task_1", Title: "Demo"},
			{TaskNumber: 1, TaskID: "task_1", Title: "Demo", Status: "pending"},
			{TaskNumber: 1, TaskID: "task_1", Title: "Demo", Status: "pending", Kind: "backend"},
		}
		for _, tc := range cases {
			if _, err := prepareTaskItem(tc); err == nil {
				t.Fatalf("expected invalid task item %#v to fail validation", tc)
			} else if !errors.Is(err, ErrWorkflowSyncInvalid) {
				t.Fatalf("expected sync validation error for %#v, got %v", tc, err)
			}
		}
	})

	t.Run("prepare review round", func(t *testing.T) {
		t.Parallel()

		prepared, err := prepareReviewRound(ReviewRoundInput{
			RoundNumber:     1,
			Provider:        " coderabbit ",
			ResolvedCount:   0,
			UnresolvedCount: 1,
		})
		if err != nil {
			t.Fatalf("prepareReviewRound(valid) error = %v", err)
		}
		if prepared.Provider != "coderabbit" {
			t.Fatalf("Provider = %q, want coderabbit", prepared.Provider)
		}
		withoutProvider, err := prepareReviewRound(ReviewRoundInput{
			RoundNumber:     2,
			ResolvedCount:   1,
			UnresolvedCount: 0,
		})
		if err != nil {
			t.Fatalf("prepareReviewRound(without provider) error = %v", err)
		}
		if withoutProvider.Provider != "" {
			t.Fatalf("Provider = %q, want empty", withoutProvider.Provider)
		}
		cases := []ReviewRoundInput{
			{},
			{RoundNumber: 1, ResolvedCount: -1},
			{RoundNumber: 1, Provider: "coderabbit", UnresolvedCount: -1},
		}
		for _, tc := range cases {
			if _, err := prepareReviewRound(tc); err == nil {
				t.Fatalf("expected invalid review round %#v to fail validation", tc)
			}
		}
	})

	t.Run("prepare review issue", func(t *testing.T) {
		t.Parallel()

		prepared, err := prepareReviewIssue(ReviewIssueInput{
			IssueNumber: 1,
			Severity:    " high ",
			Status:      " Pending ",
			SourcePath:  "reviews-001/issue_001.md",
		})
		if err != nil {
			t.Fatalf("prepareReviewIssue(valid) error = %v", err)
		}
		if prepared.Status != "pending" || prepared.Severity != "high" {
			t.Fatalf("unexpected prepared review issue: %#v", prepared)
		}
		cases := []ReviewIssueInput{
			{},
			{IssueNumber: 1},
			{IssueNumber: 1, Status: "pending"},
		}
		for _, tc := range cases {
			if _, err := prepareReviewIssue(tc); err == nil {
				t.Fatalf("expected invalid review issue %#v to fail validation", tc)
			}
		}
	})

	t.Run("misc helpers", func(t *testing.T) {
		t.Parallel()

		if left, right := splitArtifactKey("artifact-only"); left != "artifact-only" || right != "" {
			t.Fatalf("splitArtifactKey(no separator) = %q, %q", left, right)
		}
		if encoded, err := marshalJSONArray(nil); err != nil || encoded != "[]" {
			t.Fatalf("marshalJSONArray(nil) = %q, %v; want [], nil", encoded, err)
		}
	})
}

func TestReconcileWorkflowSyncRejectsDuplicateInputs(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspace := registerSyncTestWorkspace(t, db)
	baseInput := WorkflowSyncInput{
		WorkspaceID:        workspace.ID,
		WorkflowSlug:       "duplicates",
		SyncedAt:           time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC),
		CheckpointChecksum: "dup-checksum",
	}

	tests := []struct {
		name    string
		mutate  func(*WorkflowSyncInput)
		wantErr string
	}{
		{
			name: "duplicate artifact snapshots",
			mutate: func(input *WorkflowSyncInput) {
				input.ArtifactSnapshots = []ArtifactSnapshotInput{
					{
						ArtifactKind: "task",
						RelativePath: "task_01.md",
						Checksum:     "one",
						SourceMTime:  time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC),
					},
					{
						ArtifactKind: "task",
						RelativePath: "task_01.md",
						Checksum:     "two",
						SourceMTime:  time.Date(2026, 4, 18, 0, 1, 0, 0, time.UTC),
					},
				}
			},
			wantErr: "duplicate artifact snapshot",
		},
		{
			name: "duplicate task numbers",
			mutate: func(input *WorkflowSyncInput) {
				input.TaskItems = []TaskItemInput{
					{
						TaskNumber: 1,
						TaskID:     "task_1",
						Title:      "One",
						Status:     "pending",
						Kind:       "backend",
						SourcePath: "task_01.md",
					},
					{
						TaskNumber: 1,
						TaskID:     "task_1b",
						Title:      "Two",
						Status:     "pending",
						Kind:       "backend",
						SourcePath: "task_01b.md",
					},
				}
			},
			wantErr: "duplicate task number",
		},
		{
			name: "duplicate review rounds",
			mutate: func(input *WorkflowSyncInput) {
				input.ReviewRounds = []ReviewRoundInput{
					{RoundNumber: 1, Provider: "coderabbit", UnresolvedCount: 1},
					{RoundNumber: 1, Provider: "coderabbit", UnresolvedCount: 2},
				}
			},
			wantErr: "duplicate review round",
		},
		{
			name: "duplicate review issues",
			mutate: func(input *WorkflowSyncInput) {
				input.ReviewRounds = []ReviewRoundInput{
					{
						RoundNumber:     1,
						Provider:        "coderabbit",
						UnresolvedCount: 2,
						Issues: []ReviewIssueInput{
							{IssueNumber: 1, Status: "pending", SourcePath: "reviews-001/issue_001.md"},
							{IssueNumber: 1, Status: "pending", SourcePath: "reviews-001/issue_001-copy.md"},
						},
					},
				}
			},
			wantErr: "duplicate review issue",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			input := baseInput
			input.WorkflowSlug = strings.ReplaceAll(tc.name, " ", "-")
			tc.mutate(&input)
			_, err := db.ReconcileWorkflowSync(context.Background(), input)
			if err == nil {
				t.Fatalf("expected %s to fail", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestReconcileWorkflowSyncRejectsNilContext(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	var nilCtx context.Context
	_, err := db.ReconcileWorkflowSync(nilCtx, WorkflowSyncInput{
		WorkspaceID:  "ws-1",
		WorkflowSlug: "demo",
	})
	if err == nil {
		t.Fatal("expected nil context to fail")
	}
	if !strings.Contains(err.Error(), "context is required") {
		t.Fatalf("unexpected nil-context error: %v", err)
	}
}

func TestSyncRowLoaderHelpersReadExistingTaskAndReviewRows(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspace := registerSyncTestWorkspace(t, db)
	result, err := db.ReconcileWorkflowSync(context.Background(), WorkflowSyncInput{
		WorkspaceID:        workspace.ID,
		WorkflowSlug:       "loader-demo",
		SyncedAt:           time.Date(2026, 4, 18, 1, 0, 0, 0, time.UTC),
		CheckpointChecksum: "loader-checksum",
		TaskItems: []TaskItemInput{
			{
				TaskNumber: 1,
				TaskID:     "task_1",
				Title:      "Task one",
				Status:     "pending",
				Kind:       "backend",
				SourcePath: "task_01.md",
			},
		},
		ReviewRounds: []ReviewRoundInput{
			{
				RoundNumber:     1,
				Provider:        "coderabbit",
				UnresolvedCount: 1,
				Issues: []ReviewIssueInput{
					{IssueNumber: 1, Status: "pending", SourcePath: "reviews-001/issue_001.md"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("ReconcileWorkflowSync(): %v", err)
	}

	tx, err := db.db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx(): %v", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	taskIDs, err := loadExistingTaskItemIDs(context.Background(), tx, result.Workflow.ID)
	if err != nil {
		t.Fatalf("loadExistingTaskItemIDs(): %v", err)
	}
	if got := taskIDs[1]; got == "" {
		t.Fatalf("expected existing task row id for task 1, got %#v", taskIDs)
	}

	roundIDs, err := loadExistingReviewRoundIDs(context.Background(), tx, result.Workflow.ID)
	if err != nil {
		t.Fatalf("loadExistingReviewRoundIDs(): %v", err)
	}
	roundID := roundIDs[1]
	if roundID == "" {
		t.Fatalf("expected existing round id for round 1, got %#v", roundIDs)
	}

	issueIDs, err := loadExistingReviewIssueIDs(context.Background(), tx, roundID)
	if err != nil {
		t.Fatalf("loadExistingReviewIssueIDs(): %v", err)
	}
	if got := issueIDs[1]; got == "" {
		t.Fatalf("expected existing issue row id for issue 1, got %#v", issueIDs)
	}
}

func TestReconcileWorkflowRowTxKeepsStableWorkflowIdentity(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspace := registerSyncTestWorkspace(t, db)
	firstSync := time.Date(2026, 4, 18, 2, 0, 0, 0, time.UTC)
	secondSync := firstSync.Add(time.Hour)

	tx, err := db.db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx(first): %v", err)
	}
	workflow, err := db.reconcileWorkflowRowTx(context.Background(), tx, workspace.ID, "identity", firstSync)
	if err != nil {
		t.Fatalf("reconcileWorkflowRowTx(first): %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit(first): %v", err)
	}

	tx, err = db.db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx(second): %v", err)
	}
	updatedWorkflow, err := db.reconcileWorkflowRowTx(context.Background(), tx, workspace.ID, "identity", secondSync)
	if err != nil {
		t.Fatalf("reconcileWorkflowRowTx(second): %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit(second): %v", err)
	}

	if updatedWorkflow.ID != workflow.ID {
		t.Fatalf("workflow id changed across tx helper update: before=%q after=%q", workflow.ID, updatedWorkflow.ID)
	}
	if updatedWorkflow.LastSyncedAt == nil || !updatedWorkflow.LastSyncedAt.Equal(secondSync) {
		t.Fatalf("unexpected LastSyncedAt after update: %#v", updatedWorkflow.LastSyncedAt)
	}
}

func TestReconcileWorkflowSyncDefaultsScopeAndDeletesStaleTaskAndIssueRows(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspace := registerSyncTestWorkspace(t, db)
	firstResult, err := db.ReconcileWorkflowSync(context.Background(), WorkflowSyncInput{
		WorkspaceID:  workspace.ID,
		WorkflowSlug: "defaults-demo",
		TaskItems: []TaskItemInput{
			{
				TaskNumber: 1,
				TaskID:     "task_1",
				Title:      "Task one",
				Status:     "pending",
				Kind:       "backend",
				SourcePath: "task_01.md",
			},
		},
		ReviewRounds: []ReviewRoundInput{
			{
				RoundNumber:     1,
				Provider:        "coderabbit",
				UnresolvedCount: 1,
				Issues: []ReviewIssueInput{
					{IssueNumber: 1, Status: "pending", SourcePath: "reviews-001/issue_001.md"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("ReconcileWorkflowSync(first): %v", err)
	}
	if got := queryTableRowCount(
		t,
		db,
		"sync_checkpoints",
		"workflow_id = ? AND scope = ?",
		firstResult.Workflow.ID,
		defaultSyncScope,
	); got != 1 {
		t.Fatalf("default scope checkpoint count = %d, want 1", got)
	}

	tx, err := db.db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx(): %v", err)
	}

	if _, err := db.reconcileTaskItemsTx(
		context.Background(),
		tx,
		firstResult.Workflow.ID,
		nil,
		time.Date(2026, 4, 18, 3, 0, 0, 0, time.UTC),
	); err != nil {
		t.Fatalf("reconcileTaskItemsTx(delete stale): %v", err)
	}

	roundIDs, err := loadExistingReviewRoundIDs(context.Background(), tx, firstResult.Workflow.ID)
	if err != nil {
		t.Fatalf("loadExistingReviewRoundIDs(): %v", err)
	}
	if _, err := db.reconcileReviewIssuesTx(
		context.Background(),
		tx,
		roundIDs[1],
		nil,
		time.Date(2026, 4, 18, 3, 0, 0, 0, time.UTC),
	); err != nil {
		t.Fatalf("reconcileReviewIssuesTx(delete stale): %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit(): %v", err)
	}

	if got := queryTableRowCount(t, db, "task_items", "workflow_id = ?", firstResult.Workflow.ID); got != 0 {
		t.Fatalf("task_items count after stale delete = %d, want 0", got)
	}
	if got := queryTableRowCount(t, db, "review_issues", "round_id = ?", roundIDs[1]); got != 0 {
		t.Fatalf("review_issues count after stale delete = %d, want 0", got)
	}
}

func registerSyncTestWorkspace(t *testing.T, db *GlobalDB) Workspace {
	t.Helper()

	workspaceRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy"), 0o755); err != nil {
		t.Fatalf("mkdir workspace marker: %v", err)
	}
	workspace, err := db.Register(context.Background(), workspaceRoot, "sync-workspace")
	if err != nil {
		t.Fatalf("Register(): %v", err)
	}
	return workspace
}

func assertRowCount(t *testing.T, db *GlobalDB, tableName string, want int) {
	t.Helper()

	var got int
	if err := db.db.QueryRowContext(
		context.Background(),
		fmt.Sprintf("SELECT COUNT(1) FROM %s", tableName),
	).Scan(&got); err != nil {
		t.Fatalf("count rows in %s: %v", tableName, err)
	}
	if got != want {
		t.Fatalf("%s row count = %d, want %d", tableName, got, want)
	}
}

func assertRowCountByWorkflow(t *testing.T, db *GlobalDB, tableName string, workflowID string, want int) {
	t.Helper()

	var got int
	if err := db.db.QueryRowContext(
		context.Background(),
		fmt.Sprintf("SELECT COUNT(1) FROM %s WHERE workflow_id = ?", tableName),
		workflowID,
	).Scan(&got); err != nil {
		t.Fatalf("count rows in %s for workflow %s: %v", tableName, workflowID, err)
	}
	if got != want {
		t.Fatalf("%s row count for workflow %s = %d, want %d", tableName, workflowID, got, want)
	}
}

func queryTableRowCount(t *testing.T, db *GlobalDB, tableName string, whereClause string, args ...any) int {
	t.Helper()

	var count int
	query := fmt.Sprintf("SELECT COUNT(1) FROM %s", tableName)
	if strings.TrimSpace(whereClause) != "" {
		query += " WHERE " + whereClause
	}
	if err := db.db.QueryRowContext(context.Background(), query, args...).Scan(&count); err != nil {
		t.Fatalf("query row count for %s: %v", tableName, err)
	}
	return count
}

func mustReconcilePruneWorkflow(
	t *testing.T,
	db *GlobalDB,
	workspaceID string,
	slug string,
	syncedAt time.Time,
) WorkflowSyncResult {
	t.Helper()

	result, err := db.ReconcileWorkflowSync(context.Background(), WorkflowSyncInput{
		WorkspaceID:        workspaceID,
		WorkflowSlug:       slug,
		SyncedAt:           syncedAt,
		CheckpointScope:    "workflow",
		CheckpointChecksum: slug + "-checkpoint",
		ArtifactSnapshots: []ArtifactSnapshotInput{
			{
				ArtifactKind:    "task",
				RelativePath:    "task_01.md",
				Checksum:        slug + "-task-checksum",
				FrontmatterJSON: `{"status":"completed"}`,
				BodyText:        "# Task 01",
				SourceMTime:     syncedAt,
			},
		},
		TaskItems: []TaskItemInput{
			{
				TaskNumber: 1,
				TaskID:     "task_1",
				Title:      slug + " task",
				Status:     "completed",
				Kind:       "backend",
				SourcePath: "task_01.md",
			},
		},
		ReviewRounds: []ReviewRoundInput{
			{
				RoundNumber:     1,
				Provider:        "coderabbit",
				PRRef:           "123",
				ResolvedCount:   1,
				UnresolvedCount: 0,
				Issues: []ReviewIssueInput{
					{
						IssueNumber: 1,
						Severity:    "medium",
						Status:      "resolved",
						SourcePath:  "reviews-001/issue_001.md",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("ReconcileWorkflowSync(%q): %v", slug, err)
	}
	return result
}

func equalStringSlices(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if left[idx] != right[idx] {
			return false
		}
	}
	return true
}
