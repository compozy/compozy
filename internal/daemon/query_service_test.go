package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/store/globaldb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

type stubDaemonStatusReader struct {
	status apicore.DaemonStatus
	health apicore.DaemonHealth
}

func (s stubDaemonStatusReader) Status(context.Context) (apicore.DaemonStatus, error) {
	return s.status, nil
}

func (s stubDaemonStatusReader) Health(context.Context) (apicore.DaemonHealth, error) {
	return s.health, nil
}

func TestDocumentReaderNormalizesFrontmatterAndInvalidatesCacheOnMTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(path, []byte(strings.Join([]string{
		"---",
		"title: Initial Title",
		"owner: daemon",
		"---",
		"",
		"# Ignored H1",
		"",
		"Initial body.",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("WriteFile(initial) error = %v", err)
	}

	reader := newDocumentReader()
	first, err := reader.Read(context.Background(), path, "techspec", "techspec")
	if err != nil {
		t.Fatalf("Read(initial) error = %v", err)
	}
	if first.Title != "Initial Title" {
		t.Fatalf("first.Title = %q, want Initial Title", first.Title)
	}
	if strings.Contains(first.Markdown, "owner: daemon") {
		t.Fatalf("first.Markdown unexpectedly contains front matter:\n%s", first.Markdown)
	}
	if got := metadataString(first.Metadata, "owner"); got != "daemon" {
		t.Fatalf("metadata owner = %q, want daemon", got)
	}

	if err := os.WriteFile(path, []byte("# Updated Title\n\nUpdated body.\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(updated) error = %v", err)
	}
	updatedAt := time.Now().Add(2 * time.Second).UTC()
	if err := os.Chtimes(path, updatedAt, updatedAt); err != nil {
		t.Fatalf("Chtimes(updated) error = %v", err)
	}

	second, err := reader.Read(context.Background(), path, "techspec", "techspec")
	if err != nil {
		t.Fatalf("Read(updated) error = %v", err)
	}
	if second.Title != "Updated Title" {
		t.Fatalf("second.Title = %q, want Updated Title", second.Title)
	}
	if strings.Contains(second.Markdown, "Initial body.") {
		t.Fatalf("second.Markdown still contains initial body:\n%s", second.Markdown)
	}
}

func TestMemoryFileIDStableAndOpaque(t *testing.T) {
	first := memoryFileID("ws-1", "demo", "memory/task_01.md")
	second := memoryFileID("ws-1", "demo", "memory/task_01.md")
	other := memoryFileID("ws-1", "demo", "memory/task_02.md")

	if first != second {
		t.Fatalf("memoryFileID() stability mismatch: %q != %q", first, second)
	}
	if first == other {
		t.Fatalf("memoryFileID() collision: %q == %q", first, other)
	}
	if !strings.HasPrefix(first, "mem_") {
		t.Fatalf("memoryFileID() = %q, want mem_ prefix", first)
	}
	if strings.Contains(first, "task_01.md") || strings.Contains(first, "memory/task_01.md") {
		t.Fatalf("memoryFileID() leaked path content: %q", first)
	}
}

func TestQueryServiceAssemblesReadModelsFromRealDaemonState(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(ctx context.Context, prep *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			runArtifacts, err := model.ResolveHomeRunArtifacts(cfg.RunID)
			if err != nil {
				return err
			}

			submitEvent(ctx, t, prep.Journal(), cfg.RunID, eventspkg.EventKindJobQueued, kinds.JobQueuedPayload{
				Index:           1,
				SafeName:        "job-001",
				TaskTitle:       "Query task A",
				TaskType:        "backend",
				IDE:             "codex",
				Model:           "gpt-5.4",
				ReasoningEffort: "high",
				AccessMode:      "workspace-write",
			})
			submitEvent(ctx, t, prep.Journal(), cfg.RunID, eventspkg.EventKindJobStarted, kinds.JobStartedPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{Index: 1, Attempt: 1, MaxAttempts: 1},
				IDE:            "codex",
				Model:          "gpt-5.4",
			})
			textBlock, err := kinds.NewContentBlock(kinds.TextBlock{Text: "hello from query service"})
			if err != nil {
				return err
			}
			submitEvent(ctx, t, prep.Journal(), cfg.RunID, eventspkg.EventKindSessionUpdate, kinds.SessionUpdatePayload{
				Index: 1,
				Update: kinds.SessionUpdate{
					Kind:   kinds.UpdateKindAgentMessageChunk,
					Status: kinds.StatusRunning,
					Blocks: []kinds.ContentBlock{textBlock},
				},
			})
			submitEvent(ctx, t, prep.Journal(), cfg.RunID, eventspkg.EventKindUsageUpdated, kinds.UsageUpdatedPayload{
				Index: 1,
				Usage: kinds.Usage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
			})
			submitEvent(ctx, t, prep.Journal(), cfg.RunID, eventspkg.EventKindJobCompleted, kinds.JobCompletedPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{Index: 1, Attempt: 1, MaxAttempts: 1},
			})
			submitEvent(ctx, t, prep.Journal(), cfg.RunID, eventspkg.EventKindRunCompleted, kinds.RunCompletedPayload{
				ArtifactsDir:   runArtifacts.RunDir,
				ResultPath:     runArtifacts.ResultPath,
				SummaryMessage: "query service complete",
			})
			return nil
		},
	})

	env.writeWorkflowFile(t, env.workflowSlug, "_prd.md", strings.Join([]string{
		"---",
		"title: Query PRD",
		"---",
		"",
		"# Query PRD",
		"",
		"PRD body.",
	}, "\n"))
	env.writeWorkflowFile(t, env.workflowSlug, "_techspec.md", strings.Join([]string{
		"---",
		"title: Query TechSpec",
		"---",
		"",
		"# Query TechSpec",
		"",
		"TechSpec body.",
	}, "\n"))
	env.writeWorkflowFile(t, env.workflowSlug, filepath.Join("adrs", "adr-001.md"), "# ADR 001\n\nADR body.\n")
	env.writeWorkflowFile(t, env.workflowSlug, "task_01.md", daemonTaskBody("pending", "Query task A"))
	env.writeWorkflowFile(t, env.workflowSlug, "task_02.md", daemonTaskBody("completed", "Query task B"))
	env.writeWorkflowFile(
		t,
		env.workflowSlug,
		filepath.Join("memory", "MEMORY.md"),
		"# Workflow Memory\n\nWorkflow note.\n",
	)
	env.writeWorkflowFile(
		t,
		env.workflowSlug,
		filepath.Join("memory", "task_01.md"),
		"# Task 01 Memory\n\nTask note.\n",
	)
	env.writeWorkflowFile(
		t,
		env.workflowSlug,
		filepath.Join("reviews-001", "_meta.md"),
		daemonReviewRoundMetaBody("coderabbit", "123", 1),
	)
	env.writeWorkflowFile(
		t,
		env.workflowSlug,
		filepath.Join("reviews-001", "issue_001.md"),
		daemonReviewIssueBody("pending", "medium"),
	)

	run := env.startTaskRun(t, "query-service-run-001", nil)
	waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCompleted
	})

	service := NewQueryService(QueryServiceConfig{
		GlobalDB:   env.globalDB,
		RunManager: env.manager,
		Daemon: stubDaemonStatusReader{
			status: apicore.DaemonStatus{PID: 42, ActiveRunCount: 0, WorkspaceCount: 1},
			health: apicore.DaemonHealth{Ready: true},
		},
	})

	dashboard, err := service.Dashboard(context.Background(), env.workspaceRoot)
	if err != nil {
		t.Fatalf("Dashboard() error = %v", err)
	}
	if dashboard.PendingReviews != 1 {
		t.Fatalf("dashboard.PendingReviews = %d, want 1", dashboard.PendingReviews)
	}
	if len(dashboard.Workflows) != 1 {
		t.Fatalf("len(dashboard.Workflows) = %d, want 1", len(dashboard.Workflows))
	}
	card := dashboard.Workflows[0]
	if card.TaskTotal != 2 || card.TaskCompleted != 1 || card.TaskPending != 1 {
		t.Fatalf("unexpected workflow card task counts: %#v", card)
	}

	overview, err := service.WorkflowOverview(context.Background(), env.workspaceRoot, env.workflowSlug)
	if err != nil {
		t.Fatalf("WorkflowOverview() error = %v", err)
	}
	if overview.TaskCounts.Total != 2 || overview.TaskCounts.Completed != 1 || overview.TaskCounts.Pending != 1 {
		t.Fatalf("unexpected workflow overview counts: %#v", overview.TaskCounts)
	}
	if overview.LatestReview == nil || overview.LatestReview.RoundNumber != 1 {
		t.Fatalf("unexpected workflow latest review: %#v", overview.LatestReview)
	}
	if len(overview.RecentRuns) == 0 || overview.RecentRuns[0].RunID != run.RunID {
		t.Fatalf("unexpected workflow recent runs: %#v", overview.RecentRuns)
	}

	board, err := service.TaskBoard(context.Background(), env.workspaceRoot, env.workflowSlug)
	if err != nil {
		t.Fatalf("TaskBoard() error = %v", err)
	}
	if len(board.Lanes) < 2 {
		t.Fatalf("len(board.Lanes) = %d, want at least 2", len(board.Lanes))
	}

	spec, err := service.WorkflowSpec(context.Background(), env.workspaceRoot, env.workflowSlug)
	if err != nil {
		t.Fatalf("WorkflowSpec() error = %v", err)
	}
	if spec.PRD == nil || spec.PRD.Title != "Query PRD" {
		t.Fatalf("unexpected PRD payload: %#v", spec.PRD)
	}
	if spec.TechSpec == nil || spec.TechSpec.Title != "Query TechSpec" {
		t.Fatalf("unexpected TechSpec payload: %#v", spec.TechSpec)
	}
	if len(spec.ADRs) != 1 || spec.ADRs[0].Title != "ADR 001" {
		t.Fatalf("unexpected ADR payloads: %#v", spec.ADRs)
	}
	if strings.Contains(spec.TechSpec.Markdown, "title: Query TechSpec") {
		t.Fatalf("TechSpec.Markdown unexpectedly contains front matter:\n%s", spec.TechSpec.Markdown)
	}

	memoryIndex, err := service.WorkflowMemoryIndex(context.Background(), env.workspaceRoot, env.workflowSlug)
	if err != nil {
		t.Fatalf("WorkflowMemoryIndex() error = %v", err)
	}
	if len(memoryIndex.Entries) != 2 {
		t.Fatalf("len(memoryIndex.Entries) = %d, want 2", len(memoryIndex.Entries))
	}
	for _, entry := range memoryIndex.Entries {
		if !strings.HasPrefix(entry.FileID, "mem_") {
			t.Fatalf("memory entry file id = %q, want mem_ prefix", entry.FileID)
		}
		if !strings.HasPrefix(entry.DisplayPath, "memory/") {
			t.Fatalf("memory entry display path = %q, want memory/ prefix", entry.DisplayPath)
		}
	}

	var taskMemoryID string
	for _, entry := range memoryIndex.Entries {
		if entry.Kind == "task" {
			taskMemoryID = entry.FileID
		}
	}
	if taskMemoryID == "" {
		t.Fatal("task memory entry not found")
	}

	memoryDoc, err := service.WorkflowMemoryFile(
		context.Background(),
		env.workspaceRoot,
		env.workflowSlug,
		taskMemoryID,
	)
	if err != nil {
		t.Fatalf("WorkflowMemoryFile() error = %v", err)
	}
	if memoryDoc.Title != "Task 01 Memory" {
		t.Fatalf("memoryDoc.Title = %q, want Task 01 Memory", memoryDoc.Title)
	}

	taskDetail, err := service.TaskDetail(context.Background(), env.workspaceRoot, env.workflowSlug, "task_1")
	if err != nil {
		t.Fatalf("TaskDetail() error = %v", err)
	}
	if taskDetail.Task.Title != "Query task A" {
		t.Fatalf("taskDetail.Task.Title = %q, want Query task A", taskDetail.Task.Title)
	}
	if len(taskDetail.MemoryEntries) != 1 || taskDetail.MemoryEntries[0].Kind != "task" {
		t.Fatalf("unexpected task memory entries: %#v", taskDetail.MemoryEntries)
	}
	if len(taskDetail.RelatedRuns) == 0 || taskDetail.RelatedRuns[0].RunID != run.RunID {
		t.Fatalf("unexpected task related runs: %#v", taskDetail.RelatedRuns)
	}

	reviewDetail, err := service.ReviewDetail(context.Background(), env.workspaceRoot, env.workflowSlug, 1, "1")
	if err != nil {
		t.Fatalf("ReviewDetail() error = %v", err)
	}
	if reviewDetail.Issue.IssueNumber != 1 || reviewDetail.Issue.Severity != "medium" {
		t.Fatalf("unexpected review issue detail: %#v", reviewDetail.Issue)
	}
	if reviewDetail.Document.Title != "Issue 001: Example" {
		t.Fatalf("review detail title = %q, want Issue 001: Example", reviewDetail.Document.Title)
	}

	runDetail, err := service.RunDetail(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("RunDetail() error = %v", err)
	}
	if runDetail.Snapshot.Run.RunID != run.RunID {
		t.Fatalf("run detail snapshot run id = %q, want %q", runDetail.Snapshot.Run.RunID, run.RunID)
	}
	if len(runDetail.Timeline) < 5 {
		t.Fatalf("len(runDetail.Timeline) = %d, want at least 5", len(runDetail.Timeline))
	}
	if runDetail.JobCounts.Completed != 1 {
		t.Fatalf("runDetail.JobCounts.Completed = %d, want 1", runDetail.JobCounts.Completed)
	}
	if len(runDetail.Snapshot.Jobs) != 1 || runDetail.Snapshot.Jobs[0].Summary == nil {
		t.Fatalf("unexpected runDetail.Snapshot.Jobs payload: %#v", runDetail.Snapshot.Jobs)
	}
	if runDetail.Snapshot.Jobs[0].Summary.Usage.TotalTokens != 15 {
		t.Fatalf(
			"runDetail.Snapshot.Jobs[0].Summary.Usage.TotalTokens = %d, want 15",
			runDetail.Snapshot.Jobs[0].Summary.Usage.TotalTokens,
		)
	}
	if len(runDetail.Runtime.Models) == 0 || runDetail.Runtime.Models[0] != "gpt-5.4" {
		t.Fatalf("unexpected run runtime summary: %#v", runDetail.Runtime)
	}
}

