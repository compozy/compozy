package core_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/pkg/compozy/events"
)

func TestSharedHandlersServiceErrorPaths(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	boom := errors.New("boom")

	testCases := []struct {
		name       string
		config     *core.HandlerConfig
		method     string
		target     string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			"daemon status service error",
			&core.HandlerConfig{Daemon: &errorDaemonService{err: boom}},
			http.MethodGet,
			"/api/daemon/status",
			"",
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"daemon health service error",
			&core.HandlerConfig{Daemon: &errorDaemonService{err: boom}},
			http.MethodGet,
			"/api/daemon/health",
			"",
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"daemon metrics service error",
			&core.HandlerConfig{Daemon: &errorDaemonService{err: boom}},
			http.MethodGet,
			"/api/daemon/metrics",
			"",
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"daemon stop service error",
			&core.HandlerConfig{Daemon: &errorDaemonService{err: boom}},
			http.MethodPost,
			"/api/daemon/stop?force=true",
			"",
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"workspace register service error",
			&core.HandlerConfig{Workspaces: &errorWorkspaceService{err: boom}},
			http.MethodPost,
			"/api/workspaces",
			`{"path":"/tmp/workspace","name":"workspace"}`,
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"workspace list service error",
			&core.HandlerConfig{Workspaces: &errorWorkspaceService{err: boom}},
			http.MethodGet,
			"/api/workspaces",
			"",
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"workspace get service error",
			&core.HandlerConfig{Workspaces: &errorWorkspaceService{err: boom}},
			http.MethodGet,
			"/api/workspaces/ws-1",
			"",
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"workspace update service error",
			&core.HandlerConfig{Workspaces: &errorWorkspaceService{err: boom}},
			http.MethodPatch,
			"/api/workspaces/ws-1",
			`{"name":"workspace"}`,
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"workspace delete service error",
			&core.HandlerConfig{Workspaces: &errorWorkspaceService{err: boom}},
			http.MethodDelete,
			"/api/workspaces/ws-1",
			"",
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"workspace resolve service error",
			&core.HandlerConfig{Workspaces: &errorWorkspaceService{err: boom}},
			http.MethodPost,
			"/api/workspaces/resolve",
			`{"path":"/tmp/workspace"}`,
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"dashboard service error",
			&core.HandlerConfig{Tasks: &errorTaskService{err: boom}},
			http.MethodGet,
			"/api/ui/dashboard?workspace=ws-1",
			"",
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"task workflows service error",
			&core.HandlerConfig{Tasks: &errorTaskService{err: boom}},
			http.MethodGet,
			"/api/tasks?workspace=ws-1",
			"",
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"task workflow service error",
			&core.HandlerConfig{Tasks: &errorTaskService{err: boom}},
			http.MethodGet,
			"/api/tasks/daemon?workspace=ws-1",
			"",
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"task spec service error",
			&core.HandlerConfig{Tasks: &errorTaskService{err: boom}},
			http.MethodGet,
			"/api/tasks/daemon/spec?workspace=ws-1",
			"",
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"task memory service error",
			&core.HandlerConfig{Tasks: &errorTaskService{err: boom}},
			http.MethodGet,
			"/api/tasks/daemon/memory?workspace=ws-1",
			"",
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"task memory file service error",
			&core.HandlerConfig{Tasks: &errorTaskService{err: boom}},
			http.MethodGet,
			"/api/tasks/daemon/memory/files/file-1?workspace=ws-1",
			"",
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"task board service error",
			&core.HandlerConfig{Tasks: &errorTaskService{err: boom}},
			http.MethodGet,
			"/api/tasks/daemon/board?workspace=ws-1",
			"",
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"task items service error",
			&core.HandlerConfig{Tasks: &errorTaskService{err: boom}},
			http.MethodGet,
			"/api/tasks/daemon/items?workspace=ws-1",
			"",
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"task item detail service error",
			&core.HandlerConfig{Tasks: &errorTaskService{err: boom}},
			http.MethodGet,
			"/api/tasks/daemon/items/task_01?workspace=ws-1",
			"",
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"task validate service error",
			&core.HandlerConfig{Tasks: &errorTaskService{err: boom}},
			http.MethodPost,
			"/api/tasks/daemon/validate",
			`{"workspace":"ws-1"}`,
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"task run service error",
			&core.HandlerConfig{Tasks: &errorTaskService{err: boom}},
			http.MethodPost,
			"/api/tasks/daemon/runs",
			`{"workspace":"ws-1","presentation_mode":"stream"}`,
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"task archive service error",
			&core.HandlerConfig{Tasks: &errorTaskService{err: boom}},
			http.MethodPost,
			"/api/tasks/daemon/archive",
			`{"workspace":"ws-1"}`,
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"task archive active run conflict",
			&core.HandlerConfig{Tasks: &errorTaskService{err: globaldb.WorkflowActiveRunsError{
				WorkspaceID: "ws-1",
				WorkflowID:  "wf-1",
				Slug:        "daemon",
				ActiveRuns:  1,
			}}},
			http.MethodPost,
			"/api/tasks/daemon/archive",
			`{"workspace":"ws-1"}`,
			http.StatusConflict,
			"conflict",
		},
		{
			"review fetch service error",
			&core.HandlerConfig{Reviews: &errorReviewService{err: boom}},
			http.MethodPost,
			"/api/reviews/daemon/fetch",
			`{"workspace":"ws-1","provider":"coderabbit"}`,
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"review latest service error",
			&core.HandlerConfig{Reviews: &errorReviewService{err: boom}},
			http.MethodGet,
			"/api/reviews/daemon?workspace=ws-1",
			"",
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"review round service error",
			&core.HandlerConfig{Reviews: &errorReviewService{err: boom}},
			http.MethodGet,
			"/api/reviews/daemon/rounds/1?workspace=ws-1",
			"",
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"review issues service error",
			&core.HandlerConfig{Reviews: &errorReviewService{err: boom}},
			http.MethodGet,
			"/api/reviews/daemon/rounds/1/issues?workspace=ws-1",
			"",
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"review issue detail service error",
			&core.HandlerConfig{Reviews: &errorReviewService{err: boom}},
			http.MethodGet,
			"/api/reviews/daemon/rounds/1/issues/issue-1?workspace=ws-1",
			"",
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"review run service error",
			&core.HandlerConfig{Reviews: &errorReviewService{err: boom}},
			http.MethodPost,
			"/api/reviews/daemon/rounds/1/runs",
			`{"workspace":"ws-1","presentation_mode":"stream"}`,
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"runs list service error",
			&core.HandlerConfig{Runs: &errorRunService{err: boom}},
			http.MethodGet,
			"/api/runs?workspace=ws-1&limit=10",
			"",
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"run snapshot service error",
			&core.HandlerConfig{Runs: &errorRunService{err: boom}},
			http.MethodGet,
			"/api/runs/run-1/snapshot",
			"",
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"stale workspace context returns precondition failed",
			&core.HandlerConfig{Tasks: &errorTaskService{err: globaldb.ErrWorkspaceNotFound}},
			http.MethodGet,
			"/api/ui/dashboard?workspace=ws-stale",
			"",
			http.StatusPreconditionFailed,
			"workspace_context_stale",
		},
		{
			"run events service error",
			&core.HandlerConfig{Runs: &errorRunService{err: boom}},
			http.MethodGet,
			"/api/runs/run-1/events?limit=10",
			"",
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"run cancel service error",
			&core.HandlerConfig{Runs: &errorRunService{err: boom}},
			http.MethodPost,
			"/api/runs/run-1/cancel",
			"",
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"sync service error",
			&core.HandlerConfig{Sync: &errorSyncService{err: boom}},
			http.MethodPost,
			"/api/sync",
			`{"workspace":"ws-1","workflow_slug":"daemon"}`,
			http.StatusInternalServerError,
			"internal_error",
		},
		{
			"sync validation error",
			&core.HandlerConfig{Sync: &errorSyncService{
				err: tasks.WrapParseError("/tmp/task_01.md", tasks.ErrV1TaskMetadata),
			}},
			http.MethodPost,
			"/api/sync",
			`{"workspace":"ws-1","workflow_slug":"daemon"}`,
			http.StatusUnprocessableEntity,
			"validation_error",
		},
		{
			"exec service error",
			&core.HandlerConfig{Exec: &errorExecService{err: boom}},
			http.MethodPost,
			"/api/exec",
			`{"workspace_path":"/tmp/workspace","prompt":"run","presentation_mode":"stream"}`,
			http.StatusInternalServerError,
			"internal_error",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			engine := gin.New()
			engine.Use(core.RequestIDMiddleware())
			engine.Use(core.ErrorMiddleware())
			core.RegisterRoutes(engine, core.NewHandlers(tc.config))

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

			var payload core.TransportError
			decodeJSON(t, response.Body.Bytes(), &payload)
			if payload.Code != tc.wantCode {
				t.Fatalf("payload.Code = %q, want %q", payload.Code, tc.wantCode)
			}
			if got := strings.TrimSpace(response.Header().Get(core.HeaderRequestID)); got == "" {
				t.Fatal("X-Request-Id header = empty, want non-empty")
			}
		})
	}
}

