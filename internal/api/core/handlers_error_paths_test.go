package core_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/core"
)

func TestSharedHandlersValidationAndServiceErrors(t *testing.T) {
	previousMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

	workspaceSvc := &smokeWorkspaceService{}
	taskSvc := &smokeTaskService{}
	reviewSvc := &smokeReviewService{}
	runSvc := &smokeRunService{}
	syncSvc := &smokeSyncService{}
	execSvc := &smokeExecService{}
	daemonSvc := &smokeDaemonService{}

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
			"daemon status service unavailable",
			&core.HandlerConfig{},
			http.MethodGet,
			"/api/daemon/status",
			"",
			http.StatusServiceUnavailable,
			"service_unavailable",
		},
		{
			"daemon health service unavailable",
			&core.HandlerConfig{},
			http.MethodGet,
			"/api/daemon/health",
			"",
			http.StatusServiceUnavailable,
			"service_unavailable",
		},
		{
			"daemon metrics service unavailable",
			&core.HandlerConfig{},
			http.MethodGet,
			"/api/daemon/metrics",
			"",
			http.StatusServiceUnavailable,
			"service_unavailable",
		},
		{
			"daemon stop invalid force",
			&core.HandlerConfig{Daemon: daemonSvc},
			http.MethodPost,
			"/api/daemon/stop?force=bad",
			"",
			http.StatusUnprocessableEntity,
			"force_invalid",
		},
		{
			"workspace register invalid json",
			&core.HandlerConfig{Workspaces: workspaceSvc},
			http.MethodPost,
			"/api/workspaces",
			"{",
			http.StatusBadRequest,
			"invalid_request",
		},
		{
			"workspace list service unavailable",
			&core.HandlerConfig{},
			http.MethodGet,
			"/api/workspaces",
			"",
			http.StatusServiceUnavailable,
			"service_unavailable",
		},
		{
			"workspace get service unavailable",
			&core.HandlerConfig{},
			http.MethodGet,
			"/api/workspaces/ws-1",
			"",
			http.StatusServiceUnavailable,
			"service_unavailable",
		},
		{
			"workspace update missing name",
			&core.HandlerConfig{Workspaces: workspaceSvc},
			http.MethodPatch,
			"/api/workspaces/ws-1",
			`{"name":""}`,
			http.StatusUnprocessableEntity,
			"name_required",
		},
		{
			"workspace delete service unavailable",
			&core.HandlerConfig{},
			http.MethodDelete,
			"/api/workspaces/ws-1",
			"",
			http.StatusServiceUnavailable,
			"service_unavailable",
		},
		{
			"workspace resolve missing path",
			&core.HandlerConfig{Workspaces: workspaceSvc},
			http.MethodPost,
			"/api/workspaces/resolve",
			`{"path":""}`,
			http.StatusUnprocessableEntity,
			"path_required",
		},
		{
			"dashboard missing workspace",
			&core.HandlerConfig{Tasks: taskSvc},
			http.MethodGet,
			"/api/ui/dashboard",
			"",
			http.StatusPreconditionFailed,
			"workspace_context_missing",
		},
		{
			"task workflows missing workspace",
			&core.HandlerConfig{Tasks: taskSvc},
			http.MethodGet,
			"/api/tasks",
			"",
			http.StatusPreconditionFailed,
			"workspace_context_missing",
		},
		{
			"task workflow missing workspace",
			&core.HandlerConfig{Tasks: taskSvc},
			http.MethodGet,
			"/api/tasks/daemon",
			"",
			http.StatusPreconditionFailed,
			"workspace_context_missing",
		},
		{
			"task spec missing workspace",
			&core.HandlerConfig{Tasks: taskSvc},
			http.MethodGet,
			"/api/tasks/daemon/spec",
			"",
			http.StatusPreconditionFailed,
			"workspace_context_missing",
		},
		{
			"task memory missing workspace",
			&core.HandlerConfig{Tasks: taskSvc},
			http.MethodGet,
			"/api/tasks/daemon/memory",
			"",
			http.StatusPreconditionFailed,
			"workspace_context_missing",
		},
		{
			"task memory file missing workspace",
			&core.HandlerConfig{Tasks: taskSvc},
			http.MethodGet,
			"/api/tasks/daemon/memory/files/file-1",
			"",
			http.StatusPreconditionFailed,
			"workspace_context_missing",
		},
		{
			"task board missing workspace",
			&core.HandlerConfig{Tasks: taskSvc},
			http.MethodGet,
			"/api/tasks/daemon/board",
			"",
			http.StatusPreconditionFailed,
			"workspace_context_missing",
		},
		{
			"task items missing workspace",
			&core.HandlerConfig{Tasks: taskSvc},
			http.MethodGet,
			"/api/tasks/daemon/items",
			"",
			http.StatusPreconditionFailed,
			"workspace_context_missing",
		},
		{
			"task item detail missing workspace",
			&core.HandlerConfig{Tasks: taskSvc},
			http.MethodGet,
			"/api/tasks/daemon/items/task_01",
			"",
			http.StatusPreconditionFailed,
			"workspace_context_missing",
		},
		{
			"task validate missing workspace",
			&core.HandlerConfig{Tasks: taskSvc},
			http.MethodPost,
			"/api/tasks/daemon/validate",
			`{}`,
			http.StatusUnprocessableEntity,
			"workspace_required",
		},
		{
			"task run missing workspace",
			&core.HandlerConfig{Tasks: taskSvc},
			http.MethodPost,
			"/api/tasks/daemon/runs",
			`{}`,
			http.StatusPreconditionFailed,
			"workspace_context_missing",
		},
		{
			"task archive missing workspace",
			&core.HandlerConfig{Tasks: taskSvc},
			http.MethodPost,
			"/api/tasks/daemon/archive",
			`{}`,
			http.StatusPreconditionFailed,
			"workspace_context_missing",
		},
		{
			"review fetch missing workspace",
			&core.HandlerConfig{Reviews: reviewSvc},
			http.MethodPost,
			"/api/reviews/daemon/fetch",
			`{}`,
			http.StatusUnprocessableEntity,
			"workspace_required",
		},
		{
			"review latest missing workspace",
			&core.HandlerConfig{Reviews: reviewSvc},
			http.MethodGet,
			"/api/reviews/daemon",
			"",
			http.StatusPreconditionFailed,
			"workspace_context_missing",
		},
		{
			"review round invalid round",
			&core.HandlerConfig{Reviews: reviewSvc},
			http.MethodGet,
			"/api/reviews/daemon/rounds/bad?workspace=ws-1",
			"",
			http.StatusUnprocessableEntity,
			"round_invalid",
		},
		{
			"review fetch invalid round",
			&core.HandlerConfig{Reviews: reviewSvc},
			http.MethodPost,
			"/api/reviews/daemon/fetch",
			`{"workspace":"ws-1","round":0}`,
			http.StatusUnprocessableEntity,
			"round_invalid",
		},
		{
			"review issues invalid round",
			&core.HandlerConfig{Reviews: reviewSvc},
			http.MethodGet,
			"/api/reviews/daemon/rounds/bad/issues?workspace=ws-1",
			"",
			http.StatusUnprocessableEntity,
			"round_invalid",
		},
		{
			"review issue detail missing workspace",
			&core.HandlerConfig{Reviews: reviewSvc},
			http.MethodGet,
			"/api/reviews/daemon/rounds/1/issues/issue-1",
			"",
			http.StatusPreconditionFailed,
			"workspace_context_missing",
		},
		{
			"review run missing workspace",
			&core.HandlerConfig{Reviews: reviewSvc},
			http.MethodPost,
			"/api/reviews/daemon/rounds/1/runs",
			`{}`,
			http.StatusPreconditionFailed,
			"workspace_context_missing",
		},
		{
			"runs list invalid limit",
			&core.HandlerConfig{Runs: runSvc},
			http.MethodGet,
			"/api/runs?limit=bad",
			"",
			http.StatusUnprocessableEntity,
			"limit_invalid",
		},
		{
			"runs list oversized limit",
			&core.HandlerConfig{Runs: runSvc},
			http.MethodGet,
			"/api/runs?limit=501",
			"",
			http.StatusUnprocessableEntity,
			"limit_invalid",
		},
		{
			"run get service unavailable",
			&core.HandlerConfig{},
			http.MethodGet,
			"/api/runs/run-1",
			"",
			http.StatusServiceUnavailable,
			"service_unavailable",
		},
		{
			"run snapshot service unavailable",
			&core.HandlerConfig{},
			http.MethodGet,
			"/api/runs/run-1/snapshot",
			"",
			http.StatusServiceUnavailable,
			"service_unavailable",
		},
		{
			"run events invalid cursor",
			&core.HandlerConfig{Runs: runSvc},
			http.MethodGet,
			"/api/runs/run-1/events?after=bad",
			"",
			http.StatusUnprocessableEntity,
			"invalid_cursor",
		},
		{
			"run events oversized limit",
			&core.HandlerConfig{Runs: runSvc},
			http.MethodGet,
			"/api/runs/run-1/events?limit=501",
			"",
			http.StatusUnprocessableEntity,
			"limit_invalid",
		},
		{
			"run cancel service unavailable",
			&core.HandlerConfig{},
			http.MethodPost,
			"/api/runs/run-1/cancel",
			"",
			http.StatusServiceUnavailable,
			"service_unavailable",
		},
		{
			"sync missing target",
			&core.HandlerConfig{Sync: syncSvc},
			http.MethodPost,
			"/api/sync",
			`{}`,
			http.StatusPreconditionFailed,
			"workspace_context_missing",
		},
		{
			"exec missing prompt",
			&core.HandlerConfig{Exec: execSvc},
			http.MethodPost,
			"/api/exec",
			`{"workspace_path":"/tmp/workspace"}`,
			http.StatusUnprocessableEntity,
			"prompt_required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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
