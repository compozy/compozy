package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	corepkg "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/workpackages"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/rundb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

type transportReadModelFixture struct {
	env     *runManagerTestEnv
	query   QueryService
	taskRun apicore.Run
}

func TestTaskTransportServiceExposesRichReadModelsFromRealDaemonState(t *testing.T) {
	fixture := newTransportReadModelFixture(t)
	service := newTransportTaskService(fixture.env.globalDB, fixture.env.manager, fixture.query)

	dashboard, err := service.Dashboard(context.Background(), fixture.env.workspaceRoot)
	if err != nil {
		t.Fatalf("Dashboard() error = %v", err)
	}
	if !sameCanonicalPath(t, dashboard.Workspace.RootDir, fixture.env.workspaceRoot) {
		t.Fatalf(
			"dashboard.Workspace.RootDir = %q, want canonical match for %q",
			dashboard.Workspace.RootDir,
			fixture.env.workspaceRoot,
		)
	}
	if dashboard.Queue.Completed != 1 || dashboard.PendingReviews != 1 {
		t.Fatalf("unexpected dashboard queue/review payload: %#v", dashboard)
	}
	if len(dashboard.Workflows) != 1 || dashboard.Workflows[0].Workflow.Slug != fixture.env.workflowSlug {
		t.Fatalf("unexpected dashboard workflows: %#v", dashboard.Workflows)
	}
	if dashboard.Workflows[0].LatestReview == nil || dashboard.Workflows[0].LatestReview.RoundNumber != 1 {
		t.Fatalf("unexpected dashboard latest review: %#v", dashboard.Workflows[0].LatestReview)
	}

	overview, err := service.WorkflowOverview(context.Background(), fixture.env.workspaceRoot, fixture.env.workflowSlug)
	if err != nil {
		t.Fatalf("WorkflowOverview() error = %v", err)
	}
	if overview.TaskCounts.Total != 2 || overview.TaskCounts.Completed != 1 || overview.TaskCounts.Pending != 1 {
		t.Fatalf("unexpected workflow overview counts: %#v", overview.TaskCounts)
	}
	if overview.Workflow.TaskCounts == nil ||
		overview.Workflow.TaskCounts.Total != 2 ||
		overview.Workflow.TaskCounts.Completed != 1 ||
		overview.Workflow.TaskCounts.Pending != 1 {
		t.Fatalf("unexpected embedded workflow task counts: %#v", overview.Workflow.TaskCounts)
	}
	if overview.Workflow.CanStartRun == nil || !*overview.Workflow.CanStartRun ||
		overview.Workflow.StartBlockReason != "" {
		t.Fatalf("unexpected embedded workflow start metadata: %#v", overview.Workflow)
	}
	if len(overview.RecentRuns) == 0 || overview.RecentRuns[0].RunID != fixture.taskRun.RunID {
		t.Fatalf("unexpected workflow overview recent runs: %#v", overview.RecentRuns)
	}

	board, err := service.TaskBoard(context.Background(), fixture.env.workspaceRoot, fixture.env.workflowSlug)
	if err != nil {
		t.Fatalf("TaskBoard() error = %v", err)
	}
	if board.Workflow.Slug != fixture.env.workflowSlug {
		t.Fatalf("board.Workflow.Slug = %q, want %q", board.Workflow.Slug, fixture.env.workflowSlug)
	}
	if board.TaskCounts.Total != 2 {
		t.Fatalf("board.TaskCounts.Total = %d, want 2", board.TaskCounts.Total)
	}
	boardItems := 0
	for _, lane := range board.Lanes {
		boardItems += len(lane.Items)
	}
	if boardItems != 2 {
		t.Fatalf("board item count = %d, want 2 across lanes %#v", boardItems, board.Lanes)
	}

	spec, err := service.WorkflowSpec(context.Background(), fixture.env.workspaceRoot, fixture.env.workflowSlug)
	if err != nil {
		t.Fatalf("WorkflowSpec() error = %v", err)
	}
	if spec.PRD == nil || spec.PRD.Title != "Transport PRD" {
		t.Fatalf("unexpected spec PRD payload: %#v", spec.PRD)
	}
	if spec.TechSpec == nil || spec.TechSpec.Title != "Transport TechSpec" {
		t.Fatalf("unexpected spec TechSpec payload: %#v", spec.TechSpec)
	}
	if len(spec.ADRs) != 1 || spec.ADRs[0].Title != "ADR 001" {
		t.Fatalf("unexpected spec ADR payload: %#v", spec.ADRs)
	}
	if strings.Contains(spec.TechSpec.Markdown, "title: Transport TechSpec") {
		t.Fatalf("TechSpec.Markdown unexpectedly contains front matter:\n%s", spec.TechSpec.Markdown)
	}

	memoryIndex, err := service.WorkflowMemoryIndex(
		context.Background(),
		fixture.env.workspaceRoot,
		fixture.env.workflowSlug,
	)
	if err != nil {
		t.Fatalf("WorkflowMemoryIndex() error = %v", err)
	}
	if len(memoryIndex.Entries) != 2 {
		t.Fatalf("len(memoryIndex.Entries) = %d, want 2", len(memoryIndex.Entries))
	}

	var taskMemoryID string
	for _, entry := range memoryIndex.Entries {
		if !strings.HasPrefix(entry.FileID, "mem_") {
			t.Fatalf("memory entry file id = %q, want mem_ prefix", entry.FileID)
		}
		if !strings.HasPrefix(entry.DisplayPath, "memory/") {
			t.Fatalf("memory entry display path = %q, want memory/ prefix", entry.DisplayPath)
		}
		if entry.Kind == "task" {
			taskMemoryID = entry.FileID
		}
	}
	if taskMemoryID == "" {
		t.Fatal("task memory entry not found")
	}

	memoryDoc, err := service.WorkflowMemoryFile(
		context.Background(),
		fixture.env.workspaceRoot,
		fixture.env.workflowSlug,
		taskMemoryID,
	)
	if err != nil {
		t.Fatalf("WorkflowMemoryFile() error = %v", err)
	}
	if memoryDoc.Title != "Task 01 Memory" {
		t.Fatalf("memoryDoc.Title = %q, want Task 01 Memory", memoryDoc.Title)
	}

	taskDetail, err := service.TaskDetail(
		context.Background(),
		fixture.env.workspaceRoot,
		fixture.env.workflowSlug,
		"task_01",
	)
	if err != nil {
		t.Fatalf("TaskDetail() error = %v", err)
	}
	if taskDetail.Task.Title != "Transport task A" {
		t.Fatalf("taskDetail.Task.Title = %q, want Transport task A", taskDetail.Task.Title)
	}
	if len(taskDetail.MemoryEntries) != 1 || taskDetail.MemoryEntries[0].Kind != "task" {
		t.Fatalf("unexpected task detail memory entries: %#v", taskDetail.MemoryEntries)
	}
	if len(taskDetail.RelatedRuns) == 0 || taskDetail.RelatedRuns[0].RunID != fixture.taskRun.RunID {
		t.Fatalf("unexpected task detail related runs: %#v", taskDetail.RelatedRuns)
	}
	if taskDetail.LiveTailAvailable {
		t.Fatal("taskDetail.LiveTailAvailable = true, want false for completed runs")
	}
}

