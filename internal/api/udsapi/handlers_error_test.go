package udsapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/observe"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestCreateGetResumeDeleteAndStopHandlersReturnExpectedErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		method string
		path   string
		body   []byte
		error  string
	}{
		{
			name:   "ShouldReturnNotFoundWhenCreateFails",
			method: http.MethodPost,
			path:   "/api/sessions",
			body:   []byte(`{"agent_name":"coder","workspace":"alpha"}`),
			error:  "file does not exist",
		},
		{
			name:   "ShouldReturnNotFoundWhenSessionLookupFails",
			method: http.MethodGet,
			path:   "/api/workspaces/ws-workspace/sessions/missing",
			error:  "session not found",
		},
		{
			name:   "ShouldReturnNotFoundWhenAttachFails",
			method: http.MethodPost,
			path:   "/api/workspaces/ws-workspace/sessions/missing/attach",
			error:  "session not found",
		},
		{
			name:   "ShouldReturnNotFoundWhenDeleteFails",
			method: http.MethodDelete,
			path:   "/api/workspaces/ws-workspace/sessions/missing",
			error:  "session not found",
		},
		{
			name:   "ShouldReturnNotFoundWhenStopFails",
			method: http.MethodPost,
			path:   "/api/workspaces/ws-workspace/sessions/missing/stop",
			error:  "session not found",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			homePaths := newTestHomePaths(t)
			manager := stubSessionManager{
				CreateFn: func(context.Context, session.CreateOpts) (*session.Session, error) {
					return nil, os.ErrNotExist
				},
				StatusFn: func(context.Context, string) (*session.Info, error) {
					return nil, session.ErrSessionNotFound
				},
				ResumeFn: func(context.Context, string) (*session.Session, error) {
					return nil, session.ErrSessionNotFound
				},
				DeleteFn: func(context.Context, string) error {
					return session.ErrSessionNotFound
				},
				StopFn: func(context.Context, string) error {
					return session.ErrSessionNotFound
				},
			}
			engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, homePaths))

			resp := performRequest(t, engine, tt.method, tt.path, tt.body)
			if resp.Code != http.StatusNotFound {
				t.Fatalf(
					"%s %s status = %d, want %d; body=%s",
					tt.method,
					tt.path,
					resp.Code,
					http.StatusNotFound,
					resp.Body.String(),
				)
			}
			var payload contract.ErrorPayload
			decodeJSONResponse(t, resp, &payload)
			if !strings.Contains(payload.Error, tt.error) {
				t.Fatalf("error = %q, want substring %q", payload.Error, tt.error)
			}
		})
	}
}

