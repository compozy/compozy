package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestHookCatalogHandlerReturnsResolvedHooksAndWorkspaceFilter(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	source := hookspkg.HookSourceConfig
	observer := stubObserver{
		QueryHookCatalogFn: func(_ context.Context, filter hookspkg.CatalogFilter) ([]hookspkg.CatalogEntry, error) {
			if filter.WorkspaceID != "ws-alpha" {
				t.Fatalf("filter.WorkspaceID = %q, want ws-alpha", filter.WorkspaceID)
			}
			if filter.WorkspaceRoot != "/workspace/alpha" {
				t.Fatalf("filter.WorkspaceRoot = %q, want /workspace/alpha", filter.WorkspaceRoot)
			}
			if filter.AgentName != "coder" {
				t.Fatalf("filter.AgentName = %q, want coder", filter.AgentName)
			}
			if filter.Event != hookspkg.HookSessionPostCreate {
				t.Fatalf("filter.Event = %q, want %q", filter.Event, hookspkg.HookSessionPostCreate)
			}
			if filter.Source == nil || *filter.Source != source {
				t.Fatalf("filter.Source = %#v, want %q", filter.Source, source)
			}
			if filter.Mode != hookspkg.HookModeSync {
				t.Fatalf("filter.Mode = %q, want %q", filter.Mode, hookspkg.HookModeSync)
			}
			return []hookspkg.CatalogEntry{
				{
					Order:        1,
					Name:         "native-first",
					Event:        hookspkg.HookSessionPostCreate,
					Source:       hookspkg.HookSourceNative,
					Mode:         hookspkg.HookModeSync,
					Priority:     1000,
					ExecutorKind: hookspkg.HookExecutorNative,
				},
				{
					Order:        2,
					Name:         "config-second",
					Event:        hookspkg.HookSessionPostCreate,
					Source:       hookspkg.HookSourceConfig,
					Mode:         hookspkg.HookModeSync,
					Priority:     0,
					ExecutorKind: hookspkg.HookExecutorSubprocess,
				},
			}, nil
		},
	}
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
	recorder := performRequest(
		t,
		engine,
		http.MethodGet,
		"/api/hooks/catalog?workspace=alpha&agent=coder&event=session.post_create&source=config&mode=sync",
		nil,
	)
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

func TestHookRunsHandlerReturnsExecutionHistoryWithPatchDiffs(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	since := time.Date(2026, 4, 9, 17, 59, 0, 0, time.UTC)
	manager := stubSessionManager{
		StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			if id != "sess-hook" {
				t.Fatalf("Status() id = %q, want sess-hook", id)
			}
			return newSessionInfo(id), nil
		},
	}
	observer := stubObserver{
		QueryHookRunsFn: func(_ context.Context, query store.HookRunQuery) ([]hookspkg.HookRunRecord, error) {
			if query.SessionID != "sess-hook" {
				t.Fatalf("query.SessionID = %q, want sess-hook", query.SessionID)
			}
			if query.Event != hookspkg.HookPermissionRequest.String() {
				t.Fatalf("query.Event = %q, want %q", query.Event, hookspkg.HookPermissionRequest)
			}
			if query.Outcome != hookspkg.HookRunOutcomeDenied {
				t.Fatalf("query.Outcome = %q, want %q", query.Outcome, hookspkg.HookRunOutcomeDenied)
			}
			if !query.Since.Equal(since) {
				t.Fatalf("query.Since = %s, want %s", query.Since, since)
			}
			if query.Limit != 20 {
				t.Fatalf("query.Limit = %d, want 20", query.Limit)
			}
			return []hookspkg.HookRunRecord{
				{
					HookName:      "permission-audit",
					Event:         hookspkg.HookPermissionRequest,
					Source:        hookspkg.HookSourceConfig,
					Mode:          hookspkg.HookModeSync,
					Duration:      25 * time.Millisecond,
					Outcome:       hookspkg.HookRunOutcomeDenied,
					DispatchDepth: 2,
					PatchApplied:  []byte(`{"decision":"deny","reason":"policy"}`),
					Error:         "denied by policy",
					Required:      true,
					RecordedAt:    time.Date(2026, 4, 9, 18, 0, 0, 0, time.UTC),
				},
			}, nil
		},
	}

	engine := newTestRouter(t, newTestHandlers(t, manager, observer, homePaths))
	recorder := performRequest(
		t,
		engine,
		http.MethodGet,
		"/api/workspaces/ws-workspace/hooks/runs?session=sess-hook&event=permission.request&outcome=denied&since=2026-04-09T17:59:00Z&last=20",
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
	if response.Runs[0].HookName != "permission-audit" {
		t.Fatalf("runs[0].HookName = %q, want permission-audit", response.Runs[0].HookName)
	}
	if string(response.Runs[0].PatchApplied) != `{"decision":"deny","reason":"policy"}` {
		t.Fatalf("runs[0].PatchApplied = %s, want deny patch", response.Runs[0].PatchApplied)
	}
	if response.Runs[0].DispatchDepth != 2 || response.Runs[0].Outcome != string(hookspkg.HookRunOutcomeDenied) {
		t.Fatalf("runs[0] = %#v", response.Runs[0])
	}
}

func TestHookRunsHandlerRejectsMissingSession(t *testing.T) {
	t.Parallel()

	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, newTestHomePaths(t)))
	recorder := performRequest(t, engine, http.MethodGet, "/api/workspaces/ws-workspace/hooks/runs", nil)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}

