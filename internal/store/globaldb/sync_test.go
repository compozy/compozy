package globaldb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func TestReconcileAggregateWorkflowSyncKeepsHierarchyAtomicAndStable(t *testing.T) {
	// Suite boundary
	// IN: aggregate parent/child reconciliation through the real SQLite transaction
	// OUT: filesystem plan discovery and daemon read models
	// Invariant: an initiative owns exactly one active child per package across retries and partial scans.
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()
	workspace := registerSyncTestWorkspace(t, db)
	syncedAt := time.Date(2026, 4, 18, 9, 0, 0, 0, time.UTC)
	parent := WorkflowSyncInput{
		WorkspaceID:        workspace.ID,
		WorkflowSlug:       "initiative",
		Kind:               WorkflowKindInitiative,
		DisplayTitle:       "Initiative",
		SyncedAt:           syncedAt,
		CheckpointChecksum: "initiative-checkpoint",
	}
	child := func(packageID string, completed bool) WorkflowSyncInput {
		status := "pending"
		if completed {
			status = "completed"
		}
		dependencies := []WorkflowDependency(nil)
		if packageID == "WP-002" {
			dependencies = []WorkflowDependency{{
				PackageID: "WP-001",
				Rationale: "Consumes the persisted contract.",
			}}
		}
		return WorkflowSyncInput{
			WorkspaceID:        workspace.ID,
			WorkflowSlug:       "initiative/" + packageID,
			Kind:               WorkflowKindWorkPackage,
			PackageID:          packageID,
			DisplayTitle:       "Package " + packageID,
			Outcome:            "Deliver " + packageID,
			LifecycleCompleted: completed,
			Dependencies:       dependencies,
			SyncedAt:           syncedAt,
			CheckpointChecksum: packageID + "-checkpoint",
			TaskItems: []TaskItemInput{{
				TaskNumber: 1,
				TaskID:     packageID + "-task",
				Title:      packageID + " task",
				Status:     status,
				Kind:       "backend",
				SourcePath: "task_01.md",
			}},
		}
	}

	first, err := db.ReconcileAggregateWorkflowSync(context.Background(), AggregateWorkflowSyncInput{
		Parent: parent,
		Children: []WorkflowSyncInput{
			child("WP-001", true),
			child("WP-002", false),
		},
	})
	if err != nil {
		t.Fatalf("ReconcileAggregateWorkflowSync(first): %v", err)
	}
	if first.Parent.Workflow.Kind != WorkflowKindInitiative || len(first.Children) != 2 {
		t.Fatalf("first aggregate result = %#v, want initiative parent and two children", first)
	}
	firstIDs := map[string]string{}
	for _, result := range first.Children {
		childWorkflow := result.Workflow
		if childWorkflow.ParentWorkflowID != first.Parent.Workflow.ID || childWorkflow.Kind != WorkflowKindWorkPackage {
			t.Fatalf("child hierarchy = %#v, want parent %q", childWorkflow, first.Parent.Workflow.ID)
		}
		firstIDs[childWorkflow.PackageID] = childWorkflow.ID
	}
	wp002 := first.Children[1].Workflow
	if wp002.DisplayTitle != "Package WP-002" || wp002.Outcome != "Deliver WP-002" ||
		len(wp002.Dependencies) != 1 || wp002.Dependencies[0].PackageID != "WP-001" {
		t.Fatalf("WP-002 metadata projection = %#v", wp002)
	}

	parent.SyncedAt = syncedAt.Add(time.Minute)
	second, err := db.ReconcileAggregateWorkflowSync(context.Background(), AggregateWorkflowSyncInput{
		Parent: parent,
		Children: []WorkflowSyncInput{
			child("WP-001", true),
			child("WP-002", false),
		},
	})
	if err != nil {
		t.Fatalf("ReconcileAggregateWorkflowSync(retry): %v", err)
	}
	for _, result := range second.Children {
		if got, want := result.Workflow.ID, firstIDs[result.Workflow.PackageID]; got != want {
			t.Fatalf("child %s id = %q, want stable %q", result.Workflow.PackageID, got, want)
		}
	}

	partial, err := db.ReconcileAggregateWorkflowSync(context.Background(), AggregateWorkflowSyncInput{
		Parent:                  parent,
		Children:                []WorkflowSyncInput{child("WP-001", true)},
		PreserveMissingChildren: true,
	})
	if err != nil {
		t.Fatalf("ReconcileAggregateWorkflowSync(partial): %v", err)
	}
	if len(partial.PrunedChildPackageIDs) != 0 {
		t.Fatalf("partial sync pruned children: %#v", partial.PrunedChildPackageIDs)
	}
	children, err := db.ListChildWorkflows(context.Background(), first.Parent.Workflow.ID, false)
	if err != nil {
		t.Fatalf("ListChildWorkflows(): %v", err)
	}
	if len(children) != 2 || children[1].ID != firstIDs["WP-002"] {
		t.Fatalf("children after partial sync = %#v, want preserved WP-002", children)
	}

	_, err = db.ReconcileAggregateWorkflowSync(context.Background(), AggregateWorkflowSyncInput{
		Parent: WorkflowSyncInput{
			WorkspaceID:  workspace.ID,
			WorkflowSlug: "interrupted",
			Kind:         WorkflowKindInitiative,
			SyncedAt:     syncedAt,
		},
		Children: []WorkflowSyncInput{{
			WorkspaceID:  workspace.ID,
			WorkflowSlug: "interrupted/WP-001",
			Kind:         WorkflowKindWorkPackage,
			PackageID:    "WP-001",
			SyncedAt:     syncedAt,
			TaskItems: []TaskItemInput{{
				TaskNumber: 1,
			}},
		}},
	})
	if err == nil {
		t.Fatal("ReconcileAggregateWorkflowSync(interrupted) error = nil, want rollback")
	}
	if got := queryTableRowCount(t, db, "workflows", "slug LIKE ?", "interrupted%"); got != 0 {
		t.Fatalf("interrupted aggregate left %d workflow rows, want 0", got)
	}
}

