package core_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/pkg/compozy/events"
)

func TestSharedHandlersSmokeSuccessPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Date(2026, 4, 17, 13, 0, 0, 0, time.UTC)
	nextCursor := core.StreamCursor{Timestamp: now.Add(time.Second), Sequence: 2}

	handlers := core.NewHandlers(&core.HandlerConfig{
		TransportName: "smoke",
		Daemon: &smokeDaemonService{
			status: core.DaemonStatus{
				PID:            123,
				Version:        "test",
				StartedAt:      now,
				SocketPath:     "/tmp/compozy.sock",
				HTTPPort:       4321,
				ActiveRunCount: 1,
				WorkspaceCount: 1,
			},
			health: core.DaemonHealth{Ready: true},
			metrics: core.MetricsPayload{
				Body:        "daemon_active_runs 1\n",
				ContentType: "text/plain; version=0.0.4; charset=utf-8",
			},
		},
		Workspaces: &smokeWorkspaceService{
			workspace: core.Workspace{
				ID:        "ws-1",
				RootDir:   "/tmp/workspace",
				Name:      "workspace",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		Tasks: &smokeTaskService{
			workflow: core.WorkflowSummary{
				ID:           "wf-1",
				WorkspaceID:  "ws-1",
				Slug:         "daemon",
				LastSyncedAt: ptrTime(now),
			},
			item: core.TaskItem{
				ID:         "task-1",
				TaskNumber: 1,
				TaskID:     "task_01",
				Title:      "Shared Transport Core",
				Status:     "pending",
				Type:       "backend",
				DependsOn:  []string{"task_00"},
				SourcePath: ".compozy/tasks/daemon/task_01.md",
				UpdatedAt:  now,
			},
			run: core.Run{
				RunID:            "run-1",
				WorkspaceID:      "ws-1",
				Mode:             "task",
				Status:           "running",
				PresentationMode: "stream",
				StartedAt:        now,
			},
		},
		Reviews: &smokeReviewService{
			summary: core.ReviewSummary{
				WorkflowSlug:    "daemon",
				RoundNumber:     1,
				Provider:        "coderabbit",
				ResolvedCount:   1,
				UnresolvedCount: 2,
				UpdatedAt:       now,
			},
			round: core.ReviewRound{
				ID:              "round-1",
				WorkflowSlug:    "daemon",
				RoundNumber:     1,
				Provider:        "coderabbit",
				ResolvedCount:   1,
				UnresolvedCount: 2,
				UpdatedAt:       now,
			},
			issue: core.ReviewIssue{
				ID:          "issue-1",
				IssueNumber: 1,
				Severity:    "high",
				Status:      "open",
				SourcePath:  ".compozy/tasks/daemon/reviews-001/issue_001.md",
				UpdatedAt:   now,
			},
			run: core.Run{
				RunID:            "review-run-1",
				WorkspaceID:      "ws-1",
				Mode:             "review",
				Status:           "running",
				PresentationMode: "stream",
				StartedAt:        now,
			},
		},
		Runs: &smokeRunService{
			run: core.Run{
				RunID:            "run-1",
				WorkspaceID:      "ws-1",
				Mode:             "task",
				Status:           "running",
				PresentationMode: "stream",
				StartedAt:        now,
			},
			snapshot: core.RunSnapshot{
				Run: core.Run{
					RunID:            "run-1",
					WorkspaceID:      "ws-1",
					Mode:             "task",
					Status:           "running",
					PresentationMode: "stream",
					StartedAt:        now,
				},
				Jobs: []core.RunJobState{{
					JobID:     "job-1",
					Status:    "running",
					UpdatedAt: now,
				}},
				Transcript: []core.RunTranscriptMessage{{
					Sequence:  1,
					Stream:    "session",
					Role:      "assistant",
					Content:   "hello",
					Timestamp: now,
				}},
				NextCursor: &nextCursor,
			},
			page: core.RunEventPage{
				Events: []events.Event{
					{
						SchemaVersion: events.SchemaVersion,
						RunID:         "run-1",
						Seq:           1,
						Timestamp:     now,
						Kind:          events.EventKindRunStarted,
						Payload:       json.RawMessage(`{"status":"started"}`),
					},
				},
				NextCursor: &nextCursor,
				HasMore:    true,
			},
		},
		Sync: &smokeSyncService{
			result: core.SyncResult{
				WorkspaceID:  "ws-1",
				WorkflowSlug: "daemon",
				SyncedAt:     ptrTime(now),
			},
		},
		Exec: &smokeExecService{
			run: core.Run{
				RunID:            "exec-run-1",
				WorkspaceID:      "ws-1",
				Mode:             "exec",
				Status:           "running",
				PresentationMode: "stream",
				StartedAt:        now,
			},
		},
	})

	engine := gin.New()
	engine.Use(core.RequestIDMiddleware())
	engine.Use(core.ErrorMiddleware())
	core.RegisterRoutes(engine, handlers)

	testCases := []struct {
		name        string
		method      string
		target      string
		body        string
		wantStatus  int
		wantContain string
	}{
		{"daemon status", http.MethodGet, "/api/daemon/status", "", http.StatusOK, `"pid":123`},
		{"daemon health", http.MethodGet, "/api/daemon/health", "", http.StatusOK, `"ready":true`},
		{"daemon metrics", http.MethodGet, "/api/daemon/metrics", "", http.StatusOK, `daemon_active_runs 1`},
		{"daemon stop", http.MethodPost, "/api/daemon/stop?force=true", "", http.StatusAccepted, `"accepted":true`},
		{
			"workspace register",
			http.MethodPost,
			"/api/workspaces",
			`{"path":"/tmp/workspace","name":"workspace"}`,
			http.StatusCreated,
			`"workspace":{"id":"ws-1"`,
		},
		{"workspace list", http.MethodGet, "/api/workspaces", "", http.StatusOK, `"workspaces":[`},
		{"workspace get", http.MethodGet, "/api/workspaces/ws-1", "", http.StatusOK, `"root_dir":"/tmp/workspace"`},
		{
			"workspace update",
			http.MethodPatch,
			"/api/workspaces/ws-1",
			`{"name":"workspace"}`,
			http.StatusOK,
			`"name":"workspace"`,
		},
		{"workspace delete", http.MethodDelete, "/api/workspaces/ws-1", "", http.StatusNoContent, ``},
		{
			"workspace resolve",
			http.MethodPost,
			"/api/workspaces/resolve",
			`{"path":"/tmp/workspace"}`,
			http.StatusOK,
			`"workspace":{"id":"ws-1"`,
		},
		{"task workflows", http.MethodGet, "/api/tasks?workspace=ws-1", "", http.StatusOK, `"workflows":[`},
		{"task workflow", http.MethodGet, "/api/tasks/daemon?workspace=ws-1", "", http.StatusOK, `"slug":"daemon"`},
		{
			"task items",
			http.MethodGet,
			"/api/tasks/daemon/items?workspace=ws-1",
			"",
			http.StatusOK,
			`"task_id":"task_01"`,
		},
		{
			"task validate",
			http.MethodPost,
			"/api/tasks/daemon/validate",
			`{"workspace":"ws-1"}`,
			http.StatusOK,
			`"valid":true`,
		},
		{
			"task run",
			http.MethodPost,
			"/api/tasks/daemon/runs",
			`{"workspace":"ws-1","presentation_mode":"stream"}`,
			http.StatusCreated,
			`"run":{"run_id":"run-1"`,
		},
		{
			"task archive",
			http.MethodPost,
			"/api/tasks/daemon/archive",
			`{"workspace":"ws-1"}`,
			http.StatusOK,
			`"archived":true`,
		},
		{
			"review fetch",
			http.MethodPost,
			"/api/reviews/daemon/fetch",
			`{"workspace":"ws-1","provider":"coderabbit"}`,
			http.StatusCreated,
			`"review":{"workflow_slug":"daemon"`,
		},
		{
			"review latest",
			http.MethodGet,
			"/api/reviews/daemon?workspace=ws-1",
			"",
			http.StatusOK,
			`"workflow_slug":"daemon"`,
		},
		{
			"review round",
			http.MethodGet,
			"/api/reviews/daemon/rounds/1?workspace=ws-1",
			"",
			http.StatusOK,
			`"round_number":1`,
		},
		{
			"review issues",
			http.MethodGet,
			"/api/reviews/daemon/rounds/1/issues?workspace=ws-1",
			"",
			http.StatusOK,
			`"issues":[`,
		},
		{
			"review run",
			http.MethodPost,
			"/api/reviews/daemon/rounds/1/runs",
			`{"workspace":"ws-1","presentation_mode":"stream"}`,
			http.StatusCreated,
			`"run":{"run_id":"review-run-1"`,
		},
		{"runs list", http.MethodGet, "/api/runs?workspace=ws-1&limit=10", "", http.StatusOK, `"runs":[`},
		{"run get", http.MethodGet, "/api/runs/run-1", "", http.StatusOK, `"run":{"run_id":"run-1"`},
		{"run snapshot", http.MethodGet, "/api/runs/run-1/snapshot", "", http.StatusOK, `"next_cursor":"`},
		{"run events", http.MethodGet, "/api/runs/run-1/events?limit=10", "", http.StatusOK, `"has_more":true`},
		{"run cancel", http.MethodPost, "/api/runs/run-1/cancel", "", http.StatusAccepted, `"accepted":true`},
		{
			"sync",
			http.MethodPost,
			"/api/sync",
			`{"workspace":"ws-1","workflow_slug":"daemon"}`,
			http.StatusOK,
			`"workflow_slug":"daemon"`,
		},
		{
			"exec",
			http.MethodPost,
			"/api/exec",
			`{"workspace_path":"/tmp/workspace","prompt":"run","presentation_mode":"stream"}`,
			http.StatusCreated,
			`"run":{"run_id":"exec-run-1"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(
				context.Background(),
				tc.method,
				tc.target,
				strings.NewReader(tc.body),
			)
			if tc.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			response := httptest.NewRecorder()

			engine.ServeHTTP(response, request)

			if response.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, tc.wantStatus, response.Body.String())
			}
			if got := strings.TrimSpace(response.Header().Get(core.HeaderRequestID)); got == "" {
				t.Fatal("X-Request-Id header = empty, want non-empty")
			}
			if tc.wantContain != "" && !strings.Contains(response.Body.String(), tc.wantContain) {
				t.Fatalf("response body missing %q:\n%s", tc.wantContain, response.Body.String())
			}
		})
	}
}

type smokeDaemonService struct {
	status  core.DaemonStatus
	health  core.DaemonHealth
	metrics core.MetricsPayload
}

func (s *smokeDaemonService) Status(context.Context) (core.DaemonStatus, error) {
	return s.status, nil
}

func (s *smokeDaemonService) Health(context.Context) (core.DaemonHealth, error) {
	return s.health, nil
}

func (s *smokeDaemonService) Metrics(context.Context) (core.MetricsPayload, error) {
	return s.metrics, nil
}

func (*smokeDaemonService) Stop(context.Context, bool) error {
	return nil
}

type smokeWorkspaceService struct {
	workspace core.Workspace
}

func (s *smokeWorkspaceService) Register(context.Context, string, string) (core.WorkspaceRegisterResult, error) {
	return core.WorkspaceRegisterResult{Workspace: s.workspace, Created: true}, nil
}

func (s *smokeWorkspaceService) List(context.Context) ([]core.Workspace, error) {
	return []core.Workspace{s.workspace}, nil
}

func (s *smokeWorkspaceService) Get(context.Context, string) (core.Workspace, error) {
	return s.workspace, nil
}

func (s *smokeWorkspaceService) Update(context.Context, string, core.WorkspaceUpdateInput) (core.Workspace, error) {
	return s.workspace, nil
}

func (*smokeWorkspaceService) Delete(context.Context, string) error {
	return nil
}

func (s *smokeWorkspaceService) Resolve(context.Context, string) (core.Workspace, error) {
	return s.workspace, nil
}

type smokeTaskService struct {
	workflow core.WorkflowSummary
	item     core.TaskItem
	run      core.Run
}

func (s *smokeTaskService) Dashboard(context.Context, string) (core.DashboardPayload, error) {
	return core.DashboardPayload{}, nil
}

func (s *smokeTaskService) ListWorkflows(context.Context, string) ([]core.WorkflowSummary, error) {
	return []core.WorkflowSummary{s.workflow}, nil
}

func (s *smokeTaskService) GetWorkflow(context.Context, string, string) (core.WorkflowSummary, error) {
	return s.workflow, nil
}

func (s *smokeTaskService) WorkflowOverview(context.Context, string, string) (core.WorkflowOverviewPayload, error) {
	return core.WorkflowOverviewPayload{Workflow: s.workflow}, nil
}

func (s *smokeTaskService) ListItems(context.Context, string, string) ([]core.TaskItem, error) {
	return []core.TaskItem{s.item}, nil
}

func (s *smokeTaskService) TaskBoard(context.Context, string, string) (core.TaskBoardPayload, error) {
	return core.TaskBoardPayload{}, nil
}

func (s *smokeTaskService) WorkflowSpec(context.Context, string, string) (core.WorkflowSpecDocument, error) {
	return core.WorkflowSpecDocument{}, nil
}

func (s *smokeTaskService) WorkflowMemoryIndex(context.Context, string, string) (core.WorkflowMemoryIndex, error) {
	return core.WorkflowMemoryIndex{}, nil
}

func (s *smokeTaskService) WorkflowMemoryFile(context.Context, string, string, string) (core.MarkdownDocument, error) {
	return core.MarkdownDocument{}, nil
}

func (s *smokeTaskService) TaskDetail(context.Context, string, string, string) (core.TaskDetailPayload, error) {
	return core.TaskDetailPayload{}, nil
}

func (*smokeTaskService) Validate(context.Context, string, string) (core.ValidationSuccess, error) {
	return core.ValidationSuccess{Valid: true}, nil
}

func (s *smokeTaskService) StartRun(context.Context, string, string, core.TaskRunRequest) (core.Run, error) {
	return s.run, nil
}

func (*smokeTaskService) Archive(context.Context, string, string) (core.ArchiveResult, error) {
	return core.ArchiveResult{Archived: true}, nil
}

type smokeReviewService struct {
	summary core.ReviewSummary
	round   core.ReviewRound
	issue   core.ReviewIssue
	run     core.Run
}

func (s *smokeReviewService) Fetch(
	context.Context,
	string,
	string,
	core.ReviewFetchRequest,
) (core.ReviewFetchResult, error) {
	return core.ReviewFetchResult{Summary: s.summary, Created: true}, nil
}

func (s *smokeReviewService) GetLatest(context.Context, string, string) (core.ReviewSummary, error) {
	return s.summary, nil
}

func (s *smokeReviewService) GetRound(context.Context, string, string, int) (core.ReviewRound, error) {
	return s.round, nil
}

func (s *smokeReviewService) ListIssues(context.Context, string, string, int) ([]core.ReviewIssue, error) {
	return []core.ReviewIssue{s.issue}, nil
}

func (s *smokeReviewService) ReviewDetail(
	context.Context,
	string,
	string,
	int,
	string,
) (core.ReviewDetailPayload, error) {
	return core.ReviewDetailPayload{}, nil
}

func (s *smokeReviewService) StartRun(context.Context, string, string, int, core.ReviewRunRequest) (core.Run, error) {
	return s.run, nil
}

type smokeRunService struct {
	run      core.Run
	snapshot core.RunSnapshot
	page     core.RunEventPage
}

func (s *smokeRunService) List(context.Context, core.RunListQuery) ([]core.Run, error) {
	return []core.Run{s.run}, nil
}

func (s *smokeRunService) Get(context.Context, string) (core.Run, error) {
	return s.run, nil
}

func (s *smokeRunService) Snapshot(context.Context, string) (core.RunSnapshot, error) {
	return s.snapshot, nil
}

func (s *smokeRunService) RunDetail(context.Context, string) (core.RunDetailPayload, error) {
	return core.RunDetailPayload{Run: s.run, Snapshot: s.snapshot}, nil
}

func (s *smokeRunService) Events(context.Context, string, core.RunEventPageQuery) (core.RunEventPage, error) {
	return s.page, nil
}

func (*smokeRunService) OpenStream(context.Context, string, core.StreamCursor) (core.RunStream, error) {
	stream := newFakeRunStream()
	close(stream.events)
	close(stream.errors)
	return stream, nil
}

func (*smokeRunService) Cancel(context.Context, string) error {
	return nil
}

type smokeSyncService struct {
	result core.SyncResult
}

func (s *smokeSyncService) Sync(context.Context, core.SyncRequest) (core.SyncResult, error) {
	return s.result, nil
}

type smokeExecService struct {
	run core.Run
}

func (s *smokeExecService) Start(context.Context, core.ExecRequest) (core.Run, error) {
	return s.run, nil
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
