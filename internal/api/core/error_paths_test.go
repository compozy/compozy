package core_test

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/testutil"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/observe"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/transcript"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestBaseHandlersRejectInvalidRequestsAndMapErrors(t *testing.T) {
	t.Parallel()

	manager := testutil.StubSessionManager{
		CreateFn: func(context.Context, session.CreateOpts) (*session.Session, error) {
			return nil, os.ErrNotExist
		},
		StatusFn: func(context.Context, string) (*session.Info, error) {
			return nil, session.ErrSessionNotFound
		},
		AttachSessionFn: func(context.Context, store.SessionAttachRequest) (store.SessionAttach, error) {
			return store.SessionAttach{}, store.ErrSessionNotFound
		},
		DeleteFn: func(context.Context, string) error {
			return session.ErrSessionNotFound
		},
		StopFn: func(context.Context, string) error {
			return session.ErrSessionNotFound
		},
		ListAllFn: func(context.Context) ([]*session.Info, error) {
			return nil, errors.New("list failed")
		},
	}
	observer := testutil.StubObserver{
		QueryEventsFn: func(context.Context, store.EventSummaryQuery) ([]store.EventSummary, error) {
			return nil, errors.New("boom")
		},
		HealthFn: func(context.Context) (observe.Health, error) {
			return observe.Health{}, errors.New("health failed")
		},
	}
	workspaces := testutil.StubWorkspaceService{
		RegisterFn: func(context.Context, workspacepkg.RegisterOptions) (workspacepkg.Workspace, error) {
			return workspacepkg.Workspace{}, workspacepkg.ErrWorkspacePathTaken
		},
		GetFn: func(context.Context, string) (workspacepkg.Workspace, error) {
			return workspacepkg.Workspace{}, workspacepkg.ErrWorkspaceNotFound
		},
		ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
			if ref == "ws-workspace" {
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{
						ID:      ref,
						RootDir: "/workspace",
						Name:    "Workspace",
					},
					WorkspaceID: ref,
				}, nil
			}
			return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceRootMissing
		},
		ResolveOrRegisterFn: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
			return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceRootMissing
		},
	}

	fixture := newHandlerFixture(t, manager, observer, workspaces, nil, nil)

	requests := []struct {
		name             string
		method           string
		path             string
		body             []byte
		want             int
		wantBodyContains string
	}{
		{
			name:   "Should reject session create requests without workspace context",
			method: http.MethodPost,
			path:   "/sessions",
			body:   []byte(`{"agent_name":"coder"}`),
			want:   http.StatusBadRequest,
		},
		{
			name:   "Should return not found when session creation fallback resolves to a missing session",
			method: http.MethodPost,
			path:   "/sessions",
			body:   []byte(`{"agent_name":"coder","workspace":"alpha"}`),
			want:   http.StatusNotFound,
		},
		{
			name:             "Should return not found when reading a missing session",
			method:           http.MethodGet,
			path:             "/workspaces/ws-workspace/sessions/missing",
			want:             http.StatusNotFound,
			wantBodyContains: "session not found",
		},
		{
			name:             "Should return not found when attaching to a missing session",
			method:           http.MethodPost,
			path:             "/workspaces/ws-workspace/sessions/missing/attach",
			want:             http.StatusNotFound,
			wantBodyContains: "session not found",
		},
		{
			name:             "Should return not found when deleting a missing session",
			method:           http.MethodDelete,
			path:             "/workspaces/ws-workspace/sessions/missing",
			want:             http.StatusNotFound,
			wantBodyContains: "session not found",
		},
		{
			name:             "Should return bad request for invalid session event since filters",
			method:           http.MethodGet,
			path:             "/workspaces/ws-workspace/sessions/missing/events?since=bad",
			want:             http.StatusBadRequest,
			wantBodyContains: "invalid",
		},
		{
			name:             "Should return internal server error when log queries fail",
			method:           http.MethodGet,
			path:             "/logs?workspace_id=ws-workspace",
			want:             http.StatusInternalServerError,
			wantBodyContains: "boom",
		},
		{
			name:   "Should return internal server error when health queries fail",
			method: http.MethodGet,
			path:   "/status",
			want:   http.StatusInternalServerError,
		},
		{
			name:   "Should return ok for doctor endpoint without dependencies",
			method: http.MethodGet,
			path:   "/doctor",
			want:   http.StatusOK,
		},
		{
			name:   "Should reject workspace registration with relative root paths",
			method: http.MethodPost,
			path:   "/workspaces",
			body:   []byte(`{"root_dir":"relative"}`),
			want:   http.StatusBadRequest,
		},
		{
			name:   "Should map missing workspace roots to gone",
			method: http.MethodGet,
			path:   "/workspaces/ws-missing",
			want:   http.StatusGone,
		},
		{
			name:   "Should map resolve failures for missing workspace roots to gone",
			method: http.MethodPost,
			path:   "/workspaces/resolve",
			body:   []byte(`{"path":"/workspace"}`),
			want:   http.StatusGone,
		},
	}

	for _, request := range requests {
		t.Run(request.name, func(t *testing.T) {
			t.Parallel()

			resp := performRequest(t, fixture.Engine, request.method, request.path, request.body)
			if request.wantBodyContains != "" {
				assertAPIErrorResponse(t, resp, request.want, request.wantBodyContains)
				return
			}
			if resp.Code != request.want {
				t.Fatalf("%s status = %d, want %d; body=%s", request.name, resp.Code, request.want, resp.Body.String())
			}
		})
	}
}

