package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
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
	}{
		{
			name:   "ShouldReturnNotFoundWhenCreateFails",
			method: http.MethodPost,
			path:   "/api/sessions",
			body:   []byte(`{"agent_name":"coder","workspace":"alpha"}`),
		},
		{
			name:   "ShouldReturnNotFoundWhenSessionLookupFails",
			method: http.MethodGet,
			path:   "/api/workspaces/ws-workspace/sessions/missing",
		},
		{
			name:   "ShouldReturnNotFoundWhenAttachFails",
			method: http.MethodPost,
			path:   "/api/workspaces/ws-workspace/sessions/missing/attach",
		},
		{
			name:   "ShouldReturnNotFoundWhenDeleteFails",
			method: http.MethodDelete,
			path:   "/api/workspaces/ws-workspace/sessions/missing",
		},
		{
			name:   "ShouldReturnNotFoundWhenStopFails",
			method: http.MethodPost,
			path:   "/api/workspaces/ws-workspace/sessions/missing/stop",
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

func TestDeleteWorkspaceHandlerReturnsConflictWhenWorkspaceHasSessions(t *testing.T) {
	homePaths := newTestHomePaths(t)
	stopped := newSessionInfo("sess-stopped")
	stopped.WorkspaceID = "ws_alpha"
	stopped.State = session.StateStopped
	var deleted []string
	manager := stubSessionManager{
		ListAllFn: func(context.Context) ([]*session.Info, error) {
			return []*session.Info{stopped}, nil
		},
		DeleteFn: func(_ context.Context, id string) error {
			deleted = append(deleted, id)
			return nil
		},
	}
	workspaces := stubWorkspaceService{
		GetFn: func(context.Context, string) (workspacepkg.Workspace, error) {
			return workspacepkg.Workspace{ID: "ws_alpha", Name: "alpha"}, nil
		},
		UnregisterFn: func(context.Context, string) error {
			return workspacepkg.ErrWorkspaceHasSessions
		},
	}
	engine := newTestRouter(
		t,
		newTestHandlersWithWorkspace(t, manager, stubObserver{}, workspaces, homePaths),
	)

	resp := performRequest(t, engine, http.MethodDelete, "/api/workspaces/ws_alpha", nil)
	if resp.Code != http.StatusConflict {
		t.Fatalf(
			"delete workspace status = %d, want %d; body=%s",
			resp.Code,
			http.StatusConflict,
			resp.Body.String(),
		)
	}
	var payload contract.ErrorPayload
	decodeJSONResponse(t, resp, &payload)
	if payload.Error != workspacepkg.ErrWorkspaceHasSessions.Error() {
		t.Fatalf("error = %q, want %q", payload.Error, workspacepkg.ErrWorkspaceHasSessions.Error())
	}
	if len(deleted) != 0 {
		t.Fatalf("Delete() calls = %#v, want none before failed unregister", deleted)
	}
}

func TestDeleteWorkspaceHandlerReturnsConflictWhenWorkspaceHasActiveSession(t *testing.T) {
	t.Parallel()

	t.Run("Should map the resolver active-session conflict", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		unregisterCalled := false
		workspaces := stubWorkspaceService{
			GetFn: func(context.Context, string) (workspacepkg.Workspace, error) {
				return workspacepkg.Workspace{ID: "ws_alpha", Name: "alpha"}, nil
			},
			UnregisterFn: func(context.Context, string) error {
				unregisterCalled = true
				return workspacepkg.ErrWorkspaceHasActiveSessions
			},
		}
		engine := newTestRouter(
			t,
			newTestHandlersWithWorkspace(t, stubSessionManager{}, stubObserver{}, workspaces, homePaths),
		)

		resp := performRequest(t, engine, http.MethodDelete, "/api/workspaces/ws_alpha", nil)
		if resp.Code != http.StatusConflict {
			t.Fatalf(
				"delete workspace status = %d, want %d; body=%s",
				resp.Code,
				http.StatusConflict,
				resp.Body.String(),
			)
		}
		var payload contract.ErrorPayload
		decodeJSONResponse(t, resp, &payload)
		if payload.Error != workspacepkg.ErrWorkspaceHasActiveSessions.Error() {
			t.Fatalf("error = %q, want %q", payload.Error, workspacepkg.ErrWorkspaceHasActiveSessions.Error())
		}
		if !unregisterCalled {
			t.Fatal("Unregister() was not called")
		}
	})
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

func TestHandlersRejectBadPromptAndQueryValues(t *testing.T) {
	homePaths := newTestHomePaths(t)
	manager := stubSessionManager{
		StatusFn: func(context.Context, string) (*session.Info, error) {
			return newSessionInfo("sess-123"), nil
		},
	}
	engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, homePaths))

	badPrompt := performRequest(
		t,
		engine,
		http.MethodPost,
		"/api/workspaces/ws-workspace/sessions/sess-123/prompt",
		[]byte(`{"message":""}`),
	)
	if badPrompt.Code != http.StatusBadRequest {
		t.Fatalf("bad prompt status = %d, want %d", badPrompt.Code, http.StatusBadRequest)
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

	streamResp := performRequestWithHeaders(
		t,
		engine,
		http.MethodGet,
		"/api/workspaces/ws-workspace/sessions/sess-123/stream",
		nil,
		map[string]string{"Last-Event-ID": "bad"},
	)
	if streamResp.Code != http.StatusBadRequest {
		t.Fatalf("stream bad header status = %d, want %d", streamResp.Code, http.StatusBadRequest)
	}
}

func TestPromptSessionHandlerCoversThoughtPermissionAndErrorBranches(t *testing.T) {
	homePaths := newTestHomePaths(t)
	manager := stubSessionManager{
		SendPromptFn: func(context.Context, string, session.SendPromptOpts) (session.SendPromptResult, error) {
			ch := make(chan acp.AgentEvent, 3)
			ch <- acp.AgentEvent{
				Type:      "thought",
				TurnID:    "turn-err",
				Timestamp: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
				Text:      "thinking",
			}
			ch <- acp.AgentEvent{
				Type:      "permission",
				TurnID:    "turn-err",
				Timestamp: time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC),
				Action:    "fs/read_text_file",
				Decision:  "allow",
			}
			ch <- acp.AgentEvent{
				Type:      "error",
				TurnID:    "turn-err",
				Timestamp: time.Date(2026, 4, 3, 12, 0, 2, 0, time.UTC),
				Error:     "boom",
			}
			close(ch)
			return session.SendPromptResult{Status: "accepted", Events: ch, NewTurnID: "turn-err"}, nil
		},
	}
	engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, homePaths))

	resp := performRequest(
		t,
		engine,
		http.MethodPost,
		"/api/workspaces/ws-workspace/sessions/sess-123/prompt",
		[]byte(`{"message":"hello"}`),
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}

	records := parseSSE(t, resp.Body.String())
	if len(records) < 5 {
		t.Fatalf("records = %d, want at least 5; body=%s", len(records), resp.Body.String())
	}
	if string(records[len(records)-1].Data) != "[DONE]" {
		t.Fatalf("last record data = %q, want [DONE]", string(records[len(records)-1].Data))
	}

	var partTypes []string
	for _, record := range records[:len(records)-1] {
		if len(record.Data) == 0 {
			continue
		}
		var part map[string]any
		if err := json.Unmarshal(record.Data, &part); err != nil {
			t.Fatalf("json.Unmarshal(part) error = %v; data=%s", err, string(record.Data))
		}
		if value, ok := part["type"].(string); ok {
			partTypes = append(partTypes, value)
		}
	}
	if !contains(partTypes, "reasoning-start") || !contains(partTypes, "reasoning-delta") ||
		!contains(partTypes, "reasoning-end") ||
		!contains(partTypes, "data-agh-permission") ||
		!contains(partTypes, "error") ||
		!contains(partTypes, "finish") {
		t.Fatalf("part types = %#v", partTypes)
	}
}