func TestReconcileWorkflowSyncRetriesDemotedChildPruneAfterRunCompletes(t *testing.T) {
	// Suite boundary
	// IN: standalone ordinary reconciliation of a formerly-initiative slug through real SQLite
	// OUT: daemon child projection and later initiative recreation
	// Invariant: an ordinary workflow never permanently strands a package child that was
	// skipped by a demotion prune while its run was still active.
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()
	workspace := registerSyncTestWorkspace(t, db)
	ctx := context.Background()
	syncedAt := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)

	const (
		initiativeSlug = "demoted-initiative"
		childSlug      = "demoted-initiative/WP-001"
		packageID      = "WP-001"
	)
	initiative := func(at time.Time) WorkflowSyncInput {
		return WorkflowSyncInput{
			WorkspaceID:  workspace.ID,
			WorkflowSlug: initiativeSlug,
			Kind:         WorkflowKindInitiative,
			DisplayTitle: "Initiative",
			SyncedAt:     at,
		}
	}
	childInput := func(at time.Time) WorkflowSyncInput {
		return WorkflowSyncInput{
			WorkspaceID:  workspace.ID,
			WorkflowSlug: childSlug,
			Kind:         WorkflowKindWorkPackage,
			PackageID:    packageID,
			DisplayTitle: "Package WP-001",
			Outcome:      "Deliver WP-001",
			SyncedAt:     at,
		}
	}
	ordinary := func(at time.Time) WorkflowSyncInput {
		return WorkflowSyncInput{
			WorkspaceID:  workspace.ID,
			WorkflowSlug: initiativeSlug,
			Kind:         WorkflowKindOrdinary,
			DisplayTitle: "Initiative",
			SyncedAt:     at,
		}
	}

	created, err := db.ReconcileAggregateWorkflowSync(ctx, AggregateWorkflowSyncInput{
		Parent:   initiative(syncedAt),
		Children: []WorkflowSyncInput{childInput(syncedAt)},
	})
	if err != nil {
		t.Fatalf("ReconcileAggregateWorkflowSync(create): %v", err)
	}
	parentID := created.Parent.Workflow.ID
	childID := created.Children[0].Workflow.ID

	// An in-flight run on the child keeps the demotion prune from removing it.
	if _, err := db.PutRun(ctx, Run{
		RunID:            "run-child-active",
		WorkspaceID:      workspace.ID,
		WorkflowID:       &childID,
		Mode:             "task",
		Status:           "running",
		PresentationMode: "stream",
		StartedAt:        syncedAt,
	}); err != nil {
		t.Fatalf("PutRun(active child): %v", err)
	}

	// Demote the initiative to ordinary while the child run is still active.
	demoteAt := syncedAt.Add(time.Minute)
	demoted, err := db.ReconcileWorkflowSync(ctx, ordinary(demoteAt))
	if err != nil {
		t.Fatalf("ReconcileWorkflowSync(demote): %v", err)
	}
	if demoted.Workflow.ID != parentID || demoted.Workflow.Kind != WorkflowKindOrdinary {
		t.Fatalf("demoted workflow = %#v, want ordinary reuse of %q", demoted.Workflow, parentID)
	}
	retained, err := db.ListChildWorkflows(ctx, parentID, false)
	if err != nil {
		t.Fatalf("ListChildWorkflows(after demote): %v", err)
	}
	if len(retained) != 1 || retained[0].ID != childID {
		t.Fatalf("children after demotion = %#v, want retained active child %q", retained, childID)
	}

	// Finish the run and reconcile the ordinary workflow again. The prune must
	// retry now that the child is idle instead of stranding it forever.
	endedAt := demoteAt.Add(time.Minute)
	if _, err := db.UpdateRun(ctx, Run{
		RunID:            "run-child-active",
		WorkspaceID:      workspace.ID,
		WorkflowID:       &childID,
		Mode:             "task",
		Status:           "completed",
		PresentationMode: "stream",
		StartedAt:        syncedAt,
		EndedAt:          &endedAt,
	}); err != nil {
		t.Fatalf("UpdateRun(complete child): %v", err)
	}
	resyncAt := endedAt.Add(time.Minute)
	if _, err := db.ReconcileWorkflowSync(ctx, ordinary(resyncAt)); err != nil {
		t.Fatalf("ReconcileWorkflowSync(resync): %v", err)
	}
	drained, err := db.ListChildWorkflows(ctx, parentID, false)
	if err != nil {
		t.Fatalf("ListChildWorkflows(after resync): %v", err)
	}
	if len(drained) != 0 {
		t.Fatalf("children after ordinary resync = %#v, want none", drained)
	}
	if got := queryTableRowCount(t, db, "workflows", "slug = ? AND archived_at IS NULL", childSlug); got != 0 {
		t.Fatalf("active child rows for %q = %d, want 0", childSlug, got)
	}

	// The initiative hierarchy recreates cleanly: exactly one active child under
	// the reused parent with correct parent/package metadata.
	recreateAt := resyncAt.Add(time.Minute)
	recreated, err := db.ReconcileAggregateWorkflowSync(ctx, AggregateWorkflowSyncInput{
		Parent:   initiative(recreateAt),
		Children: []WorkflowSyncInput{childInput(recreateAt)},
	})
	if err != nil {
		t.Fatalf("ReconcileAggregateWorkflowSync(recreate): %v", err)
	}
	if recreated.Parent.Workflow.ID != parentID || recreated.Parent.Workflow.Kind != WorkflowKindInitiative {
		t.Fatalf("recreated parent = %#v, want promoted reuse of %q", recreated.Parent.Workflow, parentID)
	}
	if len(recreated.Children) != 1 {
		t.Fatalf("recreated children = %#v, want exactly one", recreated.Children)
	}
	recreatedChild := recreated.Children[0].Workflow
	if recreatedChild.ParentWorkflowID != parentID ||
		recreatedChild.PackageID != packageID ||
		recreatedChild.Kind != WorkflowKindWorkPackage {
		t.Fatalf("recreated child = %#v, want work package under %q", recreatedChild, parentID)
	}
	finalChildren, err := db.ListChildWorkflows(ctx, parentID, false)
	if err != nil {
		t.Fatalf("ListChildWorkflows(after recreate): %v", err)
	}
	if len(finalChildren) != 1 || finalChildren[0].PackageID != packageID {
		t.Fatalf("children after recreate = %#v, want single %q child", finalChildren, packageID)
	}
	if got := queryTableRowCount(
		t, db, "workflows", "parent_workflow_id = ? AND archived_at IS NULL", parentID,
	); got != 1 {
		t.Fatalf("active child rows under %q = %d, want 1", parentID, got)
	}
}