func TestTaskTransportServiceMapsRichReadFailuresToTransportProblems(t *testing.T) {
	fixture := newTransportReadModelFixture(t)
	service := newTransportTaskService(fixture.env.globalDB, fixture.env.manager, fixture.query)

	if _, err := service.TaskDetail(
		context.Background(),
		fixture.env.workspaceRoot,
		fixture.env.workflowSlug,
		"task_missing",
	); err == nil {
		t.Fatal("TaskDetail(missing task) error = nil, want task_item_not_found problem")
	} else {
		problem := mustProblem(t, err)
		if problem.Status != http.StatusNotFound || problem.Code != "task_item_not_found" {
			t.Fatalf("missing task problem = %#v, want 404 task_item_not_found", problem)
		}
	}

	memoryIndex, err := service.WorkflowMemoryIndex(
		context.Background(),
		fixture.env.workspaceRoot,
		fixture.env.workflowSlug,
	)
	if err != nil {
		t.Fatalf("WorkflowMemoryIndex() error = %v", err)
	}

	var workflowMemoryID string
	for _, entry := range memoryIndex.Entries {
		if entry.DisplayPath == "memory/MEMORY.md" {
			workflowMemoryID = entry.FileID
		}
	}
	if workflowMemoryID == "" {
		t.Fatal("workflow memory file id not found")
	}

	taskPath := filepath.Join(fixture.env.workflowDir(fixture.env.workflowSlug), "task_01.md")
	if err := os.Remove(taskPath); err != nil {
		t.Fatalf("Remove(task_01.md) error = %v", err)
	}

	taskDetail, err := service.TaskDetail(
		context.Background(),
		fixture.env.workspaceRoot,
		fixture.env.workflowSlug,
		"task_01",
	)
	if err != nil {
		t.Fatalf("TaskDetail(document removed) error = %v, want snapshot-backed success", err)
	}
	if !strings.Contains(taskDetail.Document.Markdown, "Transport task A") {
		t.Fatalf("TaskDetail(document removed) markdown = %q, want synced task body", taskDetail.Document.Markdown)
	}

	memoryPath := filepath.Join(fixture.env.workflowDir(fixture.env.workflowSlug), "memory", "MEMORY.md")
	if err := os.Remove(memoryPath); err != nil {
		t.Fatalf("Remove(memory/MEMORY.md) error = %v", err)
	}

	memoryDoc, err := service.WorkflowMemoryFile(
		context.Background(),
		fixture.env.workspaceRoot,
		fixture.env.workflowSlug,
		workflowMemoryID,
	)
	if err != nil {
		t.Fatalf("WorkflowMemoryFile(document removed) error = %v, want snapshot-backed success", err)
	}
	if !strings.Contains(memoryDoc.Markdown, "Workflow note.") {
		t.Fatalf("WorkflowMemoryFile(document removed) markdown = %q, want synced memory body", memoryDoc.Markdown)
	}
}

func TestReviewTransportAndRunDetailExposeReadModelsAndTypedFailures(t *testing.T) {
	fixture := newTransportReadModelFixture(t)
	reviewRun := fixture.env.startReviewRun(t, "transport-review-run-001", 1, nil, nil)
	waitForRun(t, fixture.env.globalDB, reviewRun.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCompleted
	})

	reviewService := newTransportReviewService(fixture.env.globalDB, fixture.env.manager, fixture.query)

	detail, err := reviewService.ReviewDetail(
		context.Background(),
		fixture.env.workspaceRoot,
		fixture.env.workflowSlug,
		1,
		"1",
	)
	if err != nil {
		t.Fatalf("ReviewDetail() error = %v", err)
	}
	if detail.Issue.IssueNumber != 1 || detail.Issue.Severity != "medium" {
		t.Fatalf("unexpected review detail issue payload: %#v", detail.Issue)
	}
	if detail.Document.Title != "Issue 001: Example" {
		t.Fatalf("detail.Document.Title = %q, want Issue 001: Example", detail.Document.Title)
	}
	if len(detail.RelatedRuns) == 0 || detail.RelatedRuns[0].Mode != runModeReview {
		t.Fatalf("unexpected review detail related runs: %#v", detail.RelatedRuns)
	}

	runDetail, err := fixture.env.manager.RunDetail(context.Background(), fixture.taskRun.RunID)
	if err != nil {
		t.Fatalf("RunDetail() error = %v", err)
	}
	if runDetail.Run.RunID != fixture.taskRun.RunID {
		t.Fatalf("runDetail.Run.RunID = %q, want %q", runDetail.Run.RunID, fixture.taskRun.RunID)
	}
	if len(runDetail.Timeline) < 5 {
		t.Fatalf("len(runDetail.Timeline) = %d, want at least 5", len(runDetail.Timeline))
	}
	if runDetail.JobCounts.Completed != 1 {
		t.Fatalf("runDetail.JobCounts.Completed = %d, want 1", runDetail.JobCounts.Completed)
	}
	if len(runDetail.Runtime.Models) == 0 || runDetail.Runtime.Models[0] != "gpt-5.5" {
		t.Fatalf("unexpected run detail runtime summary: %#v", runDetail.Runtime)
	}

	_, err = reviewService.ReviewDetail(
		context.Background(),
		fixture.env.workspaceRoot,
		fixture.env.workflowSlug,
		1,
		"999",
	)
	issueProblem := mustProblem(t, err)
	if issueProblem.Status != http.StatusNotFound || issueProblem.Code != "review_issue_not_found" {
		t.Fatalf("ReviewDetail(missing issue) = %#v, want 404 review_issue_not_found", issueProblem)
	}

	_, err = reviewService.ReviewDetail(
		context.Background(),
		fixture.env.workspaceRoot,
		fixture.env.workflowSlug,
		2,
		"1",
	)
	roundProblem := mustProblem(t, err)
	if roundProblem.Status != http.StatusNotFound || roundProblem.Code != "review_round_not_found" {
		t.Fatalf("ReviewDetail(missing round) = %#v, want 404 review_round_not_found", roundProblem)
	}
}