func TestAgentObserveHealthAndDaemonStatusErrorPaths(t *testing.T) {
	homePaths := newTestHomePaths(t)
	handlers := newTestHandlers(t, stubSessionManager{}, stubObserver{
		QueryEventsFn: func(context.Context, store.EventSummaryQuery) ([]store.EventSummary, error) {
			return nil, errors.New("boom")
		},
		HealthFn: func(context.Context) (observe.Health, error) {
			return observe.Health{}, errors.New("health failed")
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

func TestCORSMiddlewareRejectsDisallowedOrigins(t *testing.T) {
	homePaths := newTestHomePaths(t)
	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths))

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"http://127.0.0.1/api/sessions",
		http.NoBody,
	)
	req.Header.Set("Origin", "https://evil.example")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}

func TestCORSMiddlewareRejectsDifferentLoopbackOrigins(t *testing.T) {
	homePaths := newTestHomePaths(t)
	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{
		ListAllFn: func(context.Context) ([]*session.Info, error) {
			return nil, nil
		},
	}, stubObserver{}, homePaths))

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"http://127.0.0.1/api/sessions",
		http.NoBody,
	)
	req.Header.Set("Origin", "http://localhost:3000")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}

func TestRequestBodyLimitRejectsOversizedAPIRequests(t *testing.T) {
	homePaths := newTestHomePaths(t)
	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths))

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://127.0.0.1/api/sessions",
		strings.NewReader(strings.Repeat("x", int(maxAPIRequestBodyBytes)+1)),
	)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf(
			"status = %d, want %d; body=%s",
			recorder.Code,
			http.StatusRequestEntityTooLarge,
			recorder.Body.String(),
		)
	}

	var payload contract.ErrorPayload
	decodeJSONResponse(t, recorder, &payload)
	if payload.Error != errRequestBodyTooLarge.Error() {
		t.Fatalf("error = %q, want %q", payload.Error, errRequestBodyTooLarge.Error())
	}
}