func TestStreamRunAdditionalBranches(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	t.Run("open stream error returns transport envelope", func(t *testing.T) {
		t.Parallel()

		engine := gin.New()
		engine.Use(core.RequestIDMiddleware())
		engine.Use(core.ErrorMiddleware())
		core.RegisterRoutes(engine, core.NewHandlers(&core.HandlerConfig{
			TransportName: "test",
			Runs: &fakeRunService{
				openStream: func(context.Context, string, core.StreamCursor) (core.RunStream, error) {
					return nil, errors.New("stream open failed")
				},
			},
		}))

		request := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"/api/runs/run-1/stream",
			http.NoBody,
		)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)

		if response.Code != http.StatusInternalServerError {
			t.Fatalf(
				"status = %d, want %d; body=%s",
				response.Code,
				http.StatusInternalServerError,
				response.Body.String(),
			)
		}

		var payload core.TransportError
		decodeJSON(t, response.Body.Bytes(), &payload)
		if payload.Code != "internal_error" {
			t.Fatalf("payload.Code = %q, want internal_error", payload.Code)
		}
	})

	t.Run("stream errors are emitted as SSE error events", func(t *testing.T) {
		t.Parallel()

		stream := newFakeRunStream()
		go func() {
			stream.errors <- nil
			stream.errors <- errors.New("stream exploded")
			close(stream.errors)
			close(stream.events)
		}()

		engine := gin.New()
		engine.Use(core.RequestIDMiddleware())
		engine.Use(core.ErrorMiddleware())
		core.RegisterRoutes(engine, core.NewHandlers(&core.HandlerConfig{
			TransportName: "test",
			Runs: &fakeRunService{
				openStream: func(context.Context, string, core.StreamCursor) (core.RunStream, error) {
					return stream, nil
				},
			},
		}))

		server := httptest.NewServer(engine)
		defer server.Close()

		request, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			server.URL+"/api/runs/run-1/stream",
			http.NoBody,
		)
		if err != nil {
			t.Fatalf("NewRequest() error = %v", err)
		}

		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		defer response.Body.Close()

		body := readAllString(t, response.Body)
		if !strings.Contains(body, "event: error") {
			t.Fatalf("stream missing error frame:\n%s", body)
		}
		if !strings.Contains(body, `"code":"internal_error"`) {
			t.Fatalf("stream missing internal_error payload:\n%s", body)
		}
	})

	t.Run("nil items are skipped and terminal events close the stream", func(t *testing.T) {
		t.Parallel()

		stream := newFakeRunStream()
		now := time.Date(2026, 4, 17, 16, 0, 0, 0, time.UTC)
		go func() {
			stream.events <- core.RunStreamItem{}
			stream.events <- core.RunStreamItem{
				Event: &events.Event{
					SchemaVersion: events.SchemaVersion,
					RunID:         "run-1",
					Seq:           9,
					Timestamp:     now,
					Kind:          events.EventKindRunCompleted,
					Payload:       []byte(`{"status":"completed"}`),
				},
			}
			close(stream.events)
			close(stream.errors)
		}()

		engine := gin.New()
		engine.Use(core.RequestIDMiddleware())
		engine.Use(core.ErrorMiddleware())
		core.RegisterRoutes(engine, core.NewHandlers(&core.HandlerConfig{
			TransportName:     "test",
			HeartbeatInterval: time.Second,
			Runs: &fakeRunService{
				openStream: func(context.Context, string, core.StreamCursor) (core.RunStream, error) {
					return stream, nil
				},
			},
		}))

		server := httptest.NewServer(engine)
		defer server.Close()

		request, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			server.URL+"/api/runs/run-1/stream",
			http.NoBody,
		)
		if err != nil {
			t.Fatalf("NewRequest() error = %v", err)
		}

		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		defer response.Body.Close()

		body := readAllString(t, response.Body)
		if strings.Contains(body, "event: "+core.RunHeartbeatSSEEvent) {
			t.Fatalf("stream unexpectedly emitted a heartbeat:\n%s", body)
		}
		if !strings.Contains(body, "event: "+core.RunEventSSEEvent) {
			t.Fatalf("stream missing terminal stream event:\n%s", body)
		}
		if !strings.Contains(body, `"kind":"run.completed"`) || !strings.Contains(body, `"seq":9`) {
			t.Fatalf("stream missing terminal payload:\n%s", body)
		}
	})
}