func TestSessionHistoryEventsAndTranscriptErrorBranches(t *testing.T) {
	t.Parallel()

	manager := testutil.StubSessionManager{
		StatusFn: func(context.Context, string) (*session.Info, error) {
			return testutil.NewSessionInfo("sess-a"), nil
		},
		EventsFn: func(context.Context, string, store.EventQuery) ([]store.SessionEvent, error) {
			return nil, session.ErrSessionNotFound
		},
		HistoryFn: func(context.Context, string, store.EventQuery) ([]store.TurnHistory, error) {
			return nil, session.ErrSessionNotFound
		},
		TranscriptPageFn: func(context.Context, string, transcript.PageQuery) (transcript.Page, error) {
			return transcript.Page{}, session.ErrSessionNotFound
		},
	}
	fixture := newHandlerFixture(
		t,
		manager,
		testutil.StubObserver{},
		testutil.StubWorkspaceService{},
		nil,
		nil,
	)

	for _, path := range []string{
		"/workspaces/ws-workspace/sessions/sess-a/events",
		"/workspaces/ws-workspace/sessions/sess-a/history",
		"/workspaces/ws-workspace/sessions/sess-a/transcript",
	} {
		resp := performRequest(t, fixture.Engine, http.MethodGet, path, nil)
		if resp.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want %d", path, resp.Code, http.StatusNotFound)
		}
	}
}