func TestResolveAllowedOriginRejectsSameHostDifferentPort(t *testing.T) {
	t.Parallel()

	allowedOrigin, ok := resolveAllowedOrigin("http://example.com:3000", "http", "example.com:2123", "example.com")
	if ok {
		t.Fatalf("resolveAllowedOrigin() = %q, true, want rejection for same-host different-port origin", allowedOrigin)
	}
}

func TestRespondErrorSanitizesInternalFailures(t *testing.T) {
	homePaths := newTestHomePaths(t)
	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{
		ListAllFn: func(context.Context) ([]*session.Info, error) {
			return nil, errors.New("secret internal path")
		},
	}, stubObserver{}, homePaths))

	recorder := performRequest(t, engine, http.MethodGet, "/api/sessions", nil)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}

	var payload contract.ErrorPayload
	decodeJSONResponse(t, recorder, &payload)
	if payload.Error != http.StatusText(http.StatusInternalServerError) {
		t.Fatalf("error payload = %q, want %q", payload.Error, http.StatusText(http.StatusInternalServerError))
	}
}

func TestObserveStreamBadHeaderAndMissingAgentsDir(t *testing.T) {
	homePaths := newTestHomePaths(t)
	if err := os.RemoveAll(homePaths.AgentsDir); err != nil {
		t.Fatalf("os.RemoveAll(AgentsDir) error = %v", err)
	}
	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths))

	agentsResp := performRequest(t, engine, http.MethodGet, "/api/agents", nil)
	if agentsResp.Code != http.StatusOK {
		t.Fatalf("agents status = %d, want %d", agentsResp.Code, http.StatusOK)
	}

	observeResp := performRequestWithHeaders(
		t,
		engine,
		http.MethodGet,
		"/api/logs/stream?workspace_id=ws-workspace",
		nil,
		map[string]string{"Last-Event-ID": "bad"},
	)
	if observeResp.Code != http.StatusBadRequest {
		t.Fatalf("observe stream status = %d, want %d", observeResp.Code, http.StatusBadRequest)
	}
}
