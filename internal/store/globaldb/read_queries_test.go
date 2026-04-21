package globaldb

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReadQueriesListStructuredWorkflowRows(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("close test global db: %v", err)
		}
	}()

	workspace := registerSyncTestWorkspace(t, db)
	syncedAt := time.Date(2026, 4, 20, 20, 0, 0, 0, time.UTC)

	result, err := db.ReconcileWorkflowSync(context.Background(), WorkflowSyncInput{
		WorkspaceID:        workspace.ID,
		WorkflowSlug:       "read-model-demo",
		SyncedAt:           syncedAt,
		CheckpointChecksum: "checkpoint-1",
		ArtifactSnapshots: []ArtifactSnapshotInput{
			{
				ArtifactKind:    "techspec",
				RelativePath:    "_techspec.md",
				Checksum:        "checksum-techspec",
				FrontmatterJSON: `{"title":"TechSpec"}`,
				BodyText:        "# TechSpec",
				SourceMTime:     syncedAt.Add(-time.Minute),
			},
			{
				ArtifactKind:    "memory",
				RelativePath:    "memory/MEMORY.md",
				Checksum:        "checksum-memory",
				FrontmatterJSON: `{}`,
				BodyText:        "# Memory",
				SourceMTime:     syncedAt.Add(-2 * time.Minute),
			},
		},
		TaskItems: []TaskItemInput{
			{
				TaskNumber: 1,
				TaskID:     "task_1",
				Title:      "Read query task 1",
				Status:     "pending",
				Kind:       "backend",
				DependsOn:  []string{"task_0"},
				SourcePath: "task_01.md",
			},
			{
				TaskNumber: 2,
				TaskID:     "task_2",
				Title:      "Read query task 2",
				Status:     "completed",
				Kind:       "backend",
				SourcePath: "task_02.md",
			},
		},
		ReviewRounds: []ReviewRoundInput{
			{
				RoundNumber:     1,
				Provider:        "coderabbit",
				PRRef:           "101",
				ResolvedCount:   0,
				UnresolvedCount: 1,
				Issues: []ReviewIssueInput{
					{IssueNumber: 1, Severity: "high", Status: "pending", SourcePath: "reviews-001/issue_001.md"},
				},
			},
			{
				RoundNumber:     2,
				Provider:        "coderabbit",
				PRRef:           "101",
				ResolvedCount:   1,
				UnresolvedCount: 0,
				Issues: []ReviewIssueInput{
					{IssueNumber: 1, Severity: "medium", Status: "resolved", SourcePath: "reviews-002/issue_001.md"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("ReconcileWorkflowSync() error = %v", err)
	}

	taskItems, err := db.ListTaskItems(context.Background(), result.Workflow.ID)
	if err != nil {
		t.Fatalf("ListTaskItems() error = %v", err)
	}
	if len(taskItems) != 2 {
		t.Fatalf("len(taskItems) = %d, want 2", len(taskItems))
	}
	if taskItems[0].TaskID != "task_1" || taskItems[0].DependsOn[0] != "task_0" {
		t.Fatalf("unexpected first task item: %#v", taskItems[0])
	}
	if taskItems[1].TaskNumber != 2 || taskItems[1].Status != "completed" {
		t.Fatalf("unexpected second task item: %#v", taskItems[1])
	}

	taskItem, err := db.GetTaskItemByTaskID(context.Background(), result.Workflow.ID, "task_2")
	if err != nil {
		t.Fatalf("GetTaskItemByTaskID() error = %v", err)
	}
	if taskItem.SourcePath != "task_02.md" || taskItem.Title != "Read query task 2" {
		t.Fatalf("unexpected task item lookup: %#v", taskItem)
	}

	snapshots, err := db.ListArtifactSnapshots(context.Background(), result.Workflow.ID)
	if err != nil {
		t.Fatalf("ListArtifactSnapshots() error = %v", err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("len(snapshots) = %d, want 2", len(snapshots))
	}
	if snapshots[0].ArtifactKind != "memory" || snapshots[1].ArtifactKind != "techspec" {
		t.Fatalf("unexpected artifact snapshot ordering: %#v", snapshots)
	}
	if snapshots[0].BodyText != "# Memory" || snapshots[1].FrontmatterJSON != `{"title":"TechSpec"}` {
		t.Fatalf("unexpected artifact snapshot payloads: %#v", snapshots)
	}

	rounds, err := db.ListReviewRounds(context.Background(), result.Workflow.ID)
	if err != nil {
		t.Fatalf("ListReviewRounds() error = %v", err)
	}
	if len(rounds) != 2 {
		t.Fatalf("len(rounds) = %d, want 2", len(rounds))
	}
	if rounds[0].RoundNumber != 1 || rounds[1].RoundNumber != 2 {
		t.Fatalf("unexpected round ordering: %#v", rounds)
	}
}

func TestReadQueriesHandleNotFoundAndInvalidJSONBranches(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("close test global db: %v", err)
		}
	}()

	workspace := registerSyncTestWorkspace(t, db)
	workflow, err := db.PutWorkflow(context.Background(), Workflow{
		WorkspaceID: workspace.ID,
		Slug:        "read-query-missing",
	})
	if err != nil {
		t.Fatalf("PutWorkflow() error = %v", err)
	}

	if _, err := db.GetTaskItemByTaskID(
		context.Background(),
		workflow.ID,
		"task_404",
	); !errors.Is(
		err,
		ErrTaskItemNotFound,
	) {
		t.Fatalf("GetTaskItemByTaskID(missing) error = %v, want ErrTaskItemNotFound", err)
	}

	if values, err := unmarshalJSONArray(`["task_1"," task_2 "]`); err != nil {
		t.Fatalf("unmarshalJSONArray(valid) error = %v", err)
	} else if len(values) != 2 || values[1] != "task_2" {
		t.Fatalf("unmarshalJSONArray(valid) = %#v, want trimmed values", values)
	}

	if _, err := unmarshalJSONArray(`{`); err == nil {
		t.Fatal("unmarshalJSONArray(invalid) error = nil, want non-nil")
	}
}