func TestReconcileAggregateWorkflowSyncCollapsesConcurrentRetries(t *testing.T) {
	// Suite boundary
	// IN: concurrent callers against the same real SQLite global catalog
	// OUT: daemon scheduling and package run ownership
	// Invariant: retries leave one active initiative and one active child for each declared package.
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()
	workspace := registerSyncTestWorkspace(t, db)
	syncedAt := time.Date(2026, 4, 18, 9, 30, 0, 0, time.UTC)
	input := AggregateWorkflowSyncInput{
		Parent: WorkflowSyncInput{
			WorkspaceID:  workspace.ID,
			WorkflowSlug: "concurrent-initiative",
			Kind:         WorkflowKindInitiative,
			SyncedAt:     syncedAt,
		},
		Children: []WorkflowSyncInput{
			{
				WorkspaceID:  workspace.ID,
				WorkflowSlug: "concurrent-initiative/WP-001",
				Kind:         WorkflowKindWorkPackage,
				PackageID:    "WP-001",
				SyncedAt:     syncedAt,
			},
			{
				WorkspaceID:  workspace.ID,
				WorkflowSlug: "concurrent-initiative/WP-002",
				Kind:         WorkflowKindWorkPackage,
				PackageID:    "WP-002",
				SyncedAt:     syncedAt,
			},
		},
	}

	const attempts = 8
	start := make(chan struct{})
	errs := make(chan error, attempts)
	var workers sync.WaitGroup
	for range attempts {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			_, err := db.ReconcileAggregateWorkflowSync(context.Background(), input)
			errs <- err
		}()
	}
	close(start)
	workers.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent ReconcileAggregateWorkflowSync() error = %v", err)
		}
	}

	parent, err := db.GetActiveWorkflowBySlug(context.Background(), workspace.ID, "concurrent-initiative")
	if err != nil {
		t.Fatalf("GetActiveWorkflowBySlug(parent): %v", err)
	}
	children, err := db.ListChildWorkflows(context.Background(), parent.ID, false)
	if err != nil {
		t.Fatalf("ListChildWorkflows(): %v", err)
	}
	if len(children) != 2 || children[0].PackageID != "WP-001" || children[1].PackageID != "WP-002" {
		t.Fatalf("concurrent child hierarchy = %#v, want WP-001 and WP-002 once", children)
	}
	if got := queryTableRowCount(
		t,
		db,
		"workflows",
		"workspace_id = ? AND slug = ? AND archived_at IS NULL",
		workspace.ID,
		parent.Slug,
	); got != 1 {
		t.Fatalf("active parent rows = %d, want 1", got)
	}
	if got := queryTableRowCount(
		t,
		db,
		"workflows",
		"parent_workflow_id = ? AND archived_at IS NULL",
		parent.ID,
	); got != 2 {
		t.Fatalf("active child rows = %d, want 2", got)
	}
}