func TestTransportReadModelMappersCloneMutableCollections(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 21, 15, 0, 0, 0, time.UTC)
	specSource := WorkflowSpecDocument{
		PRD: &MarkdownDocument{
			ID:        "prd",
			Kind:      "prd",
			Title:     "PRD",
			UpdatedAt: now,
			Markdown:  "body",
			Metadata: map[string]any{
				"owner": "daemon",
				"nested": map[string]any{
					"stage": "draft",
				},
			},
		},
		ADRs: []MarkdownDocument{{
			ID:        "adr-001",
			Kind:      "adr",
			Title:     "ADR 001",
			UpdatedAt: now,
			Metadata: map[string]any{
				"stage":  "draft",
				"labels": []any{"rfc"},
			},
		}},
	}
	specMapped := transportWorkflowSpec(specSource)
	specSource.PRD.Metadata["owner"] = "browser"
	specSource.PRD.Metadata["nested"].(map[string]any)["stage"] = "accepted"
	specSource.ADRs[0].Metadata["stage"] = "final"
	specSource.ADRs[0].Metadata["labels"].([]any)[0] = "accepted"

	prdMetadata := mustTransportMetadataMap(t, specMapped.PRD.Metadata)
	if got := prdMetadata["owner"]; got != "daemon" {
		t.Fatalf("mapped PRD metadata owner = %#v, want daemon", got)
	}
	if got := prdMetadata["nested"].(map[string]any)["stage"]; got != "draft" {
		t.Fatalf("mapped PRD nested stage = %#v, want draft", got)
	}
	adrMetadata := mustTransportMetadataMap(t, specMapped.ADRs[0].Metadata)
	if got := adrMetadata["stage"]; got != "draft" {
		t.Fatalf("mapped ADR metadata stage = %#v, want draft", got)
	}
	if got := adrMetadata["labels"].([]any)[0]; got != "rfc" {
		t.Fatalf("mapped ADR metadata label = %#v, want rfc", got)
	}

	taskSource := TaskDetailPayload{
		Task: TaskCard{
			TaskID:    "task_01",
			DependsOn: []string{"task_00"},
		},
		Document: MarkdownDocument{
			ID:    "task_01",
			Kind:  "task",
			Title: "Task 1",
			Metadata: map[string]any{
				"status": "pending",
				"details": map[string]any{
					"owner": "daemon",
				},
			},
		},
		MemoryEntries: []WorkflowMemoryEntry{{FileID: "mem_1"}},
		RelatedRuns:   []apicore.Run{{RunID: "run-1"}},
	}
	taskMapped := transportTaskDetail(taskSource)
	taskMapped.Task.DependsOn[0] = "task_x"
	taskMapped.MemoryEntries[0].FileID = "mem_x"
	taskMapped.RelatedRuns[0].RunID = "run-x"
	taskSource.Document.Metadata["status"] = "completed"
	taskSource.Document.Metadata["details"].(map[string]any)["owner"] = "browser"
	if got := taskSource.Task.DependsOn[0]; got != "task_00" {
		t.Fatalf("source task depends_on mutated = %q, want task_00", got)
	}
	taskMetadata := mustTransportMetadataMap(t, taskMapped.Document.Metadata)
	if got := taskMetadata["status"]; got != "pending" {
		t.Fatalf("mapped task document status = %#v, want pending", got)
	}
	if got := taskMetadata["details"].(map[string]any)["owner"]; got != "daemon" {
		t.Fatalf("mapped task document owner = %#v, want daemon", got)
	}
	if got := taskSource.MemoryEntries[0].FileID; got != "mem_1" {
		t.Fatalf("source task memory entry mutated = %q, want mem_1", got)
	}
	if got := taskSource.RelatedRuns[0].RunID; got != "run-1" {
		t.Fatalf("source task related run mutated = %q, want run-1", got)
	}

	runSource := RunDetailPayload{
		Runtime: RunRuntimeSummary{
			Models:            []string{"gpt-5.5"},
			AccessModes:       []string{"workspace-write"},
			PresentationModes: []string{"stream"},
		},
		Timeline: []eventspkg.Event{{Seq: 1}},
		ArtifactSync: []rundb.ArtifactSyncRow{{
			Sequence:     1,
			RelativePath: "task_01.md",
		}},
	}
	runMapped := transportRunDetail(runSource)
	runMapped.Runtime.Models[0] = "changed"
	runMapped.Timeline[0].Seq = 7
	runMapped.ArtifactSync[0].RelativePath = "changed.md"
	if got := runSource.Runtime.Models[0]; got != "gpt-5.5" {
		t.Fatalf("source run runtime models mutated = %q, want gpt-5.5", got)
	}
	if got := runSource.Timeline[0].Seq; got != 1 {
		t.Fatalf("source run timeline mutated = %d, want 1", got)
	}
	if got := runSource.ArtifactSync[0].RelativePath; got != "task_01.md" {
		t.Fatalf("source run artifact sync mutated = %q, want task_01.md", got)
	}
}

func mustTransportMetadataMap(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()

	if len(raw) == 0 {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("json.Unmarshal(metadata) error = %v", err)
	}
	return out
}

func TestMapQueryTransportErrorReturnsTransportProblems(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantCode   string
		wantStatus int
		wantDetail string
	}{
		{
			name: "document missing",
			err: DocumentMissingError{
				Kind:         "task",
				WorkflowSlug: "demo",
				RelativePath: "task_01.md",
			},
			wantCode:   "document_not_found",
			wantStatus: http.StatusNotFound,
			wantDetail: "relative_path",
		},
		{
			name: "stale reference",
			err: StaleDocumentReferenceError{
				Kind:         "memory",
				WorkflowSlug: "demo",
				Reference:    "mem_1",
			},
			wantCode:   "stale_document_reference",
			wantStatus: http.StatusNotFound,
			wantDetail: "reference",
		},
		{
			name: "review issue missing",
			err: ReviewIssueNotFoundError{
				WorkflowSlug: "demo",
				Round:        2,
				IssueRef:     "9",
			},
			wantCode:   "review_issue_not_found",
			wantStatus: http.StatusNotFound,
			wantDetail: "issue_ref",
		},
		{
			name:       "task item missing",
			err:        globaldb.ErrTaskItemNotFound,
			wantCode:   "task_item_not_found",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "review round missing",
			err:        globaldb.ErrReviewRoundNotFound,
			wantCode:   "review_round_not_found",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			problem := mustProblem(t, mapQueryTransportError(tc.err))
			if problem.Status != tc.wantStatus || problem.Code != tc.wantCode {
				t.Fatalf("problem = %#v, want status=%d code=%q", problem, tc.wantStatus, tc.wantCode)
			}
			if tc.wantDetail != "" && problem.Details[tc.wantDetail] == nil {
				t.Fatalf("problem details = %#v, want key %q", problem.Details, tc.wantDetail)
			}
		})
	}

	passthrough := errors.New("plain failure")
	if got := mapQueryTransportError(passthrough); !errors.Is(got, passthrough) {
		t.Fatalf("mapQueryTransportError(plain) = %v, want passthrough %v", got, passthrough)
	}
}