func TestStreamSessionAndObserveErrorBranches(t *testing.T) {
	t.Parallel()

	manager := testutil.StubSessionManager{
		StatusFn: func(context.Context, string) (*session.Info, error) {
			info := testutil.NewSessionInfo("sess-a")
			info.State = session.StateStopped
			info.UpdatedAt = time.Date(2026, 4, 3, 12, 0, 2, 0, time.UTC)
			return info, nil
		},
		EventsFn: func(context.Context, string, store.EventQuery) ([]store.SessionEvent, error) {
			return nil, nil
		},
	}
	fixture := newHandlerFixture(
		t,
		manager,
		testutil.StubObserver{},
		testutil.StubWorkspaceService{},
		nil,
		nil,
	)

	badStream := performRequest(
		t,
		fixture.Engine,
		http.MethodGet,
		"/workspaces/ws-workspace/sessions/sess-a/stream",
		nil,
	)
	if badStream.Code != http.StatusOK {
		t.Fatalf("stream stopped status = %d, want %d", badStream.Code, http.StatusOK)
	}

	badHeader := testutil.PerformRequestWithHeaders(
		t,
		fixture.Engine,
		http.MethodGet,
		"/workspaces/ws-workspace/sessions/sess-a/stream",
		nil,
		map[string]string{"Last-Event-ID": "bad"},
	)
	if badHeader.Code != http.StatusBadRequest {
		t.Fatalf("stream bad header status = %d, want %d", badHeader.Code, http.StatusBadRequest)
	}

	observeFixture := newHandlerFixture(
		t,
		testutil.StubSessionManager{},
		testutil.StubObserver{},
		testutil.StubWorkspaceService{},
		nil,
		nil,
	)
	observeBadHeader := testutil.PerformRequestWithHeaders(
		t,
		observeFixture.Engine,
		http.MethodGet,
		"/logs/stream?workspace_id=ws-workspace",
		nil,
		map[string]string{"Last-Event-ID": "bad"},
	)
	if observeBadHeader.Code != http.StatusBadRequest {
		t.Fatalf(
			"observe bad header status = %d, want %d",
			observeBadHeader.Code,
			http.StatusBadRequest,
		)
	}
}

func TestStreamSessionInitialEventsErrorWithoutLiveSubscription(t *testing.T) {
	t.Parallel()

	t.Run("Should return an error response when live subscription is unavailable", func(t *testing.T) {
		t.Parallel()

		initialErr := errors.New("events unavailable")
		manager := testutil.StubSessionManager{
			StatusFn: func(_ context.Context, id string) (*session.Info, error) {
				return testutil.NewSessionInfo(id), nil
			},
			EventsFn: func(context.Context, string, store.EventQuery) ([]store.SessionEvent, error) {
				return nil, initialErr
			},
		}
		fixture := newHandlerFixture(
			t,
			manager,
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/sessions/sess-a/stream?frames=raw",
			nil,
		)
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("stream initial error status = %d, want %d", resp.Code, http.StatusInternalServerError)
		}
		if !strings.Contains(resp.Body.String(), initialErr.Error()) {
			t.Fatalf("stream initial error body = %s, want %q", resp.Body.String(), initialErr.Error())
		}
	})
}

func TestListAgentsHandlesMissingDirectory(t *testing.T) {
	t.Parallel()

	fixture := newHandlerFixture(
		t,
		testutil.StubSessionManager{},
		testutil.StubObserver{},
		testutil.StubWorkspaceService{},
		nil,
		nil,
	)
	if err := os.RemoveAll(fixture.HomePaths.AgentsDir); err != nil {
		t.Fatalf("RemoveAll(AgentsDir) error = %v", err)
	}

	resp := performRequest(t, fixture.Engine, http.MethodGet, "/agents", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("list agents missing dir status = %d, want %d", resp.Code, http.StatusOK)
	}
}

func TestListAgentsWorkspaceResolverUnavailable(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should return service unavailable when workspace resolver is missing",
		func(t *testing.T) {
			t.Parallel()

			fixture := newHandlerFixture(
				t,
				testutil.StubSessionManager{},
				testutil.StubObserver{},
				testutil.StubWorkspaceService{},
				nil,
				nil,
			)
			fixture.Handlers.Workspaces = nil

			resp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/agents?workspace=alpha",
				nil,
			)
			if resp.Code != http.StatusServiceUnavailable {
				t.Fatalf(
					"workspace agents status = %d, want %d; body=%s",
					resp.Code,
					http.StatusServiceUnavailable,
					resp.Body.String(),
				)
			}
			var payload contract.ErrorPayload
			testutil.DecodeJSONResponse(t, resp, &payload)
			if !strings.Contains(payload.Error, "workspace resolver unavailable") {
				t.Fatalf("workspace agents error = %#v, want resolver unavailable detail", payload)
			}
		},
	)
}

