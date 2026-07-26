//go:build integration

package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/observe"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/sessiondb"
	testutilpkg "github.com/compozy/agh/internal/testutil"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestHTTPHookCatalogEndpointReturnsResolvedHooksInPipelineOrder(t *testing.T) {
	homePaths := newTestHomePaths(t)
	observer := newHookIntegrationObserver(t, homePaths)
	hooksRuntime := newHookIntegrationRuntime(t,
		hookspkg.WithNativeDeclarations([]hookspkg.HookDecl{{
			Name:         "native-first",
			Event:        hookspkg.HookSessionPostCreate,
			Mode:         hookspkg.HookModeSync,
			ExecutorKind: hookspkg.HookExecutorNative,
		}}),
		hookspkg.WithConfigDeclarations([]hookspkg.HookDecl{{
			Name:    "config-second",
			Event:   hookspkg.HookSessionPostCreate,
			Mode:    hookspkg.HookModeSync,
			Command: "/bin/sh",
			Args:    []string{"-c", "printf '{}'"},
		}}),
		hookspkg.WithExecutorResolver(hookIntegrationResolver(map[string]hookspkg.Executor{
			"native-first": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, _ hookspkg.SessionPostCreatePayload) (hookspkg.SessionPostCreatePatch, error) {
					return hookspkg.SessionPostCreatePatch{}, nil
				},
			),
		})),
	)
	observer.AttachHooks(hooksRuntime)

	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, observer, homePaths))
	recorder := performRequest(t, engine, http.MethodGet, "/api/hooks/catalog", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Hooks []contract.HookCatalogPayload `json:"hooks"`
	}
	decodeJSONResponse(t, recorder, &response)
	if got, want := len(response.Hooks), 2; got != want {
		t.Fatalf("len(hooks) = %d, want %d", got, want)
	}
	if response.Hooks[0].Name != "native-first" || response.Hooks[0].Order != 1 ||
		response.Hooks[0].Source != "native" {
		t.Fatalf("hooks[0] = %#v", response.Hooks[0])
	}
	if response.Hooks[0].ExecutorKind != string(hookspkg.HookExecutorNative) {
		t.Fatalf("hooks[0].ExecutorKind = %q, want %q", response.Hooks[0].ExecutorKind, hookspkg.HookExecutorNative)
	}
	if response.Hooks[1].Name != "config-second" || response.Hooks[1].Order != 2 ||
		response.Hooks[1].Source != "config" {
		t.Fatalf("hooks[1] = %#v", response.Hooks[1])
	}
	if response.Hooks[1].ExecutorKind != string(hookspkg.HookExecutorSubprocess) {
		t.Fatalf("hooks[1].ExecutorKind = %q, want %q", response.Hooks[1].ExecutorKind, hookspkg.HookExecutorSubprocess)
	}
}