func newTransportReadModelFixture(t *testing.T) transportReadModelFixture {
	t.Helper()

	env := newRunManagerTestEnv(t, runManagerTestDeps{
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(ctx context.Context, prep *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			runArtifacts, err := model.ResolveRuntimeRunArtifacts(cfg)
			if err != nil {
				return err
			}

			submitEvent(ctx, t, prep.Journal(), cfg.RunID, eventspkg.EventKindJobQueued, kinds.JobQueuedPayload{
				Index:           1,
				SafeName:        "job-001",
				TaskTitle:       "Transport task A",
				TaskType:        "backend",
				IDE:             "codex",
				Model:           "gpt-5.5",
				ReasoningEffort: "high",
				AccessMode:      "workspace-write",
			})
			submitEvent(ctx, t, prep.Journal(), cfg.RunID, eventspkg.EventKindJobStarted, kinds.JobStartedPayload{
				JobAttemptInfo: kinds.JobAttemptInfo{Index: 1, Attempt: 1, MaxAttempts: 1},
				IDE:            "codex",
				Model:          "gpt-5.5",
			})
			textBlock, err := kinds.NewContentBlock(kinds.TextBlock{Text: "hello from transport service"})
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
				SummaryMessage: "transport service complete",
			})
			return nil
		},
	})

	env.writeWorkflowFile(t, env.workflowSlug, "_prd.md", strings.Join([]string{
		"---",
		"title: Transport PRD",
		"---",
		"",
		"# Transport PRD",
		"",
		"PRD body.",
	}, "\n"))
	env.writeWorkflowFile(t, env.workflowSlug, "_techspec.md", strings.Join([]string{
		"---",
		"title: Transport TechSpec",
		"---",
		"",
		"# Transport TechSpec",
		"",
		"TechSpec body.",
	}, "\n"))
	env.writeWorkflowFile(t, env.workflowSlug, filepath.Join("adrs", "adr-001.md"), "# ADR 001\n\nADR body.\n")
	env.writeWorkflowFile(t, env.workflowSlug, "task_01.md", daemonTaskBody("pending", "Transport task A"))
	env.writeWorkflowFile(t, env.workflowSlug, "task_02.md", daemonTaskBody("completed", "Transport task B"))
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

	taskRun := env.startTaskRun(t, "transport-read-model-run-001", nil)
	waitForRun(t, env.globalDB, taskRun.RunID, func(row globaldb.Run) bool {
		return row.Status == runStatusCompleted
	})

	query := NewQueryService(QueryServiceConfig{
		GlobalDB:   env.globalDB,
		RunManager: env.manager,
		Daemon: stubDaemonStatusReader{
			status: apicore.DaemonStatus{PID: 42, ActiveRunCount: 0, WorkspaceCount: 1},
			health: apicore.DaemonHealth{Ready: true},
		},
	})

	return transportReadModelFixture{
		env:     env,
		query:   query,
		taskRun: taskRun,
	}
}

func TestTaskTransportServiceProjectsFirstPartialWorkPackageSync(t *testing.T) {
	// A first-ever partial sync (a present package depends on a missing one, plus a
	// missing independent package) must leave the daemon read models readable: the
	// stored declared graph is complete, so EvaluateReadiness no longer rejects it,
	// and the initiative is not falsely archive-eligible.
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error {
			return nil
		},
	})
	const slug = "wp-initiative"
	writeDaemonPartialWorkPackageInitiative(t, env, slug)

	workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister() error = %v", err)
	}
	result, err := corepkg.SyncWithDB(context.Background(), env.globalDB, workspace, corepkg.SyncConfig{
		TasksDir: env.workflowDir(slug),
	})
	if err != nil {
		t.Fatalf("SyncWithDB(first partial) error = %v", err)
	}
	if !result.Partial {
		t.Fatalf("first partial sync not flagged partial: %#v", result)
	}

	query := NewQueryService(QueryServiceConfig{
		GlobalDB:   env.globalDB,
		RunManager: env.manager,
		Daemon: stubDaemonStatusReader{
			status: apicore.DaemonStatus{PID: 7, WorkspaceCount: 1},
			health: apicore.DaemonHealth{Ready: true},
		},
	})
	service := newTransportTaskService(env.globalDB, env.manager, query)

	t.Run("Should list the initiative without rejecting the incomplete graph", func(t *testing.T) {
		workflows, err := service.ListWorkflows(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ListWorkflows() error = %v", err)
		}
		initiative := findInitiativeSummary(t, workflows, slug)
		if len(initiative.WorkPackages) != 3 {
			t.Fatalf(
				"initiative work packages = %d, want complete declared graph of 3",
				len(initiative.WorkPackages),
			)
		}
		if initiative.ArchiveEligible == nil || *initiative.ArchiveEligible {
			t.Fatalf(
				"initiative archive eligible = %v, want blocked by missing packages",
				initiative.ArchiveEligible,
			)
		}
		// WP-001 and WP-002 are the incomplete packages blocking archive, so both are
		// named as the actionable reason. WP-003 carries a completed manifest checkbox,
		// so the first partial sync honors it as lifecycle-complete rather than falsely
		// projecting it pending; it therefore no longer inflates the pending-package
		// archive reason even though its directory is still absent.
		if !strings.Contains(initiative.ArchiveReason, "WP-001") ||
			!strings.Contains(initiative.ArchiveReason, "WP-002") {
			t.Fatalf("archive reason = %q, want pending WP-001 and WP-002", initiative.ArchiveReason)
		}
		independent := findWorkPackageSummary(t, initiative.WorkPackages, "WP-003")
		if !independent.LifecycleComplete {
			t.Fatalf("WP-003 lifecycle_complete = false, want the completed checkbox honored on a missing placeholder")
		}
		dependent := findWorkPackageSummary(t, initiative.WorkPackages, "WP-002")
		if len(dependent.UnmetDependencies) != 1 || dependent.UnmetDependencies[0].PackageID != "WP-001" {
			t.Fatalf("WP-002 unmet dependencies = %#v, want single edge to WP-001", dependent.UnmetDependencies)
		}
		// Both missing placeholders (dependency-blocked WP-001 and independent
		// WP-003) must be reported as not runnable with a missing-directory reason
		// so the inventory never enables a start that would fail on re-resolution.
		assertMissingPlaceholderBlocked(t, initiative.WorkPackages, "WP-001")
		assertMissingPlaceholderBlocked(t, initiative.WorkPackages, "WP-003")
	})

	t.Run("Should project the same complete graph into the dashboard", func(t *testing.T) {
		dashboard, err := service.Dashboard(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("Dashboard() error = %v", err)
		}
		card := findDashboardInitiativeCard(t, dashboard.Workflows, slug)
		if len(card.Workflow.WorkPackages) != 3 {
			t.Fatalf("dashboard initiative work packages = %d, want 3", len(card.Workflow.WorkPackages))
		}
		assertMissingPlaceholderBlocked(t, card.Workflow.WorkPackages, "WP-001")
		assertMissingPlaceholderBlocked(t, card.Workflow.WorkPackages, "WP-003")
	})

	t.Run("Should refuse to really start a missing independent package", func(t *testing.T) {
		_, startErr := env.manager.StartTaskRun(
			context.Background(),
			env.workspaceRoot,
			slug,
			apicore.TaskRunRequest{
				Workspace:        env.workspaceRoot,
				PresentationMode: defaultPresentationMode,
				PackageID:        "WP-003",
			},
		)
		if !errors.Is(startErr, workpackages.ErrPackageNotFound) {
			t.Fatalf("StartTaskRun(missing WP-003) error = %v, want ErrPackageNotFound", startErr)
		}
	})
}