type errorDaemonService struct {
	err error
}

func (s *errorDaemonService) Status(context.Context) (core.DaemonStatus, error) {
	return core.DaemonStatus{}, s.err
}

func (s *errorDaemonService) Health(context.Context) (core.DaemonHealth, error) {
	return core.DaemonHealth{}, s.err
}

func (s *errorDaemonService) Metrics(context.Context) (core.MetricsPayload, error) {
	return core.MetricsPayload{}, s.err
}

func (s *errorDaemonService) Stop(context.Context, bool) error {
	return s.err
}

type errorWorkspaceService struct {
	err error
}

func (s *errorWorkspaceService) Register(context.Context, string, string) (core.WorkspaceRegisterResult, error) {
	return core.WorkspaceRegisterResult{}, s.err
}

func (s *errorWorkspaceService) List(context.Context) ([]core.Workspace, error) {
	return nil, s.err
}

func (s *errorWorkspaceService) Get(context.Context, string) (core.Workspace, error) {
	return core.Workspace{}, s.err
}

func (s *errorWorkspaceService) Update(context.Context, string, core.WorkspaceUpdateInput) (core.Workspace, error) {
	return core.Workspace{}, s.err
}

func (s *errorWorkspaceService) Delete(context.Context, string) error {
	return s.err
}