func TestHTTPHookCatalogEndpointFiltersWorkspaceScopedHooks(t *testing.T) {
	homePaths := newTestHomePaths(t)
	observer := newHookIntegrationObserver(t, homePaths)
	hooksRuntime := newHookIntegrationRuntime(t,
		hookspkg.WithConfigDeclarations([]hookspkg.HookDecl{
			{
				Name:  "workspace-alpha",
				Event: hookspkg.HookSessionPostCreate,
				Mode:  hookspkg.HookModeSync,
				Matcher: hookspkg.HookMatcher{
					WorkspaceID:   "ws-alpha",
					WorkspaceRoot: "/workspace/alpha",
				},
				Command: "/bin/sh",
				Args:    []string{"-c", "printf '{}'"},
			},
			{
				Name:  "workspace-beta",
				Event: hookspkg.HookSessionPostCreate,
				Mode:  hookspkg.HookModeSync,
				Matcher: hookspkg.HookMatcher{
					WorkspaceID:   "ws-beta",
					WorkspaceRoot: "/workspace/beta",
				},
				Command: "/bin/sh",
				Args:    []string{"-c", "printf '{}'"},
			},
		}),
	)
	observer.AttachHooks(hooksRuntime)

	workspaces := stubWorkspaceService{
		ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
			if ref != "alpha" {
				t.Fatalf("Resolve() ref = %q, want alpha", ref)
			}
			return workspacepkg.ResolvedWorkspace{
				Workspace: workspacepkg.Workspace{
					ID:      "ws-alpha",
					RootDir: "/workspace/alpha",
				},
				WorkspaceID: "ws-alpha",
			}, nil
		},
	}

	engine := newTestRouter(t, newTestHandlersWithWorkspace(t, stubSessionManager{}, observer, workspaces, homePaths))
	recorder := performRequest(t, engine, http.MethodGet, "/api/hooks/catalog?workspace=alpha", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Hooks []contract.HookCatalogPayload `json:"hooks"`
	}
	decodeJSONResponse(t, recorder, &response)
	if got, want := len(response.Hooks), 1; got != want {
		t.Fatalf("len(hooks) = %d, want %d", got, want)
	}
	if response.Hooks[0].Name != "workspace-alpha" || response.Hooks[0].Source != "config" {
		t.Fatalf("hooks[0] = %#v", response.Hooks[0])
	}
}

func TestHTTPHookRunsEndpointReturnsExecutionHistoryWithPatchDiffs(t *testing.T) {
	homePaths := newTestHomePaths(t)
	observer := newHookIntegrationObserver(t, homePaths)
	sessionID := "sess-history"
	db := openHookRunSessionDB(t, homePaths, sessionID)
	recordedAt := time.Date(2026, 4, 9, 18, 30, 0, 0, time.UTC)
	if err := db.RecordHookRun(testutilpkg.Context(t), hookspkg.HookRunRecord{
		HookName:      "permission-history",
		Event:         hookspkg.HookPermissionRequest,
		Source:        hookspkg.HookSourceConfig,
		Mode:          hookspkg.HookModeSync,
		Duration:      15 * time.Millisecond,
		Outcome:       hookspkg.HookRunOutcomeDenied,
		DispatchDepth: 2,
		PatchApplied:  []byte(`{"decision":"deny","reason":"policy"}`),
		Required:      true,
		RecordedAt:    recordedAt,
	}); err != nil {
		t.Fatalf("RecordHookRun() error = %v", err)
	}
	closeHookRunSessionDB(t, db)

	manager := stubSessionManager{
		StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			return newSessionInfo(id), nil
		},
	}

	engine := newTestRouter(t, newTestHandlers(t, manager, observer, homePaths))
	recorder := performRequest(
		t,
		engine,
		http.MethodGet,
		"/api/workspaces/ws-workspace/hooks/runs?session="+sessionID,
		nil,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Runs []contract.HookRunPayload `json:"runs"`
	}
	decodeJSONResponse(t, recorder, &response)
	if got, want := len(response.Runs), 1; got != want {
		t.Fatalf("len(runs) = %d, want %d", got, want)
	}
	if response.Runs[0].HookName != "permission-history" ||
		string(response.Runs[0].PatchApplied) != `{"decision":"deny","reason":"policy"}` {
		t.Fatalf("runs[0] = %#v", response.Runs[0])
	}
}

func TestHTTPHookEventsEndpointReturnsAllEventsWithSyncEligibility(t *testing.T) {
	homePaths := newTestHomePaths(t)
	observer := newHookIntegrationObserver(t, homePaths)

	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, observer, homePaths))
	recorder := performRequest(t, engine, http.MethodGet, "/api/hooks/events", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Events []contract.HookEventPayload `json:"events"`
	}
	decodeJSONResponse(t, recorder, &response)
	if got, want := len(response.Events), len(hookspkg.AllHookEvents()); got != want {
		t.Fatalf("len(events) = %d, want %d", got, want)
	}

	byEvent := make(map[string]contract.HookEventPayload, len(response.Events))
	for _, event := range response.Events {
		byEvent[event.Event] = event
	}
	if event, ok := byEvent[hookspkg.HookMessageDelta.String()]; !ok || event.SyncEligible {
		t.Fatalf("message.delta = %#v, want async-only", event)
	}
	if event, ok := byEvent[hookspkg.HookPermissionRequest.String()]; !ok || !event.SyncEligible {
		t.Fatalf("permission.request = %#v, want sync-eligible", event)
	}
}