func TestTaskTransportServiceProjectsWorkPackageReviewIntoReadModels(t *testing.T) {
	// A work package's latest review round must survive projection into both read
	// models (workflow list and dashboard) so the UI can navigate to it, and the
	// dashboard pending-review total must include package unresolved counts that the
	// parent card cannot see on its own review rounds.
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error {
			return nil
		},
	})
	const slug = "wp-initiative"
	writeDaemonMaterializedWorkPackageInitiative(t, env, slug)
	// Seed a local review round carrying one unresolved issue under two packages.
	for _, packageID := range []string{"WP-001", "WP-002"} {
		env.writeWorkflowFile(
			t,
			slug,
			filepath.Join("_packages", packageID, "reviews-001", "_meta.md"),
			daemonReviewRoundMetaBody("coderabbit", "123", 1),
		)
		env.writeWorkflowFile(
			t,
			slug,
			filepath.Join("_packages", packageID, "reviews-001", "issue_001.md"),
			daemonReviewIssueBody("pending", "medium"),
		)
	}

	workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister() error = %v", err)
	}
	if _, err := corepkg.SyncWithDB(context.Background(), env.globalDB, workspace, corepkg.SyncConfig{
		TasksDir: env.workflowDir(slug),
	}); err != nil {
		t.Fatalf("SyncWithDB() error = %v", err)
	}

	query := NewQueryService(QueryServiceConfig{
		GlobalDB:   env.globalDB,
		RunManager: env.manager,
		Daemon: stubDaemonStatusReader{
			status: apicore.DaemonStatus{PID: 7, WorkspaceCount: 1},
			health: apicore.DaemonHealth{Ready: true},
		},
	})
	service := newTransportTaskService(env.globalDB, env.manager, query)

	assertPackageLatestReview := func(t *testing.T, pkg apicore.WorkPackageSummary) {
		t.Helper()
		if pkg.LatestReview == nil {
			t.Fatalf("work package %q latest review = nil, want projected round", pkg.PackageID)
		}
		if pkg.LatestReview.RoundNumber != 1 {
			t.Fatalf(
				"work package %q latest review round = %d, want 1",
				pkg.PackageID,
				pkg.LatestReview.RoundNumber,
			)
		}
		if pkg.LatestReview.UnresolvedCount != 1 {
			t.Fatalf(
				"work package %q latest review unresolved = %d, want 1",
				pkg.PackageID,
				pkg.LatestReview.UnresolvedCount,
			)
		}
	}

	t.Run("Should project package latest review into the workflow list", func(t *testing.T) {
		workflows, err := service.ListWorkflows(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ListWorkflows() error = %v", err)
		}
		initiative := findInitiativeSummary(t, workflows, slug)
		assertPackageLatestReview(t, findWorkPackageSummary(t, initiative.WorkPackages, "WP-001"))
		assertPackageLatestReview(t, findWorkPackageSummary(t, initiative.WorkPackages, "WP-002"))
		if pkg := findWorkPackageSummary(t, initiative.WorkPackages, "WP-003"); pkg.LatestReview != nil {
			t.Fatalf("work package WP-003 latest review = %#v, want nil (no round)", pkg.LatestReview)
		}
	})

	t.Run("Should project package latest review and aggregate unresolved into the dashboard", func(t *testing.T) {
		dashboard, err := service.Dashboard(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("Dashboard() error = %v", err)
		}
		card := findDashboardInitiativeCard(t, dashboard.Workflows, slug)
		assertPackageLatestReview(t, findWorkPackageSummary(t, card.Workflow.WorkPackages, "WP-001"))
		assertPackageLatestReview(t, findWorkPackageSummary(t, card.Workflow.WorkPackages, "WP-002"))
		// The initiative card has no review round of its own, so the whole pending
		// total must come from the two package rounds — proving package unresolved
		// counts are no longer omitted from the aggregate.
		if dashboard.PendingReviews != 2 {
			t.Fatalf("dashboard pending reviews = %d, want 2 from package rounds", dashboard.PendingReviews)
		}
	})
}