func (s *errorWorkspaceService) Resolve(context.Context, string) (core.Workspace, error) {
	return core.Workspace{}, s.err
}

type errorTaskService struct {
	err error
}

func (s *errorTaskService) Dashboard(context.Context, string) (core.DashboardPayload, error) {
	return core.DashboardPayload{}, s.err
}

func (s *errorTaskService) ListWorkflows(context.Context, string) ([]core.WorkflowSummary, error) {
	return nil, s.err
}

func (s *errorTaskService) GetWorkflow(context.Context, string, string) (core.WorkflowSummary, error) {
	return core.WorkflowSummary{}, s.err
}

func (s *errorTaskService) WorkflowOverview(context.Context, string, string) (core.WorkflowOverviewPayload, error) {
	return core.WorkflowOverviewPayload{}, s.err
}

func (s *errorTaskService) ListItems(context.Context, string, string) ([]core.TaskItem, error) {
	return nil, s.err
}

func (s *errorTaskService) TaskBoard(context.Context, string, string) (core.TaskBoardPayload, error) {
	return core.TaskBoardPayload{}, s.err
}

func (s *errorTaskService) WorkflowSpec(context.Context, string, string) (core.WorkflowSpecDocument, error) {
	return core.WorkflowSpecDocument{}, s.err
}