func TestHTTPHookCatalogEndpointFiltersByEventSourceAndMode(t *testing.T) {
	homePaths := newTestHomePaths(t)
	observer := newHookIntegrationObserver(t, homePaths)
	hooksRuntime := newHookIntegrationRuntime(t,
		hookspkg.WithNativeDeclarations([]hookspkg.HookDecl{{
			Name:         "native-tool",
			Event:        hookspkg.HookToolPreCall,
			Mode:         hookspkg.HookModeSync,
			ExecutorKind: hookspkg.HookExecutorNative,
		}}),
		hookspkg.WithConfigDeclarations([]hookspkg.HookDecl{
			{
				Name:    "config-tool-sync",
				Event:   hookspkg.HookToolPreCall,
				Mode:    hookspkg.HookModeSync,
				Command: "/bin/sh",
				Args:    []string{"-c", "printf '{}'"},
			},
			{
				Name:    "config-tool-async",
				Event:   hookspkg.HookToolPreCall,
				Mode:    hookspkg.HookModeAsync,
				Command: "/bin/sh",
				Args:    []string{"-c", "printf '{}'"},
			},
		}),
		hookspkg.WithExecutorResolver(hookIntegrationResolver(map[string]hookspkg.Executor{
			"native-tool": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, _ hookspkg.ToolPreCallPayload) (hookspkg.ToolCallPatch, error) {
					return hookspkg.ToolCallPatch{}, nil
				},
			),
		})),
	)
	observer.AttachHooks(hooksRuntime)

	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, observer, homePaths))
	recorder := performRequest(
		t,
		engine,
		http.MethodGet,
		"/api/hooks/catalog?event=tool.pre_call&source=config&mode=sync",
		nil,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Hooks []contract.HookCatalogPayload `json:"hooks"`
	}
	decodeJSONResponse(t, recorder, &response)
	if got, want := len(response.Hooks), 1; got != want {
		t.Fatalf("len(hooks) = %d, want %d", got, want)
	}
	if response.Hooks[0].Name != "config-tool-sync" ||
		response.Hooks[0].ExecutorKind != string(hookspkg.HookExecutorSubprocess) {
		t.Fatalf("hooks[0] = %#v, want filtered config sync hook", response.Hooks[0])
	}
}