func TestHookRunsHandlerRejectsForeignWorkspaceSession(t *testing.T) {
	t.Parallel()

	manager := stubSessionManager{
		StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			info := newSessionInfo(id)
			info.WorkspaceID = "ws-other"
			return info, nil
		},
	}
	observer := stubObserver{
		QueryHookRunsFn: func(context.Context, store.HookRunQuery) ([]hookspkg.HookRunRecord, error) {
			t.Fatal("QueryHookRuns() called for foreign workspace session")
			return nil, nil
		},
	}
	engine := newTestRouter(t, newTestHandlers(t, manager, observer, newTestHomePaths(t)))

	recorder := performRequest(
		t,
		engine,
		http.MethodGet,
		"/api/workspaces/ws-workspace/hooks/runs?session=sess-hook",
		nil,
	)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNotFound, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "sess-hook") {
		t.Fatalf("body = %s, want no foreign session id disclosure", recorder.Body.String())
	}
}

func TestHookRunsHandlerRejectsInvalidEvent(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	manager := stubSessionManager{
		StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			return newSessionInfo(id), nil
		},
	}

	engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, homePaths))
	recorder := performRequest(
		t,
		engine,
		http.MethodGet,
		"/api/workspaces/ws-workspace/hooks/runs?session=sess-hook&event=not-a-hook",
		nil,
	)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}

func TestHookCatalogHandlerRejectsInvalidSource(t *testing.T) {
	t.Parallel()

	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, newTestHomePaths(t)))
	recorder := performRequest(t, engine, http.MethodGet, "/api/hooks/catalog?source=wrong", nil)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}

func TestHookCatalogHandlerRejectsInvalidMode(t *testing.T) {
	t.Parallel()

	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, newTestHomePaths(t)))
	recorder := performRequest(t, engine, http.MethodGet, "/api/hooks/catalog?mode=wrong", nil)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}

func TestHookRunsHandlerRejectsInvalidOutcome(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	manager := stubSessionManager{
		StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			return newSessionInfo(id), nil
		},
	}

	engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, homePaths))
	recorder := performRequest(
		t,
		engine,
		http.MethodGet,
		"/api/workspaces/ws-workspace/hooks/runs?session=sess-hook&outcome=nope",
		nil,
	)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}

func TestHookRunsHandlerRejectsInvalidSince(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	manager := stubSessionManager{
		StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			return newSessionInfo(id), nil
		},
	}

	engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, homePaths))
	recorder := performRequest(
		t,
		engine,
		http.MethodGet,
		"/api/workspaces/ws-workspace/hooks/runs?session=sess-hook&since=not-a-time",
		nil,
	)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}

func TestHookRunsHandlerRejectsInvalidLast(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	manager := stubSessionManager{
		StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			return newSessionInfo(id), nil
		},
	}

	engine := newTestRouter(t, newTestHandlers(t, manager, stubObserver{}, homePaths))
	recorder := performRequest(
		t,
		engine,
		http.MethodGet,
		"/api/workspaces/ws-workspace/hooks/runs?session=sess-hook&last=-1",
		nil,
	)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}

func TestHookEventsHandlerReturnsPayloads(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	observer := stubObserver{
		QueryHookEventsFn: func(_ context.Context, filter hookspkg.EventFilter) ([]hookspkg.EventDescriptor, error) {
			if filter.Family != hookspkg.HookEventFamilyTool {
				t.Fatalf("filter.Family = %q, want %q", filter.Family, hookspkg.HookEventFamilyTool)
			}
			if !filter.SyncOnly {
				t.Fatal("filter.SyncOnly = false, want true")
			}
			return []hookspkg.EventDescriptor{
				{
					Event:         hookspkg.HookToolPreCall,
					Family:        hookspkg.HookEventFamilyTool,
					SyncEligible:  true,
					PayloadSchema: "ToolPreCallPayload",
					PatchSchema:   "ToolCallPatch",
				},
			}, nil
		},
	}

	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, observer, homePaths))
	recorder := performRequest(t, engine, http.MethodGet, "/api/hooks/events?family=tool&sync_only=true", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Events []contract.HookEventPayload `json:"events"`
	}
	decodeJSONResponse(t, recorder, &response)
	if got, want := len(response.Events), 1; got != want {
		t.Fatalf("len(events) = %d, want %d", got, want)
	}
	if response.Events[0].Event != hookspkg.HookToolPreCall.String() || !response.Events[0].SyncEligible {
		t.Fatalf("events[0] = %#v", response.Events[0])
	}
}

func TestHookEventsHandlerRejectsInvalidFamily(t *testing.T) {
	t.Parallel()

	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, newTestHomePaths(t)))
	recorder := performRequest(t, engine, http.MethodGet, "/api/hooks/events?family=nope", nil)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}

func TestHookEventsHandlerRejectsInvalidSyncOnly(t *testing.T) {
	t.Parallel()

	engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, newTestHomePaths(t)))
	recorder := performRequest(t, engine, http.MethodGet, "/api/hooks/events?sync_only=maybe", nil)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}