func TestTaskTransportServiceBlocksArchiveForInitiativeWithActiveParentRun(t *testing.T) {
	// An ordinary workflow promoted to an initiative in place keeps its active run on the
	// parent workflow ID, a row that lives outside the plan-declared child workflows. The
	// core archive path (activeInitiativeRunConflict) refuses that hierarchy with
	// ErrWorkflowHasActiveRuns even when every package child is complete, so both read
	// models must report the initiative as archive-ineligible with an active-run reason
	// instead of advertising a Completed workflow whose archive action always fails.
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error {
			return nil
		},
	})
	const slug = "wp-initiative"
	writeDaemonMaterializedWorkPackageInitiative(t, env, slug)

	workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister() error = %v", err)
	}
	if _, err := corepkg.SyncWithDB(context.Background(), env.globalDB, workspace, corepkg.SyncConfig{
		TasksDir: env.workflowDir(slug),
	}); err != nil {
		t.Fatalf("SyncWithDB(materialized) error = %v", err)
	}

	query := NewQueryService(QueryServiceConfig{
		GlobalDB:   env.globalDB,
		RunManager: env.manager,
		Daemon: stubDaemonStatusReader{
			status: apicore.DaemonStatus{PID: 7, WorkspaceCount: 1},
			health: apicore.DaemonHealth{Ready: true},
		},
	})
	service := newTransportTaskService(env.globalDB, env.manager, query)

	t.Run("Should report the completed initiative as archive eligible before any run", func(t *testing.T) {
		workflows, err := service.ListWorkflows(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ListWorkflows() error = %v", err)
		}
		initiative := findInitiativeSummary(t, workflows, slug)
		if initiative.ArchiveEligible == nil || !*initiative.ArchiveEligible {
			t.Fatalf(
				"baseline initiative archive eligible = %v, want true with every package completed",
				initiative.ArchiveEligible,
			)
		}
	})

	// Seed an active run on the parent (initiative) workflow ID -- the exact row an
	// in-place promotion retains, which never appears among the plan-declared child rows.
	parentWorkflow, err := env.globalDB.GetActiveWorkflowBySlug(context.Background(), workspace.ID, slug)
	if err != nil {
		t.Fatalf("GetActiveWorkflowBySlug(%q) error = %v", slug, err)
	}
	if _, err := env.globalDB.PutRun(context.Background(), globaldb.Run{
		RunID:            "run-initiative-parent-active",
		WorkspaceID:      workspace.ID,
		WorkflowID:       &parentWorkflow.ID,
		Mode:             runModeTask,
		Status:           runStatusRunning,
		PresentationMode: defaultPresentationMode,
		StartedAt:        time.Date(2026, 7, 17, 22, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("PutRun(active parent) error = %v", err)
	}

	assertBlockedByActiveRun := func(t *testing.T, eligible *bool, reason string) {
		t.Helper()
		if eligible == nil || *eligible {
			t.Fatalf("initiative archive eligible = %v, want false while the parent run is active", eligible)
		}
		// The reason must match the core refusal's active-run cause (SkipReason ->
		// "workflow has active runs") so the inventory shows why archive is blocked.
		if !strings.Contains(reason, "active run") {
			t.Fatalf("archive reason = %q, want an active-run reason", reason)
		}
	}

	t.Run("Should block archive in the workflow list while the parent run is active", func(t *testing.T) {
		workflows, err := service.ListWorkflows(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ListWorkflows() error = %v", err)
		}
		initiative := findInitiativeSummary(t, workflows, slug)
		assertBlockedByActiveRun(t, initiative.ArchiveEligible, initiative.ArchiveReason)
	})

	t.Run("Should block archive in the dashboard while the parent run is active", func(t *testing.T) {
		dashboard, err := service.Dashboard(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("Dashboard() error = %v", err)
		}
		card := findDashboardInitiativeCard(t, dashboard.Workflows, slug)
		assertBlockedByActiveRun(t, card.Workflow.ArchiveEligible, card.Workflow.ArchiveReason)
	})
}

func TestTaskTransportServiceReprojectsMaterializedThenRemovedWorkPackage(t *testing.T) {
	// A previously materialized package whose directory later disappears must be
	// re-projected as missing across the read models: start is blocked with the
	// missing-directory reason, a real start fails on re-resolution, and a completed
	// package that vanished stops reporting archive-eligible so the read model no
	// longer disagrees with the filesystem.
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error {
			return nil
		},
	})
	const slug = "wp-initiative"
	writeDaemonMaterializedWorkPackageInitiative(t, env, slug)

	workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister() error = %v", err)
	}
	if _, err := corepkg.SyncWithDB(context.Background(), env.globalDB, workspace, corepkg.SyncConfig{
		TasksDir: env.workflowDir(slug),
	}); err != nil {
		t.Fatalf("SyncWithDB(materialized) error = %v", err)
	}

	query := NewQueryService(QueryServiceConfig{
		GlobalDB:   env.globalDB,
		RunManager: env.manager,
		Daemon: stubDaemonStatusReader{
			status: apicore.DaemonStatus{PID: 7, WorkspaceCount: 1},
			health: apicore.DaemonHealth{Ready: true},
		},
	})
	service := newTransportTaskService(env.globalDB, env.manager, query)

	t.Run("Should report the completed package as archive eligible while present", func(t *testing.T) {
		workflows, err := service.ListWorkflows(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ListWorkflows() error = %v", err)
		}
		initiative := findInitiativeSummary(t, workflows, slug)
		completed := findWorkPackageSummary(t, initiative.WorkPackages, "WP-003")
		if completed.ArchiveEligible == nil || !*completed.ArchiveEligible {
			t.Fatalf("present completed WP-003 archive eligible = %v, want true baseline", completed.ArchiveEligible)
		}
	})

	if err := os.RemoveAll(filepath.Join(env.workflowDir(slug), "_packages", "WP-003")); err != nil {
		t.Fatalf("remove WP-003: %v", err)
	}
	result, err := corepkg.SyncWithDB(context.Background(), env.globalDB, workspace, corepkg.SyncConfig{
		TasksDir: env.workflowDir(slug),
	})
	if err != nil {
		t.Fatalf("SyncWithDB(removed) error = %v", err)
	}
	if !result.Partial {
		t.Fatalf("removal sync not flagged partial: %#v", result)
	}

	t.Run("Should block start and archive for the removed completed package", func(t *testing.T) {
		workflows, err := service.ListWorkflows(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ListWorkflows() error = %v", err)
		}
		initiative := findInitiativeSummary(t, workflows, slug)
		if len(initiative.WorkPackages) != 3 {
			t.Fatalf("initiative work packages = %d, want complete declared graph of 3", len(initiative.WorkPackages))
		}
		// The completed package that vanished must stop advertising as runnable and
		// stop reporting archive-eligible even though its retained projection looks
		// complete; otherwise the read model disagrees with the filesystem.
		assertMissingPlaceholderBlocked(t, initiative.WorkPackages, "WP-003")
		removed := findWorkPackageSummary(t, initiative.WorkPackages, "WP-003")
		if removed.ArchiveEligible == nil || *removed.ArchiveEligible {
			t.Fatalf("removed completed WP-003 archive eligible = %v, want false", removed.ArchiveEligible)
		}
		if initiative.ArchiveEligible == nil || *initiative.ArchiveEligible {
			t.Fatalf("initiative archive eligible = %v, want blocked by missing package", initiative.ArchiveEligible)
		}
		if !strings.Contains(initiative.ArchiveReason, "WP-003") {
			t.Fatalf("archive reason = %q, want the missing WP-003 surfaced", initiative.ArchiveReason)
		}
	})

	t.Run("Should project the removed package as blocked in the dashboard", func(t *testing.T) {
		dashboard, err := service.Dashboard(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("Dashboard() error = %v", err)
		}
		card := findDashboardInitiativeCard(t, dashboard.Workflows, slug)
		assertMissingPlaceholderBlocked(t, card.Workflow.WorkPackages, "WP-003")
	})

	t.Run("Should refuse to really start the removed package", func(t *testing.T) {
		_, startErr := env.manager.StartTaskRun(
			context.Background(),
			env.workspaceRoot,
			slug,
			apicore.TaskRunRequest{
				Workspace:        env.workspaceRoot,
				PresentationMode: defaultPresentationMode,
				PackageID:        "WP-003",
			},
		)
		if !errors.Is(startErr, workpackages.ErrPackageNotFound) {
			t.Fatalf("StartTaskRun(removed WP-003) error = %v, want ErrPackageNotFound", startErr)
		}
	})

	t.Run("Should clear the missing state when the directory returns", func(t *testing.T) {
		env.writeWorkflowFile(
			t,
			slug,
			filepath.Join("_packages", "WP-003", "task_01.md"),
			daemonTaskBody("completed", "WP-003 task"),
		)
		if _, err := corepkg.SyncWithDB(context.Background(), env.globalDB, workspace, corepkg.SyncConfig{
			TasksDir: env.workflowDir(slug),
		}); err != nil {
			t.Fatalf("SyncWithDB(rematerialized) error = %v", err)
		}
		workflows, err := service.ListWorkflows(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ListWorkflows() error = %v", err)
		}
		initiative := findInitiativeSummary(t, workflows, slug)
		restored := findWorkPackageSummary(t, initiative.WorkPackages, "WP-003")
		if restored.StartBlockReason == workflowStartReasonMissing {
			t.Fatalf("rematerialized WP-003 still blocked missing: %q", restored.StartBlockReason)
		}
		if restored.ArchiveEligible == nil || !*restored.ArchiveEligible {
			t.Fatalf("rematerialized completed WP-003 archive eligible = %v, want true", restored.ArchiveEligible)
		}
	})
}