func TestHTTPHookRunsEndpointFiltersByOutcomeAndLast(t *testing.T) {
	homePaths := newTestHomePaths(t)
	observer := newHookIntegrationObserver(t, homePaths)
	sessionID := "sess-history-filtered"
	db := openHookRunSessionDB(t, homePaths, sessionID)
	records := []hookspkg.HookRunRecord{
		{
			HookName:   "ignored-applied",
			Event:      hookspkg.HookPermissionRequest,
			Source:     hookspkg.HookSourceConfig,
			Mode:       hookspkg.HookModeSync,
			Outcome:    hookspkg.HookRunOutcomeApplied,
			RecordedAt: time.Date(2026, 4, 9, 18, 31, 0, 0, time.UTC),
		},
		{
			HookName:   "denied-older",
			Event:      hookspkg.HookPermissionRequest,
			Source:     hookspkg.HookSourceConfig,
			Mode:       hookspkg.HookModeSync,
			Outcome:    hookspkg.HookRunOutcomeDenied,
			RecordedAt: time.Date(2026, 4, 9, 18, 32, 0, 0, time.UTC),
		},
		{
			HookName:      "denied-newer",
			Event:         hookspkg.HookPermissionRequest,
			Source:        hookspkg.HookSourceConfig,
			Mode:          hookspkg.HookModeSync,
			Outcome:       hookspkg.HookRunOutcomeDenied,
			PatchApplied:  []byte(`{"decision":"deny","reason":"policy"}`),
			DispatchDepth: 1,
			RecordedAt:    time.Date(2026, 4, 9, 18, 33, 0, 0, time.UTC),
		},
	}
	for _, record := range records {
		if err := db.RecordHookRun(testutilpkg.Context(t), record); err != nil {
			t.Fatalf("RecordHookRun(%q) error = %v", record.HookName, err)
		}
	}
	closeHookRunSessionDB(t, db)

	manager := stubSessionManager{
		StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			return newSessionInfo(id), nil
		},
	}

	engine := newTestRouter(t, newTestHandlers(t, manager, observer, homePaths))
	recorder := performRequest(
		t,
		engine,
		http.MethodGet,
		"/api/workspaces/ws-workspace/hooks/runs?session="+sessionID+"&event=permission.request&outcome=denied&last=1",
		nil,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Runs []contract.HookRunPayload `json:"runs"`
	}
	decodeJSONResponse(t, recorder, &response)
	if got, want := len(response.Runs), 1; got != want {
		t.Fatalf("len(runs) = %d, want %d", got, want)
	}
	if response.Runs[0].HookName != "denied-newer" ||
		response.Runs[0].Outcome != string(hookspkg.HookRunOutcomeDenied) {
		t.Fatalf("runs[0] = %#v, want most recent denied run", response.Runs[0])
	}
}

func TestHTTPHookEventsEndpointFiltersByFamilyAndSyncOnly(t *testing.T) {
	homePaths := newTestHomePaths(t)
	observer := newHookIntegrationObserver(t, homePaths)

	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, observer, homePaths))
	recorder := performRequest(t, engine, http.MethodGet, "/api/hooks/events?family=tool&sync_only=true", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Events []contract.HookEventPayload `json:"events"`
	}
	decodeJSONResponse(t, recorder, &response)
	if len(response.Events) == 0 {
		t.Fatal("len(events) = 0, want filtered tool events")
	}
	for _, event := range response.Events {
		if event.Family != string(hookspkg.HookEventFamilyTool) {
			t.Fatalf("event.Family = %q, want %q", event.Family, hookspkg.HookEventFamilyTool)
		}
		if !event.SyncEligible {
			t.Fatalf("event.SyncEligible = false for %q, want true", event.Event)
		}
	}
}