func TestListAgentsSkipsUnreadableDefinitions(t *testing.T) {
	t.Parallel()

	fixture := newHandlerFixture(
		t,
		testutil.StubSessionManager{},
		testutil.StubObserver{},
		testutil.StubWorkspaceService{},
		nil,
		nil,
	)
	testutil.WriteAgentDef(t, fixture.HomePaths, "coder")
	testutil.WriteAgentDef(t, fixture.HomePaths, "broken")
	fixture.Handlers.AgentLoader = func(name string, homePaths aghconfig.HomePaths) (aghconfig.AgentDef, error) {
		if name == "broken" {
			return aghconfig.AgentDef{}, errors.New("bad agent")
		}
		return aghconfig.LoadAgentDef(name, homePaths)
	}

	resp := performRequest(t, fixture.Engine, http.MethodGet, "/agents", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("list agents skip unreadable status = %d, want %d", resp.Code, http.StatusOK)
	}
}

func TestMemoryHelpersAndMissingStoreBranches(t *testing.T) {
	t.Parallel()

	store := memory.NewStore(filepath.Join(t.TempDir(), "memory"))
	if err := store.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs() error = %v", err)
	}
	workspace := t.TempDir()
	globalDoc := []byte(memoryDocument(t, "Shared", memcontract.TypeUser, "global"))
	workspaceDoc := []byte(memoryDocument(t, "Shared", memcontract.TypeProject, "workspace"))
	if err := store.Write(memcontract.ScopeGlobal, "shared.md", globalDoc); err != nil {
		t.Fatalf("Write(global) error = %v", err)
	}
	if err := store.ForWorkspace(workspace).Write(memcontract.ScopeWorkspace, "shared.md", workspaceDoc); err != nil {
		t.Fatalf("Write(workspace) error = %v", err)
	}
	if err := store.ForWorkspace(workspace).
		Write(memcontract.ScopeWorkspace, "workspace-only.md", workspaceDoc); err != nil {
		t.Fatalf("Write(workspace-only) error = %v", err)
	}

	fixture := newHandlerFixture(
		t,
		testutil.StubSessionManager{},
		testutil.StubObserver{},
		testutil.StubWorkspaceService{},
		store,
		nil,
	)
	if _, err := fixture.Handlers.ResolveMemoryLocation("workspace-only.md", "", workspace); err != nil {
		t.Fatalf("ResolveMemoryLocation(workspace-only) error = %v", err)
	}
	if _, err := fixture.Handlers.ResolveMemoryLocation(
		"shared.md",
		"",
		workspace,
	); !errors.Is(
		err,
		memory.ErrValidation,
	) {
		t.Fatalf("ResolveMemoryLocation(shared) error = %v, want validation", err)
	}
	if _, _, err := core.ResolveMemoryWriteScope(contract.MemoryWriteRequest{}); !errors.Is(
		err,
		memory.ErrValidation,
	) {
		t.Fatalf("ResolveMemoryWriteScope(empty) error = %v, want validation", err)
	}

	noStoreFixture := newHandlerFixture(
		t,
		testutil.StubSessionManager{},
		testutil.StubObserver{},
		testutil.StubWorkspaceService{},
		nil,
		nil,
	)
	requests := []struct {
		method string
		path   string
		body   []byte
	}{
		{method: http.MethodGet, path: "/memory"},
		{method: http.MethodGet, path: "/memory/valid.md?scope=global"},
		{
			method: http.MethodPost,
			path:   "/memory",
			body:   []byte(`{"scope":"global","type":"user","name":"Valid","content":"hello"}`),
		},
		{method: http.MethodDelete, path: "/memory/valid.md?scope=global"},
	}
	for _, request := range requests {
		t.Run(request.method+" "+request.path, func(t *testing.T) {
			resp := performRequest(
				t,
				noStoreFixture.Engine,
				request.method,
				request.path,
				request.body,
			)
			if resp.Code != http.StatusInternalServerError {
				t.Fatalf(
					"%s %s status = %d, want %d",
					request.method,
					request.path,
					resp.Code,
					http.StatusInternalServerError,
				)
			}
		})
	}
}