func TestReconcileAggregateWorkflowSyncRetainsLargePackagePlansAcrossRetries(t *testing.T) {
	// Suite boundary
	// IN: a 300-child aggregate transaction and deterministic retry
	// OUT: filesystem manifest parsing and user-facing pagination
	// Invariant: package cardinality and child identities are not truncated by aggregate persistence.
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()
	workspace := registerSyncTestWorkspace(t, db)
	syncedAt := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	children := make([]WorkflowSyncInput, 0, 300)
	for number := 1; number <= 300; number++ {
		packageID := fmt.Sprintf("WP-%03d", number)
		children = append(children, WorkflowSyncInput{
			WorkspaceID:  workspace.ID,
			WorkflowSlug: "large-initiative/" + packageID,
			Kind:         WorkflowKindWorkPackage,
			PackageID:    packageID,
			SyncedAt:     syncedAt,
		})
	}
	input := AggregateWorkflowSyncInput{
		Parent: WorkflowSyncInput{
			WorkspaceID:  workspace.ID,
			WorkflowSlug: "large-initiative",
			Kind:         WorkflowKindInitiative,
			SyncedAt:     syncedAt,
		},
		Children: children,
	}
	first, err := db.ReconcileAggregateWorkflowSync(context.Background(), input)
	if err != nil {
		t.Fatalf("ReconcileAggregateWorkflowSync(large first): %v", err)
	}
	if len(first.Children) != 300 {
		t.Fatalf("large first child count = %d, want 300", len(first.Children))
	}
	firstIDs := make(map[string]string, len(first.Children))
	for _, child := range first.Children {
		firstIDs[child.Workflow.PackageID] = child.Workflow.ID
	}
	input.Parent.SyncedAt = syncedAt.Add(time.Minute)
	second, err := db.ReconcileAggregateWorkflowSync(context.Background(), input)
	if err != nil {
		t.Fatalf("ReconcileAggregateWorkflowSync(large retry): %v", err)
	}
	if len(second.Children) != 300 || len(second.PrunedChildPackageIDs) != 0 {
		t.Fatalf("large retry = %#v, want 300 stable children without prunes", second)
	}
	for _, child := range second.Children {
		if got, want := child.Workflow.ID, firstIDs[child.Workflow.PackageID]; got != want {
			t.Fatalf("large child %s id = %q, want %q", child.Workflow.PackageID, got, want)
		}
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

func TestPruneMissingActiveWorkflowsHandlesInitiativeChildrenAsAnAggregate(t *testing.T) {
	t.Parallel()

	const initiativeSlug = "missing-initiative"
	testCases := []struct {
		name              string
		addActiveChildRun bool
		wantPruned        bool
		wantActiveRuns    int
	}{
		{
			name:       "deletes missing initiative and children without active runs",
			wantPruned: true,
		},
		{
			name:              "keeps initiative and children when a child run is active",
			addActiveChildRun: true,
			wantActiveRuns:    1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db := openTestGlobalDB(t)
			defer func() {
				_ = db.Close()
			}()

			workspace := registerSyncTestWorkspace(t, db)
			syncedAt := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
			aggregate, err := db.ReconcileAggregateWorkflowSync(context.Background(), AggregateWorkflowSyncInput{
				Parent: WorkflowSyncInput{
					WorkspaceID:  workspace.ID,
					WorkflowSlug: initiativeSlug,
					Kind:         WorkflowKindInitiative,
					SyncedAt:     syncedAt,
				},
				Children: []WorkflowSyncInput{
					{
						WorkspaceID:  workspace.ID,
						WorkflowSlug: initiativeSlug + "/WP-001",
						Kind:         WorkflowKindWorkPackage,
						PackageID:    "WP-001",
						SyncedAt:     syncedAt,
					},
					{
						WorkspaceID:  workspace.ID,
						WorkflowSlug: initiativeSlug + "/WP-002",
						Kind:         WorkflowKindWorkPackage,
						PackageID:    "WP-002",
						SyncedAt:     syncedAt,
					},
				},
			})
			if err != nil {
				t.Fatalf("ReconcileAggregateWorkflowSync(): %v", err)
			}

			if tc.addActiveChildRun {
				childID := aggregate.Children[0].Workflow.ID
				if _, err := db.PutRun(context.Background(), Run{
					RunID:            "active-child-run",
					WorkspaceID:      workspace.ID,
					WorkflowID:       &childID,
					Mode:             "task",
					Status:           "running",
					PresentationMode: "stream",
					StartedAt:        syncedAt,
				}); err != nil {
					t.Fatalf("PutRun(active child): %v", err)
				}
			}

			result, err := db.PruneMissingActiveWorkflows(context.Background(), workspace.ID, nil)
			if err != nil {
				t.Fatalf("PruneMissingActiveWorkflows(): %v", err)
			}

			if tc.wantPruned {
				if !equalStringSlices(result.PrunedSlugs, []string{initiativeSlug}) {
					t.Fatalf("PrunedSlugs = %#v, want [%s]", result.PrunedSlugs, initiativeSlug)
				}
				if got := queryTableRowCount(t, db, "workflows", "workspace_id = ?", workspace.ID); got != 0 {
					t.Fatalf("remaining aggregate workflow rows = %d, want 0", got)
				}
				return
			}

			if len(result.PrunedSlugs) != 0 {
				t.Fatalf("PrunedSlugs = %#v, want none", result.PrunedSlugs)
			}
			if len(result.Skipped) != 1 || result.Skipped[0].Slug != initiativeSlug ||
				result.Skipped[0].Reason != archiveReasonActiveRuns || result.Skipped[0].ActiveRuns != tc.wantActiveRuns {
				t.Fatalf("Skipped = %#v, want active aggregate skip", result.Skipped)
			}
			if got := queryTableRowCount(t, db, "workflows", "workspace_id = ?", workspace.ID); got != 3 {
				t.Fatalf("remaining aggregate workflow rows = %d, want 3", got)
			}
		})
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

func TestWorkflowPruneActiveRunSkip(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		activeRuns int
		wantSkip   bool
	}{
		{
			name:       "Should report a skip when active runs remain",
			activeRuns: 2,
			wantSkip:   true,
		},
		{
			name:       "Should ignore zero-active-run delete misses",
			activeRuns: 0,
			wantSkip:   false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			skipped, ok := workflowPruneActiveRunSkip("stale-workflow", tc.activeRuns)
			if ok != tc.wantSkip {
				t.Fatalf("workflowPruneActiveRunSkip() ok = %v, want %v", ok, tc.wantSkip)
			}
			if !tc.wantSkip {
				if skipped != (WorkflowPruneSkipped{}) {
					t.Fatalf("workflowPruneActiveRunSkip() = %#v, want zero value", skipped)
				}
				return
			}
			if skipped.Slug != "stale-workflow" || skipped.Reason != archiveReasonActiveRuns ||
				skipped.ActiveRuns != tc.activeRuns {
				t.Fatalf("workflowPruneActiveRunSkip() = %#v, want active-run skip", skipped)
			}
		})
	}
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
