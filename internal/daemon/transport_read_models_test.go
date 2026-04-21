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
	"github.com/compozy/compozy/internal/core/model"
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
		"task_1",
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

	_, err = service.TaskDetail(
		context.Background(),
		fixture.env.workspaceRoot,
		fixture.env.workflowSlug,
		"task_1",
	)
	missingProblem := mustProblem(t, err)
	if missingProblem.Status != http.StatusNotFound || missingProblem.Code != "document_not_found" {
		t.Fatalf("TaskDetail(document missing) = %#v, want 404 document_not_found", missingProblem)
	}
	if got := missingProblem.Details["relative_path"]; got != "task_01.md" {
		t.Fatalf("document_not_found relative_path = %#v, want task_01.md", got)
	}

	memoryPath := filepath.Join(fixture.env.workflowDir(fixture.env.workflowSlug), "memory", "MEMORY.md")
	if err := os.Remove(memoryPath); err != nil {
		t.Fatalf("Remove(memory/MEMORY.md) error = %v", err)
	}

	_, err = service.WorkflowMemoryFile(
		context.Background(),
		fixture.env.workspaceRoot,
		fixture.env.workflowSlug,
		workflowMemoryID,
	)
	staleProblem := mustProblem(t, err)
	if staleProblem.Status != http.StatusNotFound || staleProblem.Code != "stale_document_reference" {
		t.Fatalf("WorkflowMemoryFile(stale) = %#v, want 404 stale_document_reference", staleProblem)
	}
	if got := staleProblem.Details["reference"]; got != workflowMemoryID {
		t.Fatalf("stale_document_reference reference = %#v, want %q", got, workflowMemoryID)
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
	if len(runDetail.Runtime.Models) == 0 || runDetail.Runtime.Models[0] != "gpt-5.4" {
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
			TaskID:    "task_1",
			DependsOn: []string{"task_0"},
		},
		Document: MarkdownDocument{
			ID:    "task_1",
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
	if got := taskSource.Task.DependsOn[0]; got != "task_0" {
		t.Fatalf("source task depends_on mutated = %q, want task_0", got)
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
			Models:            []string{"gpt-5.4"},
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
	if got := runSource.Runtime.Models[0]; got != "gpt-5.4" {
		t.Fatalf("source run runtime models mutated = %q, want gpt-5.4", got)
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
			runArtifacts, err := model.ResolveHomeRunArtifacts(cfg.RunID)
			if err != nil {
				return err
			}

			submitEvent(ctx, t, prep.Journal(), cfg.RunID, eventspkg.EventKindJobQueued, kinds.JobQueuedPayload{
				Index:           1,
				SafeName:        "job-001",
				TaskTitle:       "Transport task A",
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