func TestTaskTransportServiceBlocksMaterializedEmptyWorkPackageStart(t *testing.T) {
	// A materialized-but-empty work package (directory present on disk, zero
	// executable tasks) must be projected as not runnable across the read models
	// with the no-executable-tasks reason, matching the runtime start preflight
	// that rejects the same package with package_no_executable_tasks. A sibling
	// package that carries a real pending task stays runnable, proving the
	// predicate is scoped to empty packages rather than blocking the initiative.
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error {
			return nil
		},
	})
	const slug = "wp-initiative"
	writeDaemonEmptyWorkPackageInitiative(t, env, slug)

	workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister() error = %v", err)
	}
	if _, err := corepkg.SyncWithDB(context.Background(), env.globalDB, workspace, corepkg.SyncConfig{
		TasksDir: env.workflowDir(slug),
	}); err != nil {
		t.Fatalf("SyncWithDB(materialized empty) error = %v", err)
	}

	query := NewQueryService(QueryServiceConfig{
		GlobalDB:   env.globalDB,
		RunManager: env.manager,
		Daemon: stubDaemonStatusReader{
			status: apicore.DaemonStatus{PID: 7, WorkspaceCount: 1},
			health: apicore.DaemonHealth{Ready: true},
		},
	})
	service := newTransportTaskService(env.globalDB, env.manager, query)

	t.Run("Should block the empty package start in the workflow list", func(t *testing.T) {
		workflows, err := service.ListWorkflows(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ListWorkflows() error = %v", err)
		}
		initiative := findInitiativeSummary(t, workflows, slug)
		assertEmptyPackageBlocked(t, initiative.WorkPackages, "WP-002")
		runnable := findWorkPackageSummary(t, initiative.WorkPackages, "WP-001")
		if runnable.CanStartRun == nil || !*runnable.CanStartRun || runnable.StartBlockReason != "" {
			t.Fatalf(
				"WP-001 can start = %v reason = %q, want the materialized package runnable",
				runnable.CanStartRun,
				runnable.StartBlockReason,
			)
		}
	})

	t.Run("Should block the empty package start in the dashboard", func(t *testing.T) {
		dashboard, err := service.Dashboard(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("Dashboard() error = %v", err)
		}
		card := findDashboardInitiativeCard(t, dashboard.Workflows, slug)
		assertEmptyPackageBlocked(t, card.Workflow.WorkPackages, "WP-002")
	})

	t.Run("Should refuse to really start the empty package", func(t *testing.T) {
		_, startErr := env.manager.StartTaskRun(
			context.Background(),
			env.workspaceRoot,
			slug,
			apicore.TaskRunRequest{
				Workspace:        env.workspaceRoot,
				PresentationMode: defaultPresentationMode,
				PackageID:        "WP-002",
			},
		)
		// The read-model block reason must agree with the endpoint the confirm
		// action calls; the endpoint rejects the empty package outright.
		var problem *apicore.Problem
		if !errors.As(startErr, &problem) || problem.Status != http.StatusUnprocessableEntity ||
			problem.Code != "package_no_executable_tasks" {
			t.Fatalf("StartTaskRun(empty WP-002) problem = %#v error = %v", problem, startErr)
		}
	})
}

// assertMissingPlaceholderBlocked verifies a missing-directory work-package
// placeholder is projected as not runnable with the missing-directory reason.
func assertMissingPlaceholderBlocked(
	t *testing.T,
	packages []apicore.WorkPackageSummary,
	packageID string,
) {
	t.Helper()
	placeholder := findWorkPackageSummary(t, packages, packageID)
	if placeholder.CanStartRun == nil || *placeholder.CanStartRun {
		t.Fatalf("%s can start run = %v, want blocked missing placeholder", packageID, placeholder.CanStartRun)
	}
	if placeholder.StartBlockReason != workflowStartReasonMissing {
		t.Fatalf(
			"%s start block reason = %q, want %q",
			packageID,
			placeholder.StartBlockReason,
			workflowStartReasonMissing,
		)
	}
}

// assertEmptyPackageBlocked verifies a materialized-but-empty work package is
// projected as not runnable with the no-executable-tasks reason.
func assertEmptyPackageBlocked(
	t *testing.T,
	packages []apicore.WorkPackageSummary,
	packageID string,
) {
	t.Helper()
	empty := findWorkPackageSummary(t, packages, packageID)
	if empty.CanStartRun == nil || *empty.CanStartRun {
		t.Fatalf("%s can start run = %v, want blocked empty package", packageID, empty.CanStartRun)
	}
	if empty.StartBlockReason != workflowStartReasonNoExecutableTasks {
		t.Fatalf(
			"%s start block reason = %q, want %q",
			packageID,
			empty.StartBlockReason,
			workflowStartReasonNoExecutableTasks,
		)
	}
}

func writeDaemonPartialWorkPackageInitiative(t *testing.T, env *runManagerTestEnv, slug string) {
	t.Helper()
	env.writeWorkflowFile(t, slug, "_prd.md", "# Initiative\n")
	env.writeWorkflowFile(t, slug, "_techspec.md", "# Initiative Techspec\n")
	env.writeWorkflowFile(t, slug, "_work_packages.md", strings.Join([]string{
		"---",
		"schema_version: compozy.work-packages/v1",
		"initiative: " + slug,
		"graph:",
		"  nodes:",
		"    - id: WP-001",
		"      directory: _packages/WP-001",
		"    - id: WP-002",
		"      directory: _packages/WP-002",
		"    - id: WP-003",
		"      directory: _packages/WP-003",
		"  edges:",
		"    - from: WP-001",
		"      to: WP-002",
		"      rationale: WP-002 consumes the WP-001 contract.",
		"---",
		"",
		"# Initiative Work Packages",
		"",
		"## [ ] WP-001 — Persistence",
		"",
		"- Reference: `" + slug + "/WP-001`",
		"- Outcome: Persist the parent workflow.",
		"- Owns:",
		"  - persistence",
		"- Dependencies: None",
		"",
		"## [ ] WP-002 — Archive",
		"",
		"- Reference: `" + slug + "/WP-002`",
		"- Outcome: Archive the aggregate workflow.",
		"- Owns:",
		"  - archive",
		"- Dependencies:",
		"  - `WP-001` — WP-002 consumes the WP-001 contract.",
		"",
		"## [x] WP-003 — Reporting",
		"",
		"- Reference: `" + slug + "/WP-003`",
		"- Outcome: Report aggregate status.",
		"- Owns:",
		"  - reporting",
		"- Dependencies: None",
		"",
	}, "\n"))
	// Only WP-002 has a materialized directory. WP-001 (a declared prerequisite of
	// WP-002) and WP-003 (declared but independent) are absent on this first sync.
	env.writeWorkflowFile(
		t,
		slug,
		filepath.Join("_packages", "WP-002", "task_01.md"),
		daemonTaskBody("pending", "WP-002 task"),
	)
}