func TestHTTPHookRunsEndpointDispatchStoreQueryCycle(t *testing.T) {
	homePaths := newTestHomePaths(t)
	observer := newHookIntegrationObserver(t, homePaths)
	sessionID := "sess-cycle"
	closeHookRunSessionDB(t, openHookRunSessionDB(t, homePaths, sessionID))

	hooksRuntime := newHookIntegrationRuntime(t,
		hookspkg.WithTelemetrySink(observer),
		hookspkg.WithNativeDeclarations([]hookspkg.HookDecl{{
			Name:         "permission-audit",
			Event:        hookspkg.HookPermissionRequest,
			Mode:         hookspkg.HookModeSync,
			ExecutorKind: hookspkg.HookExecutorNative,
		}}),
		hookspkg.WithExecutorResolver(hookIntegrationResolver(map[string]hookspkg.Executor{
			"permission-audit": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, _ hookspkg.PermissionRequestPayload) (hookspkg.PermissionRequestPatch, error) {
					deny := "deny"
					return hookspkg.PermissionRequestPatch{
						Decision: &deny,
						Reason:   hookStringPointer("policy"),
					}, nil
				},
			),
		})),
	)

	_, err := hooksRuntime.DispatchPermissionRequest(testutilpkg.Context(t), hookspkg.PermissionRequestPayload{
		PayloadBase: hookspkg.PayloadBase{Event: hookspkg.HookPermissionRequest},
		SessionContext: hookspkg.SessionContext{
			SessionID: sessionID,
		},
		Decision: "allow",
	})
	if err != nil {
		t.Fatalf("DispatchPermissionRequest() error = %v", err)
	}

	manager := stubSessionManager{
		StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			return newSessionInfo(id), nil
		},
	}

	engine := newTestRouter(t, newTestHandlers(t, manager, observer, homePaths))
	recorder := performRequest(
		t,
		engine,
		http.MethodGet,
		"/api/workspaces/ws-workspace/hooks/runs?session="+sessionID+"&event=permission.request",
		nil,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Runs []contract.HookRunPayload `json:"runs"`
	}
	decodeJSONResponse(t, recorder, &response)
	if got, want := len(response.Runs), 1; got != want {
		t.Fatalf("len(runs) = %d, want %d", got, want)
	}
	if response.Runs[0].HookName != "permission-audit" ||
		response.Runs[0].Outcome != string(hookspkg.HookRunOutcomeDenied) {
		t.Fatalf("runs[0] = %#v", response.Runs[0])
	}
	if string(response.Runs[0].PatchApplied) != `{"decision":"deny","reason":"policy"}` {
		t.Fatalf("runs[0].PatchApplied = %s, want deny patch", response.Runs[0].PatchApplied)
	}
}

func newHookIntegrationObserver(t *testing.T, homePaths aghconfig.HomePaths) *observe.Observer {
	t.Helper()

	observer, err := observe.New(testutilpkg.Context(t),
		observe.WithHomePaths(homePaths),
		observe.WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("observe.New() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := observer.Close(ctx); err != nil {
			t.Errorf("observer.Close() error = %v", err)
		}
	})
	return observer
}

func newHookIntegrationRuntime(t *testing.T, opts ...hookspkg.Option) *hookspkg.Hooks {
	t.Helper()

	runtime := hookspkg.NewHooks(append([]hookspkg.Option{
		hookspkg.WithLogger(discardLogger()),
	}, opts...)...)
	if err := runtime.Rebuild(testutilpkg.Context(t)); err != nil {
		t.Fatalf("Hooks.Rebuild() error = %v", err)
	}
	t.Cleanup(runtime.Close)
	return runtime
}

func hookIntegrationResolver(overrides map[string]hookspkg.Executor) hookspkg.ExecutorResolver {
	return func(decl hookspkg.HookDecl) (hookspkg.Executor, error) {
		if executor, ok := overrides[decl.Name]; ok {
			return executor, nil
		}
		if decl.Command != "" {
			opts := []hookspkg.SubprocessExecutorOption{}
			if len(decl.Env) != 0 {
				opts = append(opts, hookspkg.WithSubprocessEnv(decl.Env))
			}
			return hookspkg.NewSubprocessExecutor(decl.Command, decl.Args, opts...), nil
		}
		return nil, fmt.Errorf("unexpected executor resolution for hook %q", decl.Name)
	}
}

func openHookRunSessionDB(t *testing.T, homePaths aghconfig.HomePaths, sessionID string) *sessiondb.SessionDB {
	t.Helper()

	db, err := sessiondb.OpenSessionDB(
		testutilpkg.Context(t),
		sessionID,
		store.SessionDBFile(filepath.Join(homePaths.SessionsDir, sessionID)),
	)
	if err != nil {
		t.Fatalf("OpenSessionDB(%q) error = %v", sessionID, err)
	}
	return db
}

func closeHookRunSessionDB(t *testing.T, db *sessiondb.SessionDB) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.Close(ctx); err != nil {
		t.Fatalf("SessionDB.Close() error = %v", err)
	}
}

func hookStringPointer(value string) *string {
	return &value
}