func TestCreateSessionHandlerRejectsInvalidWorkspaceContract(t *testing.T) {
	homePaths := newTestHomePaths(t)
	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths))

	tests := []struct {
		name string
		body string
	}{
		{
			name: "missing workspace reference",
			body: `{"agent_name":"coder"}`,
		},
		{
			name: "mutually exclusive workspace fields",
			body: `{"agent_name":"coder","workspace":"alpha","workspace_path":"/workspace"}`,
		},
		{
			name: "relative workspace path",
			body: `{"agent_name":"coder","workspace_path":"workspace"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := performRequest(t, engine, http.MethodPost, "/api/sessions", []byte(tt.body))
			if resp.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusBadRequest, resp.Body.String())
			}
		})
	}
}

func TestWorkspaceHandlersReturnExpectedErrors(t *testing.T) {
	homePaths := newTestHomePaths(t)
	workspaces := stubWorkspaceService{
		RegisterFn: func(context.Context, workspacepkg.RegisterOptions) (workspacepkg.Workspace, error) {
			return workspacepkg.Workspace{}, workspacepkg.ErrWorkspacePathTaken
		},
		GetFn: func(context.Context, string) (workspacepkg.Workspace, error) {
			return workspacepkg.Workspace{}, workspacepkg.ErrWorkspaceNotFound
		},
		ResolveFn: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
			return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceRootMissing
		},
		ResolveOrRegisterFn: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
			return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceRootMissing
		},
	}
	engine := newTestRouter(
		t,
		newTestHandlersWithWorkspace(t, stubSessionManager{}, stubObserver{}, workspaces, homePaths),
	)

	createResp := performRequest(t, engine, http.MethodPost, "/api/workspaces", []byte(`{"root_dir":"/workspace"}`))
	if createResp.Code != http.StatusConflict {
		t.Fatalf("create workspace status = %d, want %d", createResp.Code, http.StatusConflict)
	}

	getResp := performRequest(t, engine, http.MethodGet, "/api/workspaces/ws-missing", nil)
	if getResp.Code != http.StatusGone {
		t.Fatalf("get workspace status = %d, want %d", getResp.Code, http.StatusGone)
	}

	deleteResp := performRequest(t, engine, http.MethodDelete, "/api/workspaces/ws-missing", nil)
	if deleteResp.Code != http.StatusNotFound {
		t.Fatalf("delete workspace status = %d, want %d", deleteResp.Code, http.StatusNotFound)
	}

	resolveResp := performRequest(
		t,
		engine,
		http.MethodPost,
		"/api/workspaces/resolve",
		[]byte(`{"path":"/workspace"}`),
	)
	if resolveResp.Code != http.StatusGone {
		t.Fatalf("resolve workspace status = %d, want %d", resolveResp.Code, http.StatusGone)
	}
}

func TestCreateSessionHandlerMapsWorkspaceErrors(t *testing.T) {
	homePaths := newTestHomePaths(t)
	manager := stubSessionManager{
		CreateFn: func(context.Context, session.CreateOpts) (*session.Session, error) {
			return nil, fmt.Errorf("session: resolve workspace %q: %w", "alpha", workspacepkg.ErrWorkspaceRootMissing)
		},
	}
	engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, homePaths))

	resp := performRequest(
		t,
		engine,
		http.MethodPost,
		"/api/sessions",
		[]byte(`{"agent_name":"coder","workspace":"alpha"}`),
	)
	if resp.Code != http.StatusGone {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusGone, resp.Body.String())
	}
}

func TestListAndSessionHandlersRejectBadQueryAndHeaderValues(t *testing.T) {
	homePaths := newTestHomePaths(t)
	listEngine := newTestRouter(t, newTestHandlers(t, stubSessionManager{
		ListAllFn: func(context.Context) ([]*session.Info, error) {
			return nil, errors.New("list failed")
		},
	}, stubObserver{}, homePaths))

	listResp := performRequest(t, listEngine, http.MethodGet, "/api/sessions", nil)
	if listResp.Code != http.StatusInternalServerError {
		t.Fatalf("list status = %d, want %d", listResp.Code, http.StatusInternalServerError)
	}

	manager := stubSessionManager{
		ListAllFn: func(context.Context) ([]*session.Info, error) {
			return []*session.Info{newSessionInfo("sess-123")}, nil
		},
		StatusFn: func(context.Context, string) (*session.Info, error) {
			return newSessionInfo("sess-123"), nil
		},
	}
	engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, homePaths))

	filterResp := performRequest(t, engine, http.MethodGet, "/api/sessions?workspace=missing", nil)
	if filterResp.Code != http.StatusNotFound {
		t.Fatalf("filtered list status = %d, want %d", filterResp.Code, http.StatusNotFound)
	}

	eventsResp := performRequest(
		t,
		engine,
		http.MethodGet,
		"/api/workspaces/ws-workspace/sessions/sess-123/events?since=bad",
		nil,
	)
	if eventsResp.Code != http.StatusBadRequest {
		t.Fatalf("events bad query status = %d, want %d", eventsResp.Code, http.StatusBadRequest)
	}

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/api/workspaces/ws-workspace/sessions/sess-123/stream",
		http.NoBody,
	)
	req.Header.Set("Last-Event-ID", "bad")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("stream bad header status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestGetAgentAndObserveHandlersReturnErrors(t *testing.T) {
	homePaths := newTestHomePaths(t)
	handlers := newTestHandlers(t, stubSessionManager{}, stubObserver{
		QueryEventsFn: func(context.Context, store.EventSummaryQuery) ([]store.EventSummary, error) {
			return nil, errors.New("boom")
		},
	}, homePaths)
	handlers.AgentLoader = func(_ string, _ aghconfig.HomePaths) (aghconfig.AgentDef, error) {
		return aghconfig.AgentDef{}, os.ErrNotExist
	}
	engine := newTestRouter(t, handlers)

	agentResp := performRequest(t, engine, http.MethodGet, "/api/agents/missing", nil)
	if agentResp.Code != http.StatusNotFound {
		t.Fatalf("agent status = %d, want %d", agentResp.Code, http.StatusNotFound)
	}

	observeResp := performRequest(t, engine, http.MethodGet, "/api/logs?workspace_id=ws-workspace", nil)
	if observeResp.Code != http.StatusInternalServerError {
		t.Fatalf("observe status = %d, want %d", observeResp.Code, http.StatusInternalServerError)
	}
}

func TestListAgentsHandlesMissingDirectory(t *testing.T) {
	homePaths := newTestHomePaths(t)
	if err := os.RemoveAll(homePaths.AgentsDir); err != nil {
		t.Fatalf("os.RemoveAll(AgentsDir) error = %v", err)
	}
	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths))

	recorder := performRequest(t, engine, http.MethodGet, "/api/agents", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestObserveStreamAndHealthAndDaemonStatusErrorPaths(t *testing.T) {
	homePaths := newTestHomePaths(t)
	observer := stubObserver{
		QueryEventsFn: func(context.Context, store.EventSummaryQuery) ([]store.EventSummary, error) {
			return nil, errors.New("query failed")
		},
		HealthFn: func(context.Context) (observe.Health, error) {
			return observe.Health{}, errors.New("health failed")
		},
	}
	handlers := newTestHandlers(t, stubSessionManager{}, observer, homePaths)
	engine := newTestRouter(t, handlers)

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/api/logs/stream?workspace_id=ws-workspace",
		http.NoBody,
	)
	req.Header.Set("Last-Event-ID", "bad")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("observe stream bad header status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	healthResp := performRequest(t, engine, http.MethodGet, "/api/status", nil)
	if healthResp.Code != http.StatusInternalServerError {
		t.Fatalf("health status = %d, want %d", healthResp.Code, http.StatusInternalServerError)
	}

	statusHandlers := newTestHandlers(t, stubSessionManager{
		ListAllFn: func(context.Context) ([]*session.Info, error) {
			return nil, errors.New("list failed")
		},
	}, stubObserver{
		HealthFn: func(context.Context) (observe.Health, error) {
			return observe.Health{Status: "ok"}, nil
		},
	}, homePaths)
	statusEngine := newTestRouter(t, statusHandlers)
	statusResp := performRequest(t, statusEngine, http.MethodGet, "/api/status", nil)
	if statusResp.Code != http.StatusInternalServerError {
		t.Fatalf("daemon status = %d, want %d", statusResp.Code, http.StatusInternalServerError)
	}
}