func writeDaemonMaterializedWorkPackageInitiative(t *testing.T, env *runManagerTestEnv, slug string) {
	t.Helper()
	env.writeWorkflowFile(t, slug, "_prd.md", "# Initiative\n")
	env.writeWorkflowFile(t, slug, "_techspec.md", "# Initiative Techspec\n")
	env.writeWorkflowFile(t, slug, "_work_packages.md", strings.Join([]string{
		"---",
		"schema_version: compozy.work-packages/v1",
		"initiative: " + slug,
		"graph:",
		"  nodes:",
		"    - id: WP-001",
		"      directory: _packages/WP-001",
		"    - id: WP-002",
		"      directory: _packages/WP-002",
		"    - id: WP-003",
		"      directory: _packages/WP-003",
		"  edges:",
		"    - from: WP-001",
		"      to: WP-002",
		"      rationale: WP-002 consumes the WP-001 contract.",
		"---",
		"",
		"# Initiative Work Packages",
		"",
		"## [x] WP-001 — Persistence",
		"",
		"- Reference: `" + slug + "/WP-001`",
		"- Outcome: Persist the parent workflow.",
		"- Owns:",
		"  - persistence",
		"- Dependencies: None",
		"",
		"## [x] WP-002 — Archive",
		"",
		"- Reference: `" + slug + "/WP-002`",
		"- Outcome: Archive the aggregate workflow.",
		"- Owns:",
		"  - archive",
		"- Dependencies:",
		"  - `WP-001` — WP-002 consumes the WP-001 contract.",
		"",
		"## [x] WP-003 — Reporting",
		"",
		"- Reference: `" + slug + "/WP-003`",
		"- Outcome: Report aggregate status.",
		"- Owns:",
		"  - reporting",
		"- Dependencies: None",
		"",
	}, "\n"))
	// Every declared package is materialized and completed here so removing WP-003
	// later isolates the completed-then-vanished archive-eligibility path: the only
	// remaining archive blocker is the missing directory.
	for _, packageID := range []string{"WP-001", "WP-002", "WP-003"} {
		env.writeWorkflowFile(
			t,
			slug,
			filepath.Join("_packages", packageID, "task_01.md"),
			daemonTaskBody("completed", packageID+" task"),
		)
	}
}

func writeDaemonEmptyWorkPackageInitiative(t *testing.T, env *runManagerTestEnv, slug string) {
	t.Helper()
	env.writeWorkflowFile(t, slug, "_prd.md", "# Initiative\n")
	env.writeWorkflowFile(t, slug, "_techspec.md", "# Initiative Techspec\n")
	env.writeWorkflowFile(t, slug, "_work_packages.md", strings.Join([]string{
		"---",
		"schema_version: compozy.work-packages/v1",
		"initiative: " + slug,
		"graph:",
		"  nodes:",
		"    - id: WP-001",
		"      directory: _packages/WP-001",
		"    - id: WP-002",
		"      directory: _packages/WP-002",
		"  edges: []",
		"---",
		"",
		"# Initiative Work Packages",
		"",
		"## [ ] WP-001 — Persistence",
		"",
		"- Reference: `" + slug + "/WP-001`",
		"- Outcome: Persist the parent workflow.",
		"- Owns:",
		"  - persistence",
		"- Dependencies: None",
		"",
		"## [ ] WP-002 — Reporting",
		"",
		"- Reference: `" + slug + "/WP-002`",
		"- Outcome: Report aggregate status.",
		"- Owns:",
		"  - reporting",
		"- Dependencies: None",
		"",
	}, "\n"))
	// WP-001 carries a real pending task and stays runnable. WP-002's directory is
	// materialized but holds only a non-task artifact, so it resolves as present
	// (not missing) with zero executable tasks -- the drift case the read model
	// must block so its Start action agrees with the runtime start preflight.
	env.writeWorkflowFile(
		t,
		slug,
		filepath.Join("_packages", "WP-001", "task_01.md"),
		daemonTaskBody("pending", "WP-001 task"),
	)
	env.writeWorkflowFile(
		t,
		slug,
		filepath.Join("_packages", "WP-002", "_tasks.md"),
		"# Empty package\n",
	)
}

func findInitiativeSummary(
	t *testing.T,
	workflows []apicore.WorkflowSummary,
	slug string,
) apicore.WorkflowSummary {
	t.Helper()
	for i := range workflows {
		if workflows[i].Slug == slug && workflows[i].Kind == string(globaldb.WorkflowKindInitiative) {
			return workflows[i]
		}
	}
	t.Fatalf("initiative %q not found in %#v", slug, workflows)
	return apicore.WorkflowSummary{}
}

func findDashboardInitiativeCard(
	t *testing.T,
	cards []apicore.WorkflowCard,
	slug string,
) apicore.WorkflowCard {
	t.Helper()
	for i := range cards {
		if cards[i].Workflow.Slug == slug {
			return cards[i]
		}
	}
	t.Fatalf("initiative card %q not found in %#v", slug, cards)
	return apicore.WorkflowCard{}
}

func findWorkPackageSummary(
	t *testing.T,
	packages []apicore.WorkPackageSummary,
	packageID string,
) apicore.WorkPackageSummary {
	t.Helper()
	for i := range packages {
		if packages[i].PackageID == packageID {
			return packages[i]
		}
	}
	t.Fatalf("work package %q not found in %#v", packageID, packages)
	return apicore.WorkPackageSummary{}
}

func mustProblem(t *testing.T, err error) *apicore.Problem {
	t.Helper()

	var problem *apicore.Problem
	if !errors.As(err, &problem) {
		t.Fatalf("error = %T %v, want *core.Problem", err, err)
	}
	return problem
}

func sameCanonicalPath(t *testing.T, left string, right string) bool {
	t.Helper()

	leftResolved, err := filepath.EvalSymlinks(left)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q) error = %v", left, err)
	}
	rightResolved, err := filepath.EvalSymlinks(right)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q) error = %v", right, err)
	}
	return leftResolved == rightResolved
}