func TestQueryServiceReportsTypedMissingAndStaleDocumentFailures(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{})

	env.writeWorkflowFile(t, env.workflowSlug, "task_01.md", daemonTaskBody("pending", "Missing doc task"))
	env.writeWorkflowFile(
		t,
		env.workflowSlug,
		filepath.Join("memory", "MEMORY.md"),
		"# Workflow Memory\n\nMemory body.\n",
	)

	run := env.startTaskRun(t, "query-service-run-002", nil)
	waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCompleted
	})

	service := NewQueryService(QueryServiceConfig{
		GlobalDB:   env.globalDB,
		RunManager: env.manager,
	})

	index, err := service.WorkflowMemoryIndex(context.Background(), env.workspaceRoot, env.workflowSlug)
	if err != nil {
		t.Fatalf("WorkflowMemoryIndex() error = %v", err)
	}
	if len(index.Entries) != 1 {
		t.Fatalf("len(index.Entries) = %d, want 1", len(index.Entries))
	}
	fileID := index.Entries[0].FileID

	taskPath := filepath.Join(env.workflowDir(env.workflowSlug), "task_01.md")
	if err := os.Remove(taskPath); err != nil {
		t.Fatalf("Remove(task_01.md) error = %v", err)
	}

	_, err = service.TaskDetail(context.Background(), env.workspaceRoot, env.workflowSlug, "task_1")
	if !errors.Is(err, ErrDocumentMissing) {
		t.Fatalf("TaskDetail() error = %v, want ErrDocumentMissing", err)
	}
	var missingErr DocumentMissingError
	if !errors.As(err, &missingErr) {
		t.Fatalf("TaskDetail() error = %v, want DocumentMissingError", err)
	}
	if missingErr.RelativePath != "task_01.md" {
		t.Fatalf("missingErr.RelativePath = %q, want task_01.md", missingErr.RelativePath)
	}

	memoryPath := filepath.Join(env.workflowDir(env.workflowSlug), "memory", "MEMORY.md")
	if err := os.Remove(memoryPath); err != nil {
		t.Fatalf("Remove(memory/MEMORY.md) error = %v", err)
	}

	_, err = service.WorkflowMemoryFile(context.Background(), env.workspaceRoot, env.workflowSlug, fileID)
	if !errors.Is(err, ErrStaleDocumentReference) {
		t.Fatalf("WorkflowMemoryFile() error = %v, want ErrStaleDocumentReference", err)
	}
	var staleErr StaleDocumentReferenceError
	if !errors.As(err, &staleErr) {
		t.Fatalf("WorkflowMemoryFile() error = %v, want StaleDocumentReferenceError", err)
	}
	if staleErr.Reference != fileID {
		t.Fatalf("staleErr.Reference = %q, want %q", staleErr.Reference, fileID)
	}
}