func TestWorkspaceUpdateValidationAndDeleteErrors(t *testing.T) {
	t.Parallel()

	newFixture := func(t *testing.T, workspaces testutil.StubWorkspaceService) handlerFixture {
		t.Helper()
		return newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			workspaces,
			nil,
			nil,
		)
	}

	t.Run("Should reject empty workspace names", func(t *testing.T) {
		t.Parallel()

		workspace := workspacepkg.Workspace{ID: "ws_alpha", RootDir: t.TempDir(), Name: "alpha"}
		workspaces := testutil.StubWorkspaceService{
			GetFn: func(context.Context, string) (workspacepkg.Workspace, error) {
				return workspace, nil
			},
			UpdateFn: func(context.Context, string, workspacepkg.UpdateOptions) error {
				return nil
			},
		}
		fixture := newFixture(t, workspaces)

		badUpdate := performRequest(
			t,
			fixture.Engine,
			http.MethodPatch,
			"/workspaces/ws_alpha",
			[]byte(`{"name":""}`),
		)
		if badUpdate.Code != http.StatusBadRequest {
			t.Fatalf("bad update status = %d, want %d", badUpdate.Code, http.StatusBadRequest)
		}
		if !strings.Contains(badUpdate.Body.String(), "name is required") {
			t.Fatalf("bad update body = %s, want name validation message", badUpdate.Body.String())
		}
	})

	t.Run("Should map workspace delete conflicts", func(t *testing.T) {
		t.Parallel()

		workspace := workspacepkg.Workspace{ID: "ws_alpha", RootDir: t.TempDir(), Name: "alpha"}
		workspaces := testutil.StubWorkspaceService{
			GetFn: func(context.Context, string) (workspacepkg.Workspace, error) {
				return workspace, nil
			},
			UnregisterFn: func(context.Context, string) error {
				return workspacepkg.ErrWorkspaceHasSessions
			},
		}
		fixture := newFixture(t, workspaces)

		deleteResp := performRequest(
			t,
			fixture.Engine,
			http.MethodDelete,
			"/workspaces/ws_alpha",
			nil,
		)
		if deleteResp.Code != http.StatusConflict {
			t.Fatalf("delete conflict status = %d, want %d", deleteResp.Code, http.StatusConflict)
		}
		if !strings.Contains(deleteResp.Body.String(), "workspace has sessions") {
			t.Fatalf(
				"delete conflict body = %s, want workspace sessions message",
				deleteResp.Body.String(),
			)
		}
	})

	t.Run("Should reject unknown workspace sandbox refs as client errors", func(t *testing.T) {
		t.Parallel()

		workspace := workspacepkg.Workspace{ID: "ws_alpha", RootDir: t.TempDir(), Name: "alpha"}
		workspaces := testutil.StubWorkspaceService{
			GetFn: func(context.Context, string) (workspacepkg.Workspace, error) {
				return workspace, nil
			},
			UpdateFn: func(context.Context, string, workspacepkg.UpdateOptions) error {
				return aghconfig.ErrSandboxProfileNotFound
			},
		}
		fixture := newFixture(t, workspaces)

		badUpdate := performRequest(
			t,
			fixture.Engine,
			http.MethodPatch,
			"/workspaces/ws_alpha",
			[]byte(`{"sandbox_ref":"missing-profile"}`),
		)
		if badUpdate.Code != http.StatusBadRequest {
			t.Fatalf(
				"bad update status = %d, want %d; body=%s",
				badUpdate.Code,
				http.StatusBadRequest,
				badUpdate.Body.String(),
			)
		}
		if !strings.Contains(badUpdate.Body.String(), "sandbox profile not found") {
			t.Fatalf(
				"bad update body = %s, want sandbox profile validation message",
				badUpdate.Body.String(),
			)
		}
	})
}