func (s *errorTaskService) WorkflowMemoryIndex(context.Context, string, string) (core.WorkflowMemoryIndex, error) {
	return core.WorkflowMemoryIndex{}, s.err
}

func (s *errorTaskService) WorkflowMemoryFile(context.Context, string, string, string) (core.MarkdownDocument, error) {
	return core.MarkdownDocument{}, s.err
}

func (s *errorTaskService) TaskDetail(context.Context, string, string, string) (core.TaskDetailPayload, error) {
	return core.TaskDetailPayload{}, s.err
}

func (s *errorTaskService) Validate(context.Context, string, string) (core.ValidationSuccess, error) {
	return core.ValidationSuccess{}, s.err
}

func (s *errorTaskService) StartRun(context.Context, string, string, core.TaskRunRequest) (core.Run, error) {
	return core.Run{}, s.err
}

func (s *errorTaskService) Archive(context.Context, string, string) (core.ArchiveResult, error) {
	return core.ArchiveResult{}, s.err
}

type errorReviewService struct {
	err error
}

func (s *errorReviewService) Fetch(
	context.Context,
	string,
	string,
	core.ReviewFetchRequest,
) (core.ReviewFetchResult, error) {
	return core.ReviewFetchResult{}, s.err
}

func (s *errorReviewService) GetLatest(context.Context, string, string) (core.ReviewSummary, error) {
	return core.ReviewSummary{}, s.err
}

func (s *errorReviewService) GetRound(context.Context, string, string, int) (core.ReviewRound, error) {
	return core.ReviewRound{}, s.err
}

func (s *errorReviewService) ListIssues(context.Context, string, string, int) ([]core.ReviewIssue, error) {
	return nil, s.err
}

func (s *errorReviewService) ReviewDetail(
	context.Context,
	string,
	string,
	int,
	string,
) (core.ReviewDetailPayload, error) {
	return core.ReviewDetailPayload{}, s.err
}

func (s *errorReviewService) StartRun(context.Context, string, string, int, core.ReviewRunRequest) (core.Run, error) {
	return core.Run{}, s.err
}

type errorRunService struct {
	err error
}

func (s *errorRunService) List(context.Context, core.RunListQuery) ([]core.Run, error) {
	return nil, s.err
}

func (s *errorRunService) Get(context.Context, string) (core.Run, error) {
	return core.Run{}, s.err
}

func (s *errorRunService) Snapshot(context.Context, string) (core.RunSnapshot, error) {
	return core.RunSnapshot{}, s.err
}

func (s *errorRunService) RunDetail(context.Context, string) (core.RunDetailPayload, error) {
	return core.RunDetailPayload{}, s.err
}

func (s *errorRunService) Events(context.Context, string, core.RunEventPageQuery) (core.RunEventPage, error) {
	return core.RunEventPage{}, s.err
}

func (s *errorRunService) OpenStream(context.Context, string, core.StreamCursor) (core.RunStream, error) {
	return nil, s.err
}

func (s *errorRunService) Cancel(context.Context, string) error {
	return s.err
}

type errorSyncService struct {
	err error
}

func (s *errorSyncService) Sync(context.Context, core.SyncRequest) (core.SyncResult, error) {
	return core.SyncResult{}, s.err
}

type errorExecService struct {
	err error
}

func (s *errorExecService) Start(context.Context, core.ExecRequest) (core.Run, error) {
	return core.Run{}, s.err
}

func readAllString(t *testing.T, reader io.Reader) string {
	t.Helper()

	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	return string(body)
}