func TestWorkspaceValidationBranches(t *testing.T) {
	t.Parallel()

	workspace := workspacepkg.Workspace{ID: "ws_alpha", RootDir: t.TempDir(), Name: "alpha"}
	workspaces := testutil.StubWorkspaceService{
		GetFn: func(context.Context, string) (workspacepkg.Workspace, error) {
			return workspace, nil
		},
	}
	fixture := newHandlerFixture(
		t,
		testutil.StubSessionManager{},
		testutil.StubObserver{},
		workspaces,
		nil,
		nil,
	)

	createResp := performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/workspaces",
		[]byte(`{"root_dir":"`+workspace.RootDir+`","add_dirs":["relative"]}`),
	)
	if createResp.Code != http.StatusBadRequest {
		t.Fatalf(
			"create invalid add_dirs status = %d, want %d",
			createResp.Code,
			http.StatusBadRequest,
		)
	}

	updateResp := performRequest(
		t,
		fixture.Engine,
		http.MethodPatch,
		"/workspaces/ws_alpha",
		[]byte(`{"add_dirs":["relative"]}`),
	)
	if updateResp.Code != http.StatusBadRequest {
		t.Fatalf(
			"update invalid add_dirs status = %d, want %d",
			updateResp.Code,
			http.StatusBadRequest,
		)
	}

	resolveResp := performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/workspaces/resolve",
		[]byte(`{"path":"relative"}`),
	)
	if resolveResp.Code != http.StatusBadRequest {
		t.Fatalf(
			"resolve invalid path status = %d, want %d",
			resolveResp.Code,
			http.StatusBadRequest,
		)
	}
}

func TestMemoryErrorAndDisabledBranches(t *testing.T) {
	t.Parallel()

	store := memory.NewStore(filepath.Join(t.TempDir(), "memory"))
	if err := store.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs() error = %v", err)
	}
	fixture := newHandlerFixture(
		t,
		testutil.StubSessionManager{},
		testutil.StubObserver{},
		testutil.StubWorkspaceService{},
		store,
		nil,
	)

	readMissing := performRequest(
		t,
		fixture.Engine,
		http.MethodGet,
		"/memory/missing.md?scope=global",
		nil,
	)
	if readMissing.Code != http.StatusNotFound {
		t.Fatalf("read missing status = %d, want %d", readMissing.Code, http.StatusNotFound)
	}

	deleteMissing := performRequest(
		t,
		fixture.Engine,
		http.MethodDelete,
		"/memory/missing.md?scope=global",
		nil,
	)
	if deleteMissing.Code != http.StatusNotFound {
		t.Fatalf("delete missing status = %d, want %d", deleteMissing.Code, http.StatusNotFound)
	}

	badWrite := performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/memory",
		[]byte(`{"scope":"global","type":"user","name":"Bad"}`),
	)
	if badWrite.Code != http.StatusBadRequest {
		t.Fatalf("bad write status = %d, want %d", badWrite.Code, http.StatusBadRequest)
	}

	badConsolidate := performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/memory/dreams/trigger",
		[]byte(`{`),
	)
	if badConsolidate.Code != http.StatusBadRequest {
		t.Fatalf("bad consolidate status = %d, want %d", badConsolidate.Code, http.StatusBadRequest)
	}

	disabledConsolidate := performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/memory/dreams/trigger",
		nil,
	)
	if disabledConsolidate.Code != http.StatusOK {
		t.Fatalf(
			"disabled consolidate status = %d, want %d",
			disabledConsolidate.Code,
			http.StatusOK,
		)
	}
}
